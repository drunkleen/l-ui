package service

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/drunkleen/l-ui/internal/ufw"
)

type mockCmdRunner struct {
	runFn func(ctx context.Context, name string, args ...string) ([]byte, error)
}

func (m *mockCmdRunner) LookPath(file string) (string, error) {
	return "/usr/sbin/ufw", nil
}

func (m *mockCmdRunner) Run(ctx context.Context, name string, args ...string) ([]byte, error) {
	return m.runFn(ctx, name, args...)
}

func TestParseUfwStatus(t *testing.T) {
	parsed := parseUfwStatus(`Status: active

To                         Action      From
--                         ------      ----
[ 1] 22/tcp                 ALLOW IN    Anywhere
[ 2] 53/udp                 DENY IN     Anywhere
`)
	if !parsed.Enabled {
		t.Fatal("expected ufw to be enabled")
	}
	if len(parsed.Rules) != 2 {
		t.Fatalf("rules = %d, want 2", len(parsed.Rules))
	}
	if parsed.Rules[0].Number != 1 || parsed.Rules[0].To != "22/tcp" || parsed.Rules[0].Action != "ALLOW IN" {
		t.Fatalf("unexpected rule[0]: %#v", parsed.Rules[0])
	}
	if parsed.Rules[1].Number != 2 || parsed.Rules[1].To != "53/udp" || parsed.Rules[1].Action != "DENY IN" {
		t.Fatalf("unexpected rule[1]: %#v", parsed.Rules[1])
	}
}

func TestListUfwRulesWithMock(t *testing.T) {
	oldRunner := ufw.CmdRunner
	defer func() { ufw.CmdRunner = oldRunner }()

	ufw.CmdRunner = &mockCmdRunner{
		runFn: func(ctx context.Context, name string, args ...string) ([]byte, error) {
			if len(args) >= 2 && args[0] == "status" && args[1] == "numbered" {
				return []byte(`Status: active

To                         Action      From
--                         ------      ----
[ 1] 8080/tcp               ALLOW IN    Anywhere
`), nil
			}
			if len(args) >= 1 && args[0] == "status" {
				return []byte("Status: active"), nil
			}
			return nil, errors.New("unexpected command")
		},
	}

	svc := &ServerService{}
	status, err := svc.ListUfwRules()
	if err != nil {
		t.Fatalf("ListUfwRules: %v", err)
	}
	if !status.Installed {
		t.Fatal("expected ufw to be installed")
	}
	if !status.Enabled {
		t.Fatal("expected ufw to be enabled")
	}
	if len(status.Rules) != 1 {
		t.Fatalf("rules = %d, want 1", len(status.Rules))
	}
	if status.Rules[0].To != "8080/tcp" {
		t.Errorf("rule.To = %q, want '8080/tcp'", status.Rules[0].To)
	}
}

func TestSetUfwRuleAddsMissingRule(t *testing.T) {
	oldRunner := ufw.CmdRunner
	defer func() { ufw.CmdRunner = oldRunner }()

	var calls []string
	ufw.CmdRunner = &mockCmdRunner{
		runFn: func(ctx context.Context, name string, args ...string) ([]byte, error) {
			calls = append(calls, strings.Join(args, " "))
			if len(args) >= 2 && args[0] == "status" && args[1] == "numbered" {
				return []byte(`Status: active

To                         Action      From
--                         ------      ----
`), nil
			}
			return []byte("ok"), nil
		},
	}

	svc := &ServerService{}
	if err := svc.AllowUfwPort(8080, "tcp"); err != nil {
		t.Fatalf("AllowUfwPort: %v", err)
	}
	want := "allow 8080/tcp"
	if len(calls) != 3 {
		t.Fatalf("calls = %#v, want 3 calls (status, status, numbered, allow)", calls)
	}
	if !strings.Contains(calls[2], want) {
		t.Fatalf("last call = %q, want to contain %q", calls[2], want)
	}
}

func TestSetUfwRuleReplacesMismatchedRule(t *testing.T) {
	oldRunner := ufw.CmdRunner
	defer func() { ufw.CmdRunner = oldRunner }()

	var calls []string
	ufw.CmdRunner = &mockCmdRunner{
		runFn: func(ctx context.Context, name string, args ...string) ([]byte, error) {
			calls = append(calls, strings.Join(args, " "))
			if len(args) >= 2 && args[0] == "status" && args[1] == "numbered" {
				return []byte(`Status: active

To                         Action      From
--                         ------      ----
[ 1] 8080/tcp               DENY IN     Anywhere
`), nil
			}
			if len(args) >= 3 && args[0] == "--force" && args[1] == "delete" {
				return []byte("deleted"), nil
			}
			if len(args) >= 1 && (args[0] == "allow" || args[0] == "deny") {
				return []byte("ok"), nil
			}
			return nil, errors.New("unexpected command")
		},
	}

	svc := &ServerService{}
	if err := svc.AllowUfwPort(8080, "tcp"); err != nil {
		t.Fatalf("AllowUfwPort: %v", err)
	}
	if len(calls) < 3 {
		t.Fatalf("expected at least 3 calls, got %d: %#v", len(calls), calls)
	}
}
