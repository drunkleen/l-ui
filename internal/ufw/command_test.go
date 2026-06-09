package ufw

import (
	"context"
	"testing"
)

type mockRunner struct {
	lookPathFn func(string) (string, error)
	runFn      func(context.Context, string, ...string) ([]byte, error)
}

func (m *mockRunner) LookPath(file string) (string, error) {
	return m.lookPathFn(file)
}

func (m *mockRunner) Run(ctx context.Context, name string, args ...string) ([]byte, error) {
	return m.runFn(ctx, name, args...)
}

func TestSanitizePort(t *testing.T) {
	tests := []struct {
		input   interface{}
		want    int
		wantErr bool
	}{
		{80, 80, false},
		{443, 443, false},
		{1, 1, false},
		{65535, 65535, false},
		{0, 0, true},
		{65536, 0, true},
		{-1, 0, true},
		{"80", 80, false},
		{"443", 443, false},
		{"0", 0, true},
		{"65536", 0, true},
		{"abc", 0, true},
		{"", 0, true},
	}
	for _, tt := range tests {
		got, err := SanitizePort(tt.input)
		if (err != nil) != tt.wantErr {
			t.Errorf("SanitizePort(%v) error = %v, wantErr %v", tt.input, err, tt.wantErr)
			continue
		}
		if got != tt.want {
			t.Errorf("SanitizePort(%v) = %d, want %d", tt.input, got, tt.want)
		}
	}
}

func TestSanitizeProtocol(t *testing.T) {
	tests := []struct {
		input   string
		want    string
		wantErr bool
	}{
		{"tcp", "tcp", false},
		{"TCP", "tcp", false},
		{"udp", "udp", false},
		{"UDP", "udp", false},
		{"", "", false},
		{"abc", "", true},
		{"tcp ", "tcp", false},
	}
	for _, tt := range tests {
		got, err := SanitizeProtocol(tt.input)
		if (err != nil) != tt.wantErr {
			t.Errorf("SanitizeProtocol(%q) error = %v, wantErr %v", tt.input, err, tt.wantErr)
			continue
		}
		if got != tt.want {
			t.Errorf("SanitizeProtocol(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

func TestSanitizeComment(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"web panel", "web panel"},
		{"", ""},
		{"  SSH access  ", "SSH access"},
		{"bad;command", ""},
		{"bad&command", ""},
		{"bad`command", ""},
		{"bad$command", ""},
	}
	for _, tt := range tests {
		got := SanitizeComment(tt.input)
		if got != tt.want {
			t.Errorf("SanitizeComment(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

func TestParseRules(t *testing.T) {
	output := `Status: active

To                         Action      From
--                         ------      ----
[ 1] 2053/tcp               ALLOW IN    Anywhere
[ 2] 22/tcp                 ALLOW IN    Anywhere
[ 3] 443/tcp                DENY IN     0.0.0.0/0
`
	rules := ParseRules(output)
	if len(rules) != 3 {
		t.Fatalf("expected 3 rules, got %d", len(rules))
	}
	if rules[0].Port != "2053/tcp" || rules[0].Action != "allow" || rules[0].Protocol != "tcp" {
		t.Errorf("rule[0] = %+v", rules[0])
	}
	if rules[1].Port != "22/tcp" || rules[1].Action != "allow" {
		t.Errorf("rule[1] = %+v", rules[1])
	}
	if rules[2].Port != "443/tcp" || rules[2].Action != "deny" {
		t.Errorf("rule[2] = %+v", rules[2])
	}
}

func TestParseRulesEmpty(t *testing.T) {
	rules := ParseRules("")
	if len(rules) != 0 {
		t.Fatalf("expected 0 rules, got %d", len(rules))
	}
}

func TestParseRulesStatusOnly(t *testing.T) {
	rules := ParseRules("Status: active\nLogging: on\n")
	if len(rules) != 0 {
		t.Fatalf("expected 0 rules, got %d", len(rules))
	}
}

func TestParseRulesUnnumbered(t *testing.T) {
	output := `2053/tcp                   ALLOW IN    Anywhere
22/tcp                     ALLOW IN    Anywhere
`
	rules := ParseRules(output)
	if len(rules) != 2 {
		t.Fatalf("expected 2 rules, got %d", len(rules))
	}
}

func TestParseRulesRejectAndLimit(t *testing.T) {
	output := `443/tcp                   REJECT IN   Anywhere
22/tcp                    LIMIT IN    Anywhere
`
	rules := ParseRules(output)
	if len(rules) != 2 {
		t.Fatalf("expected 2 rules, got %d", len(rules))
	}
	if rules[0].Action != "reject" {
		t.Errorf("rule[0].Action = %q, want reject", rules[0].Action)
	}
	if rules[1].Action != "limit" {
		t.Errorf("rule[1].Action = %q, want limit", rules[1].Action)
	}
}

func TestParseRulesWithComment(t *testing.T) {
	output := `2053/tcp                   ALLOW IN    Anywhere                   # web panel
`
	rules := ParseRules(output)
	if len(rules) != 1 {
		t.Fatalf("expected 1 rule, got %d", len(rules))
	}
	if rules[0].Comment != "web panel" {
		t.Errorf("rule[0].Comment = %q, want 'web panel'", rules[0].Comment)
	}
}

func TestParseRulesShowAdded(t *testing.T) {
	output := `Added user rules:
ufw allow 2053/tcp comment 'web panel'
ufw allow 22/tcp comment 'SSH'
`
	rules := ParseRules(output)
	if len(rules) != 2 {
		t.Fatalf("expected 2 rules, got %d", len(rules))
	}
	if rules[0].Port != "2053/tcp" || rules[0].Action != "allow" {
		t.Errorf("rule[0] = %+v", rules[0])
	}
}
