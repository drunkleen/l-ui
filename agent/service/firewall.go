package service

import (
	"os/exec"
	"strings"
)

type FirewallRule struct {
	Port     string `json:"port"`
	Protocol string `json:"protocol"`
	Action   string `json:"action"`
	Comment  string `json:"comment,omitempty"`
}

type FirewallStatus struct {
	Active    bool           `json:"active"`
	Installed bool           `json:"installed"`
	Rules     []FirewallRule `json:"rules"`
}

type FirewallService struct{}

func NewFirewallService() *FirewallService {
	return &FirewallService{}
}

func (s *FirewallService) GetStatus() (*FirewallStatus, error) {
	installed := false
	if _, err := exec.Command("ufw", "--version").Output(); err == nil {
		installed = true
	}

	active := false
	if installed {
		if out, err := exec.Command("ufw", "status").Output(); err == nil {
			active = strings.Contains(string(out), "active")
		}
	}

	rules, _ := s.GetRules()

	return &FirewallStatus{
		Active:    active,
		Installed: installed,
		Rules:     rules,
	}, nil
}

func (s *FirewallService) GetRules() ([]FirewallRule, error) {
	out, err := exec.Command("ufw", "status", "numbered").Output()
	if err != nil {
		return nil, err
	}
	return ParseUFWRules(string(out)), nil
}

func (s *FirewallService) AddRule(port, protocol, action, comment string) error {
	args := []string{action, port}
	if protocol != "" {
		args = append(args, "proto", protocol)
	}
	if comment != "" {
		args = append(args, "comment", comment)
	}
	return exec.Command("ufw", args...).Run()
}

func (s *FirewallService) DeleteRule(ruleNum string) error {
	return exec.Command("ufw", "--force", "delete", ruleNum).Run()
}

func (s *FirewallService) Enable() error {
	return exec.Command("ufw", "--force", "enable").Run()
}

func (s *FirewallService) Disable() error {
	return exec.Command("ufw", "--force", "disable").Run()
}

func ParseUFWRules(output string) []FirewallRule {
	var rules []FirewallRule
	for _, line := range strings.Split(output, "\n") {
		line = strings.TrimSpace(line)
		if line == "" || !strings.Contains(line, "ALLOW") && !strings.Contains(line, "DENY") && !strings.Contains(line, "REJECT") && !strings.Contains(line, "LIMIT") {
			continue
		}
		parts := strings.Fields(line)
		if len(parts) < 2 {
			continue
		}
		rule := FirewallRule{Action: "allow"}
		if strings.Contains(line, "DENY") {
			rule.Action = "deny"
		} else if strings.Contains(line, "REJECT") {
			rule.Action = "reject"
		} else if strings.Contains(line, "LIMIT") {
			rule.Action = "limit"
		}
		// Handle numbered output: [ N] PORT ACTION ...
		portIdx := 0
		if strings.HasPrefix(parts[0], "[") {
			portIdx = 1
			// If the number has a trailing bracket (e.g. "1]"), skip it too.
			if portIdx < len(parts) && strings.HasSuffix(parts[portIdx], "]") {
				portIdx++
			}
		}
		// Find the port field — it should be before the action keyword.
		for portIdx < len(parts) && !isPortField(parts[portIdx]) {
			portIdx++
		}
		if portIdx < len(parts) {
			portField := parts[portIdx]
			rule.Port = portField
			if sub := extractProtocol(portField); sub != "" {
				rule.Protocol = sub
			}
			rules = append(rules, rule)
		}
	}
	return rules
}

func isPortField(s string) bool {
	if !strings.Contains(s, "/") {
		return false
	}
	before, _, _ := strings.Cut(s, "/")
	if before == "" {
		return false
	}
	for _, c := range before {
		if c < '0' || c > '9' {
			return false
		}
	}
	return true
}

func extractProtocol(s string) string {
	before, after, found := strings.Cut(s, "/")
	if !found || before == "" {
		return ""
	}
	switch after {
	case "tcp", "udp":
		return after
	default:
		return ""
	}
}
