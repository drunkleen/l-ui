package service

import (
	"testing"

	"github.com/drunkleen/l-ui/internal/ufw"
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
	if rules[0].Port != "2053/tcp" {
		t.Errorf("rule[0].Port = %q, want '2053/tcp'", rules[0].Port)
	}
	if rules[0].Action != "allow" {
		t.Errorf("rule[0].Action = %q, want 'allow'", rules[0].Action)
	}
	if rules[0].Protocol != "tcp" {
		t.Errorf("rule[0].Protocol = %q, want 'tcp'", rules[0].Protocol)
	}
	if rules[1].Port != "22/tcp" {
		t.Errorf("rule[1].Port = %q, want '22/tcp'", rules[1].Port)
	}
	if rules[1].Action != "allow" {
		t.Errorf("rule[1].Action = %q, want 'allow'", rules[1].Action)
	}
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
	output := `[ 1] 2053/tcp                   ALLOW IN    Anywhere                   # web panel
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

func TestParseUFWRulesNonPortLine(t *testing.T) {
	output := "ALLOW IN                     Anywhere\n"
	rules := ParseUFWRules(output)
	if len(rules) != 0 {
		t.Fatalf("expected 0 rules for non-port line, got %d", len(rules))
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

func TestInternalParseRulesEmpty(t *testing.T) {
	rules := ufw.ParseRules("")
	if len(rules) != 0 {
		t.Fatalf("expected 0 rules, got %d", len(rules))
	}
}

func TestInternalParseRulesBasic(t *testing.T) {
	output := `2053/tcp                   ALLOW IN    Anywhere
22/tcp                     ALLOW IN    Anywhere
`
	rules := ufw.ParseRules(output)
	if len(rules) != 2 {
		t.Fatalf("expected 2 rules, got %d", len(rules))
	}
	if rules[0].Port != "2053/tcp" || rules[0].Action != "allow" {
		t.Errorf("rule[0] = %+v", rules[0])
	}
}
