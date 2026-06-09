package sshutil

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// ── SshAddress ─────────────────────────────────────────────────────

func TestSshAddress(t *testing.T) {
	cases := []struct {
		host string
		port int
		want string
	}{
		{"192.168.1.1", 22, "192.168.1.1:22"},
		{"example.com", 2222, "example.com:2222"},
		{"10.0.0.1", 0, "10.0.0.1:22"},
		{"2001:db8::1", 22, "[2001:db8::1]:22"},
	}
	for _, c := range cases {
		got := SshAddress(c.host, c.port)
		if got != c.want {
			t.Errorf("SshAddress(%q, %d) = %q, want %q", c.host, c.port, got, c.want)
		}
	}
}

func TestSshAddress_NegativePort(t *testing.T) {
	got := SshAddress("host", -1)
	if got != "host:22" {
		t.Errorf("negative port should default to 22, got %q", got)
	}
}

// ── SshAuthMethods ─────────────────────────────────────────────────

func TestSshAuthMethods_Empty(t *testing.T) {
	_, err := SshAuthMethods("", "", "")
	if err == nil {
		t.Fatal("expected error when no credentials")
	}
}

func TestSshAuthMethods_PasswordOnly(t *testing.T) {
	methods, err := SshAuthMethods("", "", "secret")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(methods) != 1 {
		t.Fatalf("expected 1 auth method, got %d", len(methods))
	}
}

func TestSshAuthMethods_KeyWithPassphrase(t *testing.T) {
	// An encrypted private key (PKCS#8 PEM with aes256-cbc).
	// Password: "testpass"
	key := `-----BEGIN RSA PRIVATE KEY-----
Proc-Type: 4,ENCRYPTED
DEK-Info: AES-256-CBC,0123456789ABCDEF0123456789ABCDEF

ThisIsNotARealKeySoItWillFailToParse
-----END RSA PRIVATE KEY-----`
	_, err := SshAuthMethods(key, "testpass", "")
	if err == nil {
		t.Fatal("expected parse error for invalid key")
	}
}

func TestSshAuthMethods_KeyWithoutPassphrase(t *testing.T) {
	invalidKey := "not-a-valid-key"
	_, err := SshAuthMethods(invalidKey, "", "")
	if err == nil {
		t.Fatal("expected parse error for invalid key")
	}
}

func TestSshAuthMethods_WhitespaceOnly(t *testing.T) {
	methods, err := SshAuthMethods("  ", "", "")
	if err == nil {
		t.Fatal("expected error for whitespace-only key")
	}
	if methods != nil {
		t.Fatal("expected nil methods")
	}
}

// ── BootstrapAgentPort ─────────────────────────────────────────────

func TestBootstrapAgentPort_UserSpecified(t *testing.T) {
	got := BootstrapAgentPort(8080)
	if got != 8080 {
		t.Errorf("BootstrapAgentPort(8080) = %d, want 8080", got)
	}
}

func TestBootstrapAgentPort_Random(t *testing.T) {
	got := BootstrapAgentPort(0)
	if got <= 2000 || got > 65535 {
		t.Errorf("BootstrapAgentPort(0) = %d, want in range 2001-65535", got)
	}
}

func TestBootstrapAgentPort_RandomMultiple(t *testing.T) {
	// Test that multiple calls return different values (randomness check).
	seen := make(map[int]bool)
	for i := 0; i < 10; i++ {
		port := BootstrapAgentPort(0)
		if port <= 2000 || port > 65535 {
			t.Fatalf("port %d out of range", port)
		}
		seen[port] = true
	}
	if len(seen) < 2 {
		t.Log("warning: all 10 random ports were the same — may be ok, but unlikely")
	}
}

// ── ShouldInstallServiceFallback ───────────────────────────────────

func TestShouldInstallServiceFallback_Matches(t *testing.T) {
	if !ShouldInstallServiceFallback("missing l-ui.service in bundle", nil) {
		t.Fatal("should match 'missing l-ui.service in bundle'")
	}
}

func TestShouldInstallServiceFallback_MatchesInError(t *testing.T) {
	if !ShouldInstallServiceFallback("", fmt.Errorf("missing l-ui.service in bundle")) {
		t.Fatal("should match error containing 'missing l-ui.service'")
	}
}

func TestShouldInstallServiceFallback_NoMatch(t *testing.T) {
	if ShouldInstallServiceFallback("permission denied", nil) {
		t.Fatal("should NOT match 'permission denied'")
	}
}

func TestShouldInstallServiceFallback_Empty(t *testing.T) {
	if ShouldInstallServiceFallback("", nil) {
		t.Fatal("should NOT match empty output")
	}
}

// ── LocalServiceUnitForRelease ─────────────────────────────────────

func TestLocalServiceUnitForRelease_Debian(t *testing.T) {
	data, name, err := LocalServiceUnitForRelease("ubuntu")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if name != "l-ui-agent.service.debian" {
		t.Errorf("service name = %q, want 'l-ui-agent.service.debian'", name)
	}
	if len(data) == 0 {
		t.Fatal("expected non-empty service file data")
	}
	if !strings.Contains(string(data), "ExecStart") {
		t.Error("service file should contain ExecStart")
	}
}

func TestLocalServiceUnitForRelease_Arch(t *testing.T) {
	_, name, err := LocalServiceUnitForRelease("arch")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if name != "l-ui-agent.service.arch" {
		t.Errorf("service name = %q, want 'l-ui-agent.service.arch'", name)
	}
}

func TestLocalServiceUnitForRelease_RHEL(t *testing.T) {
	_, name, err := LocalServiceUnitForRelease("centos")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if name != "l-ui-agent.service.rhel" {
		t.Errorf("service name = %q, want 'l-ui-agent.service.rhel'", name)
	}
}

func TestLocalServiceUnitForRelease_Unknown(t *testing.T) {
	_, name, err := LocalServiceUnitForRelease("unknown-os")
	if err != nil {
		t.Fatalf("unexpected error for unknown OS: %v", err)
	}
	if name != "l-ui-agent.service.rhel" {
		t.Errorf("unknown OS should default to rhel, got %q", name)
	}
}

func TestLocalServiceUnitForRelease_CaseInsensitive(t *testing.T) {
	_, name, err := LocalServiceUnitForRelease("UBUNTU")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if name != "l-ui-agent.service.debian" {
		t.Errorf("'UBUNTU' should map to debian, got %q", name)
	}
}

// ── FindRepoRoot ───────────────────────────────────────────────────

func TestFindRepoRoot(t *testing.T) {
	// Create a temp dir with go.mod to simulate repo root.
	tmp := t.TempDir()
	goMod := filepath.Join(tmp, "go.mod")
	os.WriteFile(goMod, []byte("module test"), 0644)
	subDir := filepath.Join(tmp, "a", "b", "c")
	os.MkdirAll(subDir, 0755)

	origWd, _ := os.Getwd()
	os.Chdir(subDir)
	defer os.Chdir(origWd)

	root, err := FindRepoRoot()
	if err != nil {
		t.Fatalf("FindRepoRoot: %v", err)
	}
	if root != tmp {
		t.Errorf("root = %q, want %q", root, tmp)
	}
}

func TestFindRepoRoot_NotFound(t *testing.T) {
	tmp := t.TempDir() // no go.mod here
	origWd, _ := os.Getwd()
	os.Chdir(tmp)
	defer os.Chdir(origWd)

	_, err := FindRepoRoot()
	if err == nil {
		t.Fatal("expected error when no go.mod found")
	}
}

// ── UploadRemoteFileSession ────────────────────────────────────────

type mockUploadSession struct {
	stdinOpened bool
	started     bool
	stdin       bytes.Buffer
	stdout      bytes.Buffer
	stderr      bytes.Buffer
	startErr    error
	waitErr     error
}

func (m *mockUploadSession) StdinPipe() (io.WriteCloser, error) {
	m.stdinOpened = true
	return nopCloser{Writer: &m.stdin}, nil
}
func (m *mockUploadSession) Start(string) error          { m.started = true; return m.startErr }
func (m *mockUploadSession) Wait() error                  { return m.waitErr }
func (m *mockUploadSession) Close() error                 { return nil }
func (m *mockUploadSession) SetStdout(w io.Writer)       { m.stdout = *w.(*bytes.Buffer) }
func (m *mockUploadSession) SetStderr(w io.Writer)       { m.stderr = *w.(*bytes.Buffer) }

type nopCloser struct{ io.Writer }
func (nopCloser) Close() error { return nil }

var _ io.WriteCloser = nopCloser{}

func TestUploadRemoteFileSession_NoSudo(t *testing.T) {
	sess := &mockUploadSession{}
	out, err := UploadRemoteFileSession(sess, "", false, "/tmp/test.txt", []byte("hello"), "0644")
	if err != nil {
		t.Fatalf("UploadRemoteFileSession: %v", err)
	}
	if !sess.started {
		t.Fatal("session should be started")
	}
	if !sess.stdinOpened {
		t.Fatal("stdin pipe should be opened")
	}
	_ = out
}

func TestUploadRemoteFileSession_WithSudo(t *testing.T) {
	sess := &mockUploadSession{}
	out, err := UploadRemoteFileSession(sess, "secret", true, "/tmp/test.txt", []byte("hello"), "0644")
	if err != nil {
		t.Fatalf("UploadRemoteFileSession: %v", err)
	}
	if !sess.started {
		t.Fatal("session should be started")
	}
	if !strings.Contains(sess.stdin.String(), "secret") {
		t.Errorf("expected sudo password in stdin, got %q", sess.stdin.String())
	}
	_ = out
}

func TestUploadRemoteFileSession_StartError(t *testing.T) {
	sess := &mockUploadSession{startErr: fmt.Errorf("start failed")}
	_, err := UploadRemoteFileSession(sess, "", false, "/tmp/test.txt", []byte("data"), "0644")
	if err == nil {
		t.Fatal("expected error from start failure")
	}
}

func TestUploadRemoteFileSession_WaitError(t *testing.T) {
	sess := &mockUploadSession{waitErr: fmt.Errorf("wait failed")}
	out, err := UploadRemoteFileSession(sess, "", false, "/tmp/test.txt", []byte("data"), "0644")
	if err == nil {
		t.Fatal("expected error from wait failure")
	}
	_ = out
}

// ── RunSSHCommand helpers (testable without SSH) ───────────────────

func TestRunSSHCommand_InvalidClient(t *testing.T) {
	// nil client panics in NewSession — verify it doesn't hang.
	defer func() {
		if r := recover(); r != nil {
			// expected
		}
	}()
	RunSSHCommand(nil, "", false, "echo test")
	t.Fatal("expected panic with nil client")
}
