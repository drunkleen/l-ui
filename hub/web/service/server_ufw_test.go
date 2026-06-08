package service

import (
	"errors"
	"reflect"
	"runtime"
	"strings"
	"testing"
)

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

func TestSetUfwRuleAddsMissingRule(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("ufw tests are linux-only")
	}
	oldLookPath := ufwLookPath
	oldRunCommand := ufwRunCommand
	defer func() {
		ufwLookPath = oldLookPath
		ufwRunCommand = oldRunCommand
	}()
	ufwLookPath = func(string) (string, error) { return "/usr/sbin/ufw", nil }
	var calls []string
	ufwRunCommand = func(name string, args ...string) ([]byte, error) {
		calls = append(calls, strings.Join(args, " "))
		if len(args) >= 2 && args[0] == "status" && args[1] == "numbered" {
			return []byte(`Status: active

To                         Action      From
--                         ------      ----
`), nil
		}
		return []byte("ok"), nil
	}

	svc := &ServerService{}
	if err := svc.AllowUfwPort(8080, "tcp"); err != nil {
		t.Fatalf("AllowUfwPort: %v", err)
	}
	want := []string{"status numbered", "allow 8080/tcp"}
	if !reflect.DeepEqual(calls, want) {
		t.Fatalf("calls = %#v, want %#v", calls, want)
	}
}

func TestSetUfwRuleReplacesMismatchedRule(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("ufw tests are linux-only")
	}
	oldLookPath := ufwLookPath
	oldRunCommand := ufwRunCommand
	defer func() {
		ufwLookPath = oldLookPath
		ufwRunCommand = oldRunCommand
	}()
	ufwLookPath = func(string) (string, error) { return "/usr/sbin/ufw", nil }
	var calls []string
	ufwRunCommand = func(name string, args ...string) ([]byte, error) {
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
		if len(args) >= 2 && args[0] == "allow" {
			return []byte("ok"), nil
		}
		return nil, errors.New("unexpected command")
	}

	svc := &ServerService{}
	if err := svc.AllowUfwPort(8080, "tcp"); err != nil {
		t.Fatalf("AllowUfwPort: %v", err)
	}
	want := []string{"status numbered", "--force delete deny 8080/tcp", "allow 8080/tcp"}
	if !reflect.DeepEqual(calls, want) {
		t.Fatalf("calls = %#v, want %#v", calls, want)
	}
}
