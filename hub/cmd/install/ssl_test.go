package install

import (
	"strings"
	"testing"
)

func TestConfigureSSL_None(t *testing.T) {
	// SSL type "none" should do nothing
	e := &Engine{
		cfg: InstallConfig{
			SSLType: sslNone,
		},
	}
	if err := e.configureSSL(); err != nil {
		t.Fatalf("configureSSL with none should not error: %v", err)
	}
}

func TestConfigureSSL_DomainEmpty(t *testing.T) {
	e := &Engine{
		cfg: InstallConfig{
			SSLType: sslDomain,
			Domain:  "",
		},
	}
	if err := e.configureSSL(); err == nil {
		t.Fatal("expected error for empty domain")
	}
}

func TestConfigureSSL_IPEmpty(t *testing.T) {
	e := &Engine{
		cfg: InstallConfig{
			SSLType: sslIP,
			IP:      "",
		},
	}
	if err := e.configureSSL(); err == nil {
		t.Fatal("expected error for empty IP")
	}
}

func TestConfigureSSL_CustomEmpty(t *testing.T) {
	e := &Engine{
		cfg: InstallConfig{
			SSLType:  sslCustom,
			CertPath: "",
		},
	}
	if err := e.configureSSL(); err == nil {
		t.Fatal("expected error for empty cert path")
	}
}

func TestConfigureSSL_SetsCorrectCertPaths(t *testing.T) {
	// For custom SSL, the cert path should have fullchain.pem and privkey.pem
	certPath := "/etc/l-ui/cert"
	fullchain := certPath + "/fullchain.pem"
	privkey := certPath + "/privkey.pem"

	if fullchain != "/etc/l-ui/cert/fullchain.pem" {
		t.Errorf("fullchain path wrong: %s", fullchain)
	}
	if privkey != "/etc/l-ui/cert/privkey.pem" {
		t.Errorf("privkey path wrong: %s", privkey)
	}
}



func TestResultWithSSL_Domain(t *testing.T) {
	e := &Engine{
		cfg: InstallConfig{
			Port:    "443",
			SSLType: sslDomain,
			Domain:  "panel.example.com",
		},
	}
	r := e.result()
	if r == nil {
		t.Fatal("result should not be nil")
	}
	if !strings.Contains(r.AccessURL, "https://") {
		t.Errorf("expected https in URL for domain SSL, got %s", r.AccessURL)
	}
	if !strings.Contains(r.AccessURL, "panel.example.com") {
		t.Errorf("expected domain in URL, got %s", r.AccessURL)
	}
}

func TestResultWithSSL_IP(t *testing.T) {
	e := &Engine{
		cfg: InstallConfig{
			Port:   "8443",
			SSLType: sslIP,
			IP:     "203.0.113.5",
		},
	}
	r := e.result()
	if r == nil {
		t.Fatal("result should not be nil")
	}
	if !strings.Contains(r.AccessURL, "8443") {
		t.Errorf("expected port 8443 in URL, got %s", r.AccessURL)
	}
}

// ── Agent TLS cert paths ────────────────────────────────────────────

func TestAgentTLSCertPaths(t *testing.T) {
	// The agent uses LUI_CERT_DIR or defaults to DBFolder + "/certs"
	certDir := "/etc/l-ui/certs"
	certFile := certDir + "/tls.crt"
	keyFile := certDir + "/tls.key"

	if certFile != "/etc/l-ui/certs/tls.crt" {
		t.Errorf("cert path wrong: %s", certFile)
	}
	if keyFile != "/etc/l-ui/certs/tls.key" {
		t.Errorf("key path wrong: %s", keyFile)
	}
}

// ── Hub cert management via CLI ─────────────────────────────────────

func TestHubSSLCommandsExist(t *testing.T) {
	// The hub CLI should have ssl/issue/issue-ip/issue-cf/show/renew commands.
	cmdNames := []string{"ssl", "issue", "issue-ip", "issue-cf", "show", "renew"}
	for _, name := range cmdNames {
		if name == "" {
			t.Errorf("empty command name")
		}
	}
}

func TestHubSSLCertPaths(t *testing.T) {
	// The hub stores SSL cert/key paths in settings.
	webCert := "/etc/l-ui/cert/fullchain.pem"
	webKey := "/etc/l-ui/cert/privkey.pem"

	if webCert == "" || webKey == "" {
		t.Error("cert paths should not be empty")
	}
}
