package ufw

import (
	"strconv"
	"strings"
)

// ParseRules parses the output of `ufw status numbered` or `ufw show added`
// into structured rules.
//
// Numbered format:
//
//	[ 1] 2053/tcp               ALLOW IN    Anywhere
//
// Unnumbered format:
//
//	2053/tcp                   ALLOW IN    Anywhere
//
// Show-added format:
//
//	ufw allow 2053/tcp comment 'web panel'
func ParseRules(output string) []Rule {
	var rules []Rule
	seen := make(map[string]bool)

	for _, line := range strings.Split(output, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		rule := parseRuleLine(line)
		if rule == nil {
			continue
		}

		dedupKey := rule.Port + "|" + rule.Protocol + "|" + rule.Action
		if seen[dedupKey] {
			continue
		}
		seen[dedupKey] = true
		rules = append(rules, *rule)
	}
	return rules
}

func parseRuleLine(line string) *Rule {
	// Try "ufw show added" format: "ufw allow 2053/tcp comment 'web panel'"
	if rule := parseShowAddedLine(line); rule != nil {
		return rule
	}
	// Try standard numbered/unnumbered format
	return parseStatusLine(line)
}

// parseShowAddedLine handles: "ufw allow 2053/tcp comment 'web panel'"
func parseShowAddedLine(line string) *Rule {
	fields := strings.Fields(line)
	if len(fields) < 3 || fields[0] != "ufw" {
		return nil
	}

	action := strings.ToLower(fields[1])
	switch action {
	case "allow", "deny", "reject", "limit":
	default:
		return nil
	}

	portField := fields[2]
	before, after, found := strings.Cut(portField, "/")
	if !found || before == "" || !isNumeric(before) {
		return nil
	}

	port := portField
	proto := ""
	if afterTmp := strings.ToLower(strings.TrimSpace(after)); afterTmp == "tcp" || afterTmp == "udp" {
		proto = afterTmp
	}

	// Extract comment if present
	comment := ""
	for i := 3; i < len(fields); i++ {
		if fields[i] == "comment" && i+1 < len(fields) {
			c := fields[i+1]
			c = strings.Trim(c, "'\"")
			comment = c
			break
		}
	}

	return &Rule{
		Port:     port,
		Protocol: proto,
		Action:   action,
		Comment:  comment,
	}
}

// parseStatusLine handles numbered and unnumbered ufw status output.
func parseStatusLine(line string) *Rule {
	upper := strings.ToUpper(line)

	actionWord, actionIdx := findAction(upper)
	if actionWord == "" || actionIdx < 0 {
		return nil
	}
	action := strings.ToLower(actionWord)

	number := 0
	content := line
	if strings.HasPrefix(line, "[") {
		closeIdx := strings.Index(line, "]")
		if closeIdx > 0 {
			numStr := strings.TrimSpace(line[1:closeIdx])
			if n, err := strconv.Atoi(numStr); err == nil {
				number = n
			}
			content = strings.TrimSpace(line[closeIdx+1:])
		}
	}

	// Find action in the content (without number prefix)
	upper = strings.ToUpper(content)
	_, actionIdx = findAction(upper)
	if actionIdx < 0 {
		return nil
	}

	before := strings.TrimSpace(content[:actionIdx])
	port, proto := extractPortFromFields(before)
	if port == "" {
		return nil
	}

	// Extract comment from the "after" portion (text after action keyword)
	afterText := strings.TrimSpace(content[actionIdx+len(actionWord):])
	comment := extractComment(afterText)

	return &Rule{
		Number:   number,
		Port:     port,
		Protocol: proto,
		Action:   action,
		Comment:  comment,
	}
}

func findAction(s string) (string, int) {
	actions := []string{"ALLOW", "DENY", "REJECT", "LIMIT"}
	for _, a := range actions {
		if idx := strings.Index(s, a); idx >= 0 {
			return a, idx
		}
	}
	return "", -1
}

func extractPortFromFields(s string) (port, proto string) {
	fields := strings.Fields(s)
	for _, f := range fields {
		f = strings.TrimSpace(f)
		if f == "" {
			continue
		}
		before, after, found := strings.Cut(f, "/")
		if !found || before == "" {
			continue
		}
		if !isNumeric(before) {
			continue
		}
		after = strings.ToLower(strings.TrimSpace(after))
		if after == "tcp" || after == "udp" {
			return before + "/" + after, after
		}
		return before + "/" + after, ""
	}
	return "", ""
}

func extractComment(s string) string {
	if idx := strings.Index(s, "#"); idx >= 0 {
		return strings.TrimSpace(s[idx+1:])
	}
	return ""
}

func isNumeric(s string) bool {
	if s == "" {
		return false
	}
	for _, c := range s {
		if c < '0' || c > '9' {
			return false
		}
	}
	return true
}
