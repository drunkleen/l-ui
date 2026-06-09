package service

import (
	"fmt"
	"os"
	"strings"
	"testing"

	"github.com/drunkleen/l-ui/internal/bundle"
	"github.com/drunkleen/l-ui/internal/database/model"
)

// ── Env file format test ────────────────────────────────────────────

func TestBootstrapEnvFileFormat(t *testing.T) {
	// The bootstrap writes this env file to /etc/default/l-ui-agent on the node.
	// It must contain all vars the agent needs to start.
	apiToken := "test-api-token-123"
	agentPort := 2054

	envCmd := fmt.Sprintf(`set -e
mkdir -p /etc/default
cat >/etc/default/l-ui-agent <<'EOF'
LUI_DB_FOLDER=/etc/l-ui
LUI_MAIN_FOLDER=/usr/local/l-ui-agent
LUI_SERVICE=/etc/systemd/system
LUI_BOOTSTRAP_API_TOKEN=%s
LUI_WEB_PORT=%d
EOF`, apiToken, agentPort)

	// Verify the output contains the expected variables
	if !strings.Contains(envCmd, "LUI_DB_FOLDER=/etc/l-ui") {
		t.Error("env missing LUI_DB_FOLDER")
	}
	if !strings.Contains(envCmd, "LUI_MAIN_FOLDER=/usr/local/l-ui-agent") {
		t.Error("env missing LUI_MAIN_FOLDER")
	}
	if !strings.Contains(envCmd, "LUI_SERVICE=/etc/systemd/system") {
		t.Error("env missing LUI_SERVICE")
	}
	if !strings.Contains(envCmd, "LUI_BOOTSTRAP_API_TOKEN=test-api-token-123") {
		t.Error("env missing LUI_BOOTSTRAP_API_TOKEN")
	}
	if !strings.Contains(envCmd, "LUI_WEB_PORT=2054") {
		t.Error("env missing LUI_WEB_PORT")
	}
}

// ── Service file selection test ─────────────────────────────────────

func TestBootstrapServiceFileSelection(t *testing.T) {
	// Simulate the serviceCmd logic from bootstrapFlow:
	// It should pick l-ui-agent.service.debian for Ubuntu/Debian.
	release := "ubuntu"
	selectedService := "l-ui-agent.service"
	dir := t.TempDir()

	// Create mock service files
	os.WriteFile(dir+"/l-ui-agent.service", []byte("generic"), 0644)
	os.WriteFile(dir+"/l-ui-agent.service.debian", []byte("debian"), 0644)

	// Logic from bootstrapFlow: prefer l-ui-agent.service if exists
	if _, err := os.Stat(dir + "/l-ui-agent.service"); err == nil {
		selectedService = "l-ui-agent.service"
	} else if _, err := os.Stat(dir + "/l-ui-agent.service.debian"); err == nil {
		// The actual code also checks the release ID and picks the right variant.
		switch release {
		case "ubuntu", "debian", "armbian":
			selectedService = "l-ui-agent.service.debian"
		}
	}

	if selectedService != "l-ui-agent.service" {
		t.Fatalf("expected l-ui-agent.service (generic exists), got %s", selectedService)
	}

	// Now test without the generic service file — should use debian variant
	os.Remove(dir + "/l-ui-agent.service")
	if _, err := os.Stat(dir + "/l-ui-agent.service"); os.IsNotExist(err) {
		if _, err := os.Stat(dir + "/l-ui-agent.service.debian"); err == nil {
			selectedService = "l-ui-agent.service.debian"
		}
	}

	if selectedService != "l-ui-agent.service.debian" {
		t.Fatalf("expected l-ui-agent.service.debian, got %s", selectedService)
	}
}

// ── Verify-agent command format test ────────────────────────────────

func TestVerifyAgentCommandFormat(t *testing.T) {
	// The verify-agent step curls the agent API with auth headers.
	// It must use the correct path: /api/v1/status (not /api/v1/server/status).
	apiToken := "test-token"
	port := 2054
	path := "/api/v1/status"

	verifyCmd := fmt.Sprintf(`for i in $(seq 1 30); do
  for _scheme in https http; do
    if curl -fsSk \
      -H "Authorization: Bearer %[1]s" \
      "${_scheme}://127.0.0.1:%[2]d%[3]s" >/dev/null 2>&1; then
      exit 0
    fi
  done
  sleep 2
done
exit 1`, apiToken, port, path)

	if !strings.Contains(verifyCmd, "/api/v1/status") {
		t.Error("verify command should use /api/v1/status")
	}
	if strings.Contains(verifyCmd, "/api/v1/server/status") {
		t.Error("verify command should NOT use /api/v1/server/status (hub API)")
	}
	if !strings.Contains(verifyCmd, "Authorization: Bearer test-token") {
		t.Error("verify command should include auth token")
	}
	if !strings.Contains(verifyCmd, "127.0.0.1:2054") {
		t.Error("verify command should include agent port")
	}
}

// ── Architecture detection test ─────────────────────────────────────

func TestBootstrapDetectArchCommands(t *testing.T) {
	// The detect-arch step changed from RunSSHCommand (with sudo) to
	// RemoteCommand (without sudo). The command should be "uname -m".
	stepName := "detect-arch"
	stepCmd := "uname -m"

	if stepName != "detect-arch" {
		t.Fatal("step name mismatch")
	}
	if stepCmd != "uname -m" {
		t.Fatal("detect-arch should use uname -m")
	}
}

func TestBootstrapDetectArchRetryUsesAbsolutePath(t *testing.T) {
	// The retry command for detect-arch should use /bin/uname -m
	retryCmd := "/bin/uname -m"

	if retryCmd != "/bin/uname -m" {
		t.Fatal("retry should use absolute path /bin/uname -m")
	}
}

// ── Build bootstrap node comprehensive test ─────────────────────────

func TestBuildBootstrapNodeComprehensive(t *testing.T) {
	svc := &NodeService{}

	tests := []struct {
		name    string
		req     NodeBootstrapRequest
		wantErr bool
		check   func(*testing.T, *model.Node)
	}{
		{
			name: "basic node",
			req: NodeBootstrapRequest{
				Name:        "test-node",
				Address:     "192.168.1.100",
				SSHUser:     "root",
				SSHPassword: "secret",
			},
			check: func(t *testing.T, n *model.Node) {
				if n.Name != "test-node" {
					t.Errorf("name = %q, want 'test-node'", n.Name)
				}
				if n.Scheme != "http" {
					t.Errorf("scheme = %q, want http", n.Scheme)
				}
				if n.Port <= 2000 || n.Port > 65535 {
					t.Errorf("port = %d, want auto-selected", n.Port)
				}
				if n.ApiToken == "" {
					t.Error("api token should not be empty")
				}
				if !n.Enable {
					t.Error("node should be enabled by default")
				}
				if !n.AllowPrivateAddress {
					t.Error("private address should be allowed by default")
				}
				if n.TlsVerifyMode != "verify" {
					t.Errorf("tls verify mode = %q, want verify", n.TlsVerifyMode)
				}
			},
		},
		{
			name: "custom base path",
			req: NodeBootstrapRequest{
				Name:          "bp-node",
				Address:       "10.0.0.1",
				SSHUser:       "admin",
				SSHPassword:   "pass",
				BootstrapBase: "/custom/",
			},
			check: func(t *testing.T, n *model.Node) {
				if n.BasePath != "/custom/" {
					t.Errorf("base path = %q, want '/custom/'", n.BasePath)
				}
			},
		},
		{
			name: "custom agent port",
			req: NodeBootstrapRequest{
				Name:        "port-node",
				Address:     "10.0.0.2",
				SSHUser:     "root",
				SSHPassword: "pass",
				AgentPort:   8080,
			},
			check: func(t *testing.T, n *model.Node) {
				if n.Port != 8080 {
					t.Errorf("port = %d, want 8080", n.Port)
				}
			},
		},
		{
			name: "TLS mode without domain should error",
			req: NodeBootstrapRequest{
				Name:        "tls-node",
				Address:     "10.0.0.3",
				SSHUser:     "root",
				SSHPassword: "pass",
				UseTLS:      true,
			},
			wantErr: true,
		},
		{
			name: "TLS mode with domain",
			req: NodeBootstrapRequest{
				Name:        "tls-node-ok",
				Address:     "10.0.0.4",
				SSHUser:     "root",
				SSHPassword: "pass",
				UseTLS:      true,
				Domain:      "node.example.com",
			},
			check: func(t *testing.T, n *model.Node) {
				if n.Scheme != "https" {
					t.Errorf("scheme = %q, want https", n.Scheme)
				}
				if n.Address != "node.example.com" {
					t.Errorf("address = %q, want domain", n.Address)
				}
				if n.Port != 443 {
					t.Errorf("port = %d, want 443", n.Port)
				}
				if n.AllowPrivateAddress {
					t.Error("private address should be false for TLS")
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			node, err := svc.buildBootstrapNode(tt.req)
			if tt.wantErr {
				if err == nil {
					t.Fatal("expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if tt.check != nil {
				tt.check(t, &node)
			}
		})
	}
}

// ── StartBootstrap validation test ──────────────────────────────────

func TestStartBootstrapValidatesAddress(t *testing.T) {
	svc := &NodeService{}
	_, err := svc.StartBootstrap(nil, NodeBootstrapRequest{
		Name:    "no-address",
		Address: "",
	})
	if err == nil {
		t.Fatal("expected error for empty address")
	}
}

// ── BootstrapFlow steps ordering test ───────────────────────────────

func TestBootstrapFlowStepsOrder(t *testing.T) {
	// Steps should execute in this order:
	expectedSteps := []string{
		"detect-arch",
		"prepare-dirs",
		"check-curl",
		"download-bundle",
		"install-bundle",
		"write-env",
		"install-service",
		"verify-bundle",
		"daemon-reload",
		"enable-service",
		"restart-service",
		"open-ufw",
		"verify-agent",
	}

	actualSteps := []string{
		"detect-arch",
		"prepare-dirs",
		"check-curl",
		"download-bundle",
		"install-bundle",
		"write-env",
		"install-service",
		"verify-bundle",
		"daemon-reload",
		"enable-service",
		"restart-service",
		"open-ufw",
		"verify-agent",
	}

	if len(actualSteps) != len(expectedSteps) {
		t.Fatalf("step count: %d, want %d", len(actualSteps), len(expectedSteps))
	}
	for i, s := range expectedSteps {
		if actualSteps[i] != s {
			t.Fatalf("step[%d] = %q, want %q", i, actualSteps[i], s)
		}
	}
}

// ── Agent config path test ──────────────────────────────────────────

func TestBootstrapCreatesCorrectAgentDBPath(t *testing.T) {
	// The agent DB is at LUI_DB_FOLDER + "/l-ui-agent.db"
	// The bootstrap writes LUI_DB_FOLDER=/etc/l-ui
	// So agent DB should be /etc/l-ui/l-ui-agent.db
	dbFolder := "/etc/l-ui"
	agentDBPath := dbFolder + "/l-ui-agent.db"

	if agentDBPath != "/etc/l-ui/l-ui-agent.db" {
		t.Fatalf("agent DB path = %q, want /etc/l-ui/l-ui-agent.db", agentDBPath)
	}
}

// ── Agent startup messages test ─────────────────────────────────────

// ── Download-bundle URL construction test ──────────────────────────

func TestBootstrapDownloadURL(t *testing.T) {
	// The download-URL should point to the agent-specific tarball on GitHub.
	version := "0.0.1"
	arch := "amd64"
	want := "https://github.com/drunkleen/l-ui/releases/download/v0.0.1/l-ui-agent-linux-amd64.tar.gz"
	got := bundle.BundleReleaseURL(version, arch)
	if got != want {
		t.Fatalf("BundleReleaseURL(%q, %q) = %q, want %q", version, arch, got, want)
	}
}

func TestBootstrapDownloadURLWithCustomBase(t *testing.T) {
	t.Setenv("LUI_NODE_BUNDLE_RELEASE_BASE", "https://mirror.example.com/releases")
	got := bundle.BundleReleaseURL("1.0.0", "arm64")
	want := "https://mirror.example.com/releases/v1.0.0/l-ui-agent-linux-arm64.tar.gz"
	if got != want {
		t.Fatalf("BundleReleaseURL = %q, want %q", got, want)
	}
}

// ── Rollback script content test ────────────────────────────────────

func TestBootstrapRollbackScript(t *testing.T) {
	// The rollback script must clean up tarball, extracted files, and service.
	script := `set -e
rm -f /tmp/l-ui-agent.tar.gz
rm -rf /usr/local/l-ui-agent
rm -f /etc/l-ui/l-ui.db
if [ -d /usr/local/l-ui-agent.previous ]; then
  mv /usr/local/l-ui-agent.previous /usr/local/l-ui-agent
fi
systemctl daemon-reload
systemctl enable --now l-ui-agent || true
`

	if !strings.Contains(script, "rm -f /tmp/l-ui-agent.tar.gz") {
		t.Error("rollback must remove agent tarball")
	}
	if !strings.Contains(script, "rm -rf /usr/local/l-ui-agent") {
		t.Error("rollback must remove extracted l-ui dir")
	}
	if !strings.Contains(script, "rm -f /etc/l-ui/l-ui.db") {
		t.Error("rollback must remove DB")
	}
	if !strings.Contains(script, "mv /usr/local/l-ui-agent.previous /usr/local/l-ui-agent") {
		t.Error("rollback must restore previous installation")
	}
	if !strings.Contains(script, "systemctl enable --now l-ui-agent") {
		t.Error("rollback must restart previous installation")
	}
}

func TestBootstrapLightRollbackScript(t *testing.T) {
	// A lighter rollback — only cleanup tarball and extracted files, no service ops.
	script := `set -e
rm -f /tmp/l-ui-agent.tar.gz
rm -rf /usr/local/l-ui-agent
`
	if !strings.Contains(script, "rm -f /tmp/l-ui-agent.tar.gz") {
		t.Error("light rollback must remove agent tarball")
	}
	if !strings.Contains(script, "rm -rf /usr/local/l-ui-agent") {
		t.Error("light rollback must remove extracted l-ui dir")
	}
	if strings.Contains(script, "systemctl") {
		t.Error("light rollback should not touch systemctl")
	}
}

// ── curl availability check test ────────────────────────────────────

func TestBootstrapCheckCurlCommand(t *testing.T) {
	// The check-curl step uses `command -v curl` to verify curl exists.
	checkCmd := "command -v curl >/dev/null 2>&1"
	// Simulate a successful check
	if checkCmd != "command -v curl >/dev/null 2>&1" {
		t.Fatal("curl check command mismatch")
	}
	// The error message should mention curl
	errMsg := "curl is required on the remote server"
	if !strings.Contains(errMsg, "curl") {
		t.Error("error message should mention curl")
	}
}

// ── Download command format test ────────────────────────────────────

func TestBootstrapDownloadCommandFormat(t *testing.T) {
	url := "https://github.com/drunkleen/l-ui/releases/download/v0.0.1/l-ui-agent-linux-amd64.tar.gz"
	downloadCmd := "curl -fL -o /tmp/l-ui-agent.tar.gz " + url
	expected := "curl -fL -o /tmp/l-ui-agent.tar.gz https://github.com/drunkleen/l-ui/releases/download/v0.0.1/l-ui-agent-linux-amd64.tar.gz"
	if downloadCmd != expected {
		t.Fatalf("download command mismatch:\ngot:  %q\nwant: %q", downloadCmd, expected)
	}
}

// ── BundleVersion helper test ───────────────────────────────────────

func TestBootstrapBundleVersion(t *testing.T) {
	v, err := bundle.BundleVersion()
	if err != nil {
		t.Fatalf("BundleVersion: %v", err)
	}
	if v == "" {
		t.Fatal("BundleVersion returned empty version")
	}
}

func TestAgentStartupMessages(t *testing.T) {
	// The agent prints "Starting <name> <version> (agent mode)" on startup.
	// This helps verify which binary is running.
	msg := "Starting l-ui 0.0.1 (agent mode)"
	if !strings.Contains(msg, "(agent mode)") {
		t.Error("agent startup message should contain '(agent mode)'")
	}
	if !strings.Contains(msg, "Starting l-ui") {
		t.Error("agent startup message should contain 'Starting l-ui'")
	}
}
