package cmd

import (
	"os"
	"os/exec"
	"testing"

	"github.com/spf13/cobra"
)

func TestCertFlagParsing(t *testing.T) {
	settingReset = false
	settingWebCert = ""
	settingWebCertKey = ""

	err := certCmd.ParseFlags([]string{"--webCert", "/path/to/cert", "--webCertKey", "/path/to/key"})
	if err != nil {
		t.Fatalf("ParseFlags failed: %v", err)
	}

	if settingWebCert != "/path/to/cert" {
		t.Errorf("webCert = %q, want /path/to/cert", settingWebCert)
	}
	if settingWebCertKey != "/path/to/key" {
		t.Errorf("webCertKey = %q, want /path/to/key", settingWebCertKey)
	}
}

func TestCertResetFlagParsing(t *testing.T) {
	settingReset = false
	err := certCmd.ParseFlags([]string{"--reset"})
	if err != nil {
		t.Fatalf("ParseFlags failed: %v", err)
	}
	if !settingReset {
		t.Error("expected reset to be true")
	}
}

func TestCertCommandExists(t *testing.T) {
	if certCmd == nil {
		t.Fatal("certCmd is nil")
	}
	if certCmd.Use != "cert" {
		t.Errorf("Use = %q, want cert", certCmd.Use)
	}
}

func TestSSLCommandsExist(t *testing.T) {
	cmds := []struct {
		cmd  *cobra.Command
		name string
	}{
		{sslCmd, "ssl"},
		{sslIssueCmd, "issue"},
		{sslIssueIPCmd, "issue-ip"},
		{sslIssueCFCmd, "issue-cf"},
		{sslShowCmd, "show"},
		{sslRenewCmd, "renew"},
	}

	for _, tc := range cmds {
		t.Run(tc.name, func(t *testing.T) {
			if tc.cmd == nil {
				t.Fatalf("%sCmd is nil", tc.name)
			}
		})
	}
}

func TestSslIssueDomainFlags(t *testing.T) {
	origExec := execCommand
	origAcmeSh := acmeShInstalledFunc
	defer func() {
		execCommand = origExec
		acmeShInstalledFunc = origAcmeSh
	}()

	var capturedArgs [][]string
	execCommand = func(name string, args ...string) *exec.Cmd {
		capturedArgs = append(capturedArgs, append([]string{name}, args...))
		return exec.Command("echo")
	}
	acmeShInstalledFunc = func() bool { return true }

	sslDomain = "example.com"
	cmd := &cobra.Command{}
	runSslIssue(cmd, nil)

	if len(capturedArgs) != 1 {
		t.Fatalf("expected 1 exec call, got %d", len(capturedArgs))
	}
	expected := []string{"--issue", "--standalone", "-d", "example.com", "--server", "letsencrypt", "--keylength", "2048"}
	for i, exp := range expected {
		if capturedArgs[0][i+1] != exp {
			t.Errorf("arg[%d] = %q, want %q", i+1, capturedArgs[0][i+1], exp)
		}
	}
}

func TestSslIssueIPFlags(t *testing.T) {
	origExec := execCommand
	origAcmeSh := acmeShInstalledFunc
	defer func() {
		execCommand = origExec
		acmeShInstalledFunc = origAcmeSh
	}()

	var capturedArgs [][]string
	execCommand = func(name string, args ...string) *exec.Cmd {
		capturedArgs = append(capturedArgs, append([]string{name}, args...))
		return exec.Command("echo")
	}
	acmeShInstalledFunc = func() bool { return true }

	sslIP = "1.2.3.4"
	cmd := &cobra.Command{}
	runSslIssueIP(cmd, nil)

	if len(capturedArgs) != 1 {
		t.Fatalf("expected 1 exec call, got %d", len(capturedArgs))
	}
	// Should be: --issue --standalone -d 1.2.3.4 --ip 1.2.3.4
	expected := []string{"--issue", "--standalone", "-d", "1.2.3.4", "--ip", "1.2.3.4", "--server", "letsencrypt", "--keylength", "2048"}
	for i, exp := range expected {
		if capturedArgs[0][i+1] != exp {
			t.Errorf("arg[%d] = %q, want %q", i+1, capturedArgs[0][i+1], exp)
		}
	}
}

func TestSslIssueCFFlags(t *testing.T) {
	origExec := execCommand
	origAcmeSh := acmeShInstalledFunc
	defer func() {
		execCommand = origExec
		acmeShInstalledFunc = origAcmeSh
	}()

	var capturedArgs [][]string
	execCommand = func(name string, args ...string) *exec.Cmd {
		capturedArgs = append(capturedArgs, append([]string{name}, args...))
		return exec.Command("echo")
	}
	acmeShInstalledFunc = func() bool { return true }

	sslDomain = "example.com"
	cmd := &cobra.Command{}
	runSslIssueCF(cmd, nil)

	if len(capturedArgs) != 1 {
		t.Fatalf("expected 1 exec call, got %d", len(capturedArgs))
	}
	// Should be: --issue --dns dns_cf -d example.com
	expected := []string{"--issue", "--dns", "dns_cf", "-d", "example.com"}
	for i, exp := range expected {
		if capturedArgs[0][i+1] != exp {
			t.Errorf("arg[%d] = %q, want %q", i+1, capturedArgs[0][i+1], exp)
		}
	}
}

func TestSslRenewFlags(t *testing.T) {
	origExec := execCommand
	origAcmeSh := acmeShInstalledFunc
	defer func() {
		execCommand = origExec
		acmeShInstalledFunc = origAcmeSh
	}()

	var capturedArgs [][]string
	execCommand = func(name string, args ...string) *exec.Cmd {
		capturedArgs = append(capturedArgs, append([]string{name}, args...))
		return exec.Command("echo")
	}
	acmeShInstalledFunc = func() bool { return true }

	sslDomain = "example.com"
	cmd := &cobra.Command{}
	runSslRenew(cmd, nil)

	if len(capturedArgs) != 1 {
		t.Fatalf("expected 1 exec call, got %d", len(capturedArgs))
	}
	// Should be: --renew -d example.com
	expected := []string{"--renew", "-d", "example.com"}
	for i, exp := range expected {
		if capturedArgs[0][i+1] != exp {
			t.Errorf("arg[%d] = %q, want %q", i+1, capturedArgs[0][i+1], exp)
		}
	}
}

func TestSslIssueRequiresDomain(t *testing.T) {
	origAcmeSh := acmeShInstalledFunc
	defer func() { acmeShInstalledFunc = origAcmeSh }()
	acmeShInstalledFunc = func() bool { return true }

	sslDomain = ""
	cmd := &cobra.Command{}
	runSslIssue(cmd, nil)
	// Should return early without calling execCommand
}

func TestSslIssueIPRequiresIP(t *testing.T) {
	origAcmeSh := acmeShInstalledFunc
	defer func() { acmeShInstalledFunc = origAcmeSh }()
	acmeShInstalledFunc = func() bool { return true }

	sslIP = ""
	cmd := &cobra.Command{}
	runSslIssueIP(cmd, nil)
	// Should return early without calling execCommand
}

func TestSslIssueCFRequiresDomain(t *testing.T) {
	origAcmeSh := acmeShInstalledFunc
	defer func() { acmeShInstalledFunc = origAcmeSh }()
	acmeShInstalledFunc = func() bool { return true }

	sslDomain = ""
	cmd := &cobra.Command{}
	runSslIssueCF(cmd, nil)
	// Should return early without calling execCommand
}

func TestSslIssueAcmeShNotInstalled(t *testing.T) {
	origAcmeSh := acmeShInstalledFunc
	defer func() { acmeShInstalledFunc = origAcmeSh }()
	acmeShInstalledFunc = func() bool { return false }

	sslDomain = "example.com"
	cmd := &cobra.Command{}
	runSslIssue(cmd, nil)
	// Should return early without calling execCommand
}

func TestSslIssueIPAcmeShNotInstalled(t *testing.T) {
	origAcmeSh := acmeShInstalledFunc
	defer func() { acmeShInstalledFunc = origAcmeSh }()
	acmeShInstalledFunc = func() bool { return false }

	sslIP = "1.2.3.4"
	cmd := &cobra.Command{}
	runSslIssueIP(cmd, nil)
	// Should return early without calling execCommand
}

func TestSslIssueCFAcmeShNotInstalled(t *testing.T) {
	origAcmeSh := acmeShInstalledFunc
	defer func() { acmeShInstalledFunc = origAcmeSh }()
	acmeShInstalledFunc = func() bool { return false }

	sslDomain = "example.com"
	cmd := &cobra.Command{}
	runSslIssueCF(cmd, nil)
	// Should return early without calling execCommand
}

func TestSslRenewAcmeShNotInstalled(t *testing.T) {
	origAcmeSh := acmeShInstalledFunc
	defer func() { acmeShInstalledFunc = origAcmeSh }()
	acmeShInstalledFunc = func() bool { return false }

	sslDomain = "example.com"
	cmd := &cobra.Command{}
	runSslRenew(cmd, nil)
	// Should return early without calling execCommand
}

func TestAcmeShInstalledImpl(t *testing.T) {
	origPath := acmeShPath
	defer func() { acmeShPath = origPath }()

	t.Run("acme.sh exists", func(t *testing.T) {
		// Create a temp file to simulate acme.sh
		tmpFile, err := os.CreateTemp("", "acme.sh")
		if err != nil {
			t.Fatalf("failed to create temp file: %v", err)
		}
		defer os.Remove(tmpFile.Name())
		tmpFile.Close()

		acmeShPath = tmpFile.Name()
		if !acmeShInstalledImpl() {
			t.Error("acmeShInstalledImpl() = false, want true")
		}
	})

	t.Run("acme.sh does not exist", func(t *testing.T) {
		acmeShPath = "/nonexistent/path/to/acme.sh"
		if acmeShInstalledImpl() {
			t.Error("acmeShInstalledImpl() = true, want false")
		}
	})
}
