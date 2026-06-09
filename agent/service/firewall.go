package service

import (
	"fmt"

	"github.com/drunkleen/l-ui/internal/ufw"
)

// FirewallRule is a single UFW rule for API responses.
type FirewallRule struct {
	Port     string `json:"port"`
	Protocol string `json:"protocol"`
	Action   string `json:"action"`
	Comment  string `json:"comment,omitempty"`
}

// FirewallStatus is the current UFW state for API responses.
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
	installed := ufw.IsInstalled()
	if !installed {
		return &FirewallStatus{Installed: false, Rules: []FirewallRule{}}, nil
	}

	active, err := ufw.IsActive()
	if err != nil {
		active = false
	}

	rules, _ := s.GetRules()

	return &FirewallStatus{
		Active:    active,
		Installed: true,
		Rules:     rules,
	}, nil
}

func (s *FirewallService) GetRules() ([]FirewallRule, error) {
	raw, _, err := ufw.GetRulesOutput()
	if err != nil {
		return nil, fmt.Errorf("failed to get ufw rules: %w", err)
	}

	parsed := ufw.ParseRules(raw)
	rules := make([]FirewallRule, 0, len(parsed))
	for _, r := range parsed {
		rules = append(rules, FirewallRule{
			Port:     r.Port,
			Protocol: r.Protocol,
			Action:   r.Action,
			Comment:  r.Comment,
		})
	}
	return rules, nil
}

func (s *FirewallService) AddRule(port, protocol, action, comment string) error {
	p, pErr := ufw.SanitizePort(port)
	if pErr != nil {
		return fmt.Errorf("invalid port: %w", pErr)
	}
	proto, prErr := ufw.SanitizeProtocol(protocol)
	if prErr != nil {
		return fmt.Errorf("invalid protocol: %w", prErr)
	}

	switch action {
	case "allow":
		return ufw.RunAllow(p, proto, comment)
	case "deny":
		return ufw.RunDeny(p, proto, comment)
	default:
		return fmt.Errorf("unsupported action: %s", action)
	}
}

func (s *FirewallService) DeleteRule(ruleNum string) error {
	return ufw.RunDelete(ruleNum)
}

func (s *FirewallService) Enable() error {
	return ufw.RunEnable()
}

func (s *FirewallService) Disable() error {
	return ufw.RunDisable()
}

// Deprecated: ParseUFWRules moved to internal/ufw.ParseRules.
// Kept for backward compatibility with existing callers.
func ParseUFWRules(output string) []FirewallRule {
	parsed := ufw.ParseRules(output)
	rules := make([]FirewallRule, 0, len(parsed))
	for _, r := range parsed {
		rules = append(rules, FirewallRule{
			Port:     r.Port,
			Protocol: r.Protocol,
			Action:   r.Action,
			Comment:  r.Comment,
		})
	}
	return rules
}
