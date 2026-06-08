package service

import (
	"testing"
)

func TestParseUFWRulesEmpty(t *testing.T) {
	rules := ParseUFWRules("")
	if len(rules) != 0 {
		t.Fatalf("expected 0 rules, got %d", len(rules))
	}
}

func TestParseUFWRulesStatusOnly(t *testing.T) {
	output := "Status: active\nLogging: on\n"
	rules := ParseUFWRules(output)
	if len(rules) != 0 {
		t.Fatalf("expected 0 rules for status-only output, got %d", len(rules))
	}
}

func TestParseUFWRulesBasic(t *testing.T) {
	output := `Status: active

To                         Action      From
--                         ------      ----
2053/tcp                   ALLOW IN    Anywhere
22/tcp                     ALLOW IN    Anywhere
443/tcp                    DENY IN     0.0.0.0/0
`
	rules := ParseUFWRules(output)
	if len(rules) != 3 {
		t.Fatalf("expected 3 rules, got %d", len(rules))
	}

	// First rule: 2053/tcp ALLOW
	if rules[0].Port != "2053/tcp" {
		t.Errorf("rule[0].Port = %q, want '2053/tcp'", rules[0].Port)
	}
	if rules[0].Action != "allow" {
		t.Errorf("rule[0].Action = %q, want 'allow'", rules[0].Action)
	}
	if rules[0].Protocol != "tcp" {
		t.Errorf("rule[0].Protocol = %q, want 'tcp'", rules[0].Protocol)
	}

	// Second rule: 22/tcp ALLOW
	if rules[1].Port != "22/tcp" {
		t.Errorf("rule[1].Port = %q, want '22/tcp'", rules[1].Port)
	}
	if rules[1].Action != "allow" {
		t.Errorf("rule[1].Action = %q, want 'allow'", rules[1].Action)
	}

	// Third rule: 443/tcp DENY
	if rules[2].Port != "443/tcp" {
		t.Errorf("rule[2].Port = %q, want '443/tcp'", rules[2].Port)
	}
	if rules[2].Action != "deny" {
		t.Errorf("rule[2].Action = %q, want 'deny'", rules[2].Action)
	}
}

func TestParseUFWRulesNumbered(t *testing.T) {
	output := `Status: active

     To                         Action      From
     --                         ------      ----
[ 1] 2053/tcp                   ALLOW IN    Anywhere
[ 2] 22/tcp                     ALLOW IN    Anywhere
[ 3] 443/tcp                    DENY IN     0.0.0.0/0
`
	rules := ParseUFWRules(output)
	if len(rules) != 3 {
		t.Fatalf("expected 3 rules, got %d", len(rules))
	}
	if rules[0].Port != "2053/tcp" {
		t.Errorf("rule[0].Port = %q, want '2053/tcp'", rules[0].Port)
	}
}

func TestParseUFWRulesWithComment(t *testing.T) {
	output := `Status: active

[ 1] 2053/tcp                   ALLOW IN    Anywhere                   # web panel
[ 2] 22/tcp                     ALLOW IN    Anywhere                   # SSH
`
	rules := ParseUFWRules(output)
	if len(rules) != 2 {
		t.Fatalf("expected 2 rules, got %d", len(rules))
	}
	if rules[0].Port != "2053/tcp" {
		t.Errorf("rule[0].Port = %q, want '2053/tcp'", rules[0].Port)
	}
}

func TestParseUFWRulesReject(t *testing.T) {
	output := "443/tcp                   REJECT IN   Anywhere\n"
	rules := ParseUFWRules(output)
	if len(rules) != 1 {
		t.Fatalf("expected 1 rule, got %d", len(rules))
	}
	if rules[0].Action != "reject" {
		t.Errorf("action = %q, want 'reject'", rules[0].Action)
	}
}

func TestParseUFWRulesLimit(t *testing.T) {
	output := "22/tcp                   LIMIT IN    Anywhere\n"
	rules := ParseUFWRules(output)
	if len(rules) != 1 {
		t.Fatalf("expected 1 rule, got %d", len(rules))
	}
	if rules[0].Action != "limit" {
		t.Errorf("action = %q, want 'limit'", rules[0].Action)
	}
}

func TestParseUFWRulesUDP(t *testing.T) {
	output := "53/udp                    ALLOW IN    Anywhere\n"
	rules := ParseUFWRules(output)
	if len(rules) != 1 {
		t.Fatalf("expected 1 rule, got %d", len(rules))
	}
	if rules[0].Protocol != "udp" {
		t.Errorf("protocol = %q, want 'udp'", rules[0].Protocol)
	}
}

func TestParseUFWRulesSkipsHeader(t *testing.T) {
	output := `To                         Action      From
--                         ------      ----
2053/tcp                   ALLOW IN    Anywhere
`
	rules := ParseUFWRules(output)
	if len(rules) != 1 {
		t.Fatalf("expected 1 rule, got %d", len(rules))
	}
}

func TestParseUFWRulesNonPortLine(t *testing.T) {
	// Lines without a valid port field should be skipped.
	output := "ALLOW IN                     Anywhere\n"
	rules := ParseUFWRules(output)
	if len(rules) != 0 {
		t.Fatalf("expected 0 rules for non-port line, got %d", len(rules))
	}
}

func TestParseUFWRulesPortInWeirdPosition(t *testing.T) {
	// Parser should find the port even if it's not in the expected position.
	output := "Anywhere                   ALLOW IN    2053/tcp\n"
	rules := ParseUFWRules(output)
	if len(rules) != 1 {
		t.Fatalf("expected 1 rule, got %d", len(rules))
	}
	if rules[0].Port != "2053/tcp" {
		t.Errorf("port = %q, want '2053/tcp'", rules[0].Port)
	}
}

func TestParseUFWRulesMixedFormat(t *testing.T) {
	output := `Status: active

[ 1] 2053/tcp                   ALLOW IN    Anywhere
[ 2] 22/tcp                     ALLOW IN    Anywhere
`
	rules := ParseUFWRules(output)
	if len(rules) != 2 {
		t.Fatalf("expected 2 rules, got %d", len(rules))
	}
	if rules[0].Action != "allow" {
		t.Errorf("rule[0].Action = %q, want 'allow'", rules[0].Action)
	}
}

// ── Port parsing edge cases ─────────────────────────────────────────

func TestIsPortField(t *testing.T) {
	tests := []struct {
		input string
		want  bool
	}{
		{"2053/tcp", true},
		{"22/tcp", true},
		{"443/udp", true},
		{"Anywhere", false},
		{"ALLOW", false},
		{"IN", false},
		{"", false},
		{"/tcp", false},
		{"abc/tcp", false},
	}
	for _, tt := range tests {
		got := isPortField(tt.input)
		if got != tt.want {
			t.Errorf("isPortField(%q) = %v, want %v", tt.input, got, tt.want)
		}
	}
}

func TestExtractProtocol(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"2053/tcp", "tcp"},
		{"22/tcp", "tcp"},
		{"443/udp", "udp"},
		{"53/udp", "udp"},
		{"443", ""},
		{"443/xyz", ""},
		{"/tcp", ""},
	}
	for _, tt := range tests {
		got := extractProtocol(tt.input)
		if got != tt.want {
			t.Errorf("extractProtocol(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}
