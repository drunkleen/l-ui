package cmd

import (
	"strings"
	"testing"
)

func TestSettingFlagParsing(t *testing.T) {
	settingPort = 0
	settingUsername = ""
	settingReset = false

	err := settingCmd.ParseFlags([]string{"--port", "2053", "--username", "admin", "--reset"})
	if err != nil {
		t.Fatalf("ParseFlags failed: %v", err)
	}

	if settingPort != 2053 {
		t.Errorf("port = %d, want 2053", settingPort)
	}
	if settingUsername != "admin" {
		t.Errorf("username = %q, want admin", settingUsername)
	}
	if !settingReset {
		t.Error("expected reset to be true")
	}
}

func TestSettingSingleDashFlagParsing(t *testing.T) {
	settingPort = 0
	args := normalizeArgs([]string{"setting", "-port", "2053"})

	err := settingCmd.ParseFlags(args[1:])
	if err != nil {
		t.Fatalf("ParseFlags failed: %v", err)
	}

	if settingPort != 2053 {
		t.Errorf("port = %d, want 2053", settingPort)
	}
}

func TestSettingAllFlags(t *testing.T) {
	settingPort = 0
	settingUsername = ""
	settingPassword = ""
	settingWebBasePath = ""
	settingListenIP = ""
	settingResetTwoFactor = false
	settingGetListen = false
	settingGetCert = false
	settingGetApiToken = false
	settingWebCert = ""
	settingWebCertKey = ""
	settingTgbotToken = ""
	settingTgbotRuntime = ""
	settingTgbotChatid = ""
	settingEnableTgbot = false
	settingReset = false
	settingShow = false

	err := settingCmd.ParseFlags([]string{
		"--port", "2053",
		"--username", "admin",
		"--password", "secret",
		"--webBasePath", "/panel",
		"--listenIP", "0.0.0.0",
		"--resetTwoFactor",
		"--getListen",
		"--getCert",
		"--getApiToken",
		"--webCert", "/cert.pem",
		"--webCertKey", "/key.pem",
		"--tgbottoken", "bot123",
		"--tgbotRuntime", "0 0 * * *",
		"--tgbotchatid", "456",
		"--enabletgbot",
		"--reset",
		"--show",
	})
	if err != nil {
		t.Fatalf("ParseFlags failed: %v", err)
	}

	if settingPort != 2053 {
		t.Errorf("port = %d, want 2053", settingPort)
	}
	if settingUsername != "admin" {
		t.Errorf("username = %q, want admin", settingUsername)
	}
	if settingPassword != "secret" {
		t.Errorf("password = %q, want secret", settingPassword)
	}
	if settingWebBasePath != "/panel" {
		t.Errorf("webBasePath = %q, want /panel", settingWebBasePath)
	}
	if settingListenIP != "0.0.0.0" {
		t.Errorf("listenIP = %q, want 0.0.0.0", settingListenIP)
	}
	if !settingResetTwoFactor {
		t.Error("expected resetTwoFactor to be true")
	}
	if !settingGetListen {
		t.Error("expected getListen to be true")
	}
	if !settingGetCert {
		t.Error("expected getCert to be true")
	}
	if !settingGetApiToken {
		t.Error("expected getApiToken to be true")
	}
	if settingWebCert != "/cert.pem" {
		t.Errorf("webCert = %q, want /cert.pem", settingWebCert)
	}
	if settingWebCertKey != "/key.pem" {
		t.Errorf("webCertKey = %q, want /key.pem", settingWebCertKey)
	}
	if settingTgbotToken != "bot123" {
		t.Errorf("tgbotToken = %q, want bot123", settingTgbotToken)
	}
	if settingTgbotRuntime != "0 0 * * *" {
		t.Errorf("tgbotRuntime = %q, want 0 0 * * *", settingTgbotRuntime)
	}
	if settingTgbotChatid != "456" {
		t.Errorf("tgbotChatid = %q, want 456", settingTgbotChatid)
	}
	if !settingEnableTgbot {
		t.Error("expected enabletgbot to be true")
	}
	if !settingReset {
		t.Error("expected reset to be true")
	}
	if !settingShow {
		t.Error("expected show to be true")
	}
}

func TestSettingValidateFlagParsing(t *testing.T) {
	settingValidate = false

	err := settingCmd.ParseFlags([]string{"--validate"})
	if err != nil {
		t.Fatalf("ParseFlags failed: %v", err)
	}

	if !settingValidate {
		t.Error("expected validate to be true")
	}
}

func TestSettingsAlias(t *testing.T) {
	found := false
	for _, alias := range settingCmd.Aliases {
		if alias == "settings" {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected 'settings' alias to exist")
	}
}

func TestValidatePortAvailability(t *testing.T) {
	tests := []struct {
		name     string
		port     int
		wantPass bool
	}{
		{"invalid port 0", 0, false},
		{"invalid port -1", -1, false},
		{"invalid port 65536", 65536, false},
		{"invalid port 70000", 70000, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := validatePortAvailability(tt.port)
			isPass := result.Status == "PASS"
			if isPass != tt.wantPass {
				t.Errorf("validatePortAvailability(%d) = %s, want PASS=%v", tt.port, result.Status, tt.wantPass)
			}
		})
	}

	result := validatePortAvailability(2053)
	if result.Status != "PASS" {
		t.Logf("Note: Port 2053 may be in use or unavailable in this environment: %s", result.Message)
	}
}

func TestValidateListenIP(t *testing.T) {
	tests := []struct {
		name     string
		ip       string
		wantPass bool
	}{
		{"valid 0.0.0.0", "0.0.0.0", true},
		{"valid 127.0.0.1", "127.0.0.1", true},
		{"valid ::", "::", true},
		{"empty string", "", false},
		{"invalid IP", "invalid", false},
		{"invalid format", "192.168.1", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := validateListenIP(tt.ip)
			isPass := result.Status == "PASS"
			if isPass != tt.wantPass {
				t.Errorf("validateListenIP(%q) = %s, want PASS=%v", tt.ip, result.Status, tt.wantPass)
			}
		})
	}
}

func TestValidateWebBasePath(t *testing.T) {
	tests := []struct {
		name     string
		path     string
		wantPass bool
	}{
		{"valid /panel", "/panel", true},
		{"valid /", "/", true},
		{"valid /abc", "/abc", true},
		{"empty string", "", false},
		{"no leading slash", "panel", false},
		{"too short", "/ab", false},
		{"contains space", "/panel test", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := validateWebBasePath(tt.path)
			isPass := result.Status == "PASS"
			if isPass != tt.wantPass {
				t.Errorf("validateWebBasePath(%q) = %s, want PASS=%v", tt.path, result.Status, tt.wantPass)
			}
		})
	}
}

func TestValidateUsername(t *testing.T) {
	tests := []struct {
		name     string
		username string
		wantPass bool
	}{
		{"valid admin", "admin", true},
		{"valid user123", "user123", true},
		{"empty string", "", false},
		{"too short", "ab", false},
		{"contains space", "admin user", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := validateUsername(tt.username)
			isPass := result.Status == "PASS"
			if isPass != tt.wantPass {
				t.Errorf("validateUsername(%q) = %s, want PASS=%v", tt.username, result.Status, tt.wantPass)
			}
		})
	}
}

func TestValidatePassword(t *testing.T) {
	tests := []struct {
		name       string
		password   string
		wantPass   bool
		wantStatus string
	}{
		{"valid password123", "password123", true, "PASS"},
		{"valid 6chars", "123456", true, "PASS"},
		{"empty string", "", false, "WARN"},
		{"too short", "12345", false, "FAIL"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := validatePassword(tt.password)
			isPass := result.Status == "PASS"
			if isPass != tt.wantPass {
				t.Errorf("validatePassword(%q) = %s, want PASS=%v", tt.password, result.Status, tt.wantPass)
			}
			if result.Status != tt.wantStatus {
				t.Errorf("validatePassword(%q) Status = %s, want %s", tt.password, result.Status, tt.wantStatus)
			}
		})
	}
}

func TestFormatValidationStatus(t *testing.T) {
	tests := []struct {
		name     string
		input    settingValidation
		contains string
	}{
		{"PASS contains green", settingValidation{Status: "PASS", Message: "OK"}, "✓"},
		{"FAIL contains red marker", settingValidation{Status: "FAIL", Message: "Error"}, "✖"},
		{"WARN contains yellow marker", settingValidation{Status: "WARN", Message: "Warning"}, "⚠"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := formatValidationStatus(tt.input)
			if !strings.Contains(result, tt.contains) {
				t.Errorf("formatValidationStatus(%+v) = %q, want contains %q", tt.input, result, tt.contains)
			}
		})
	}
}

func TestGetLocalIPs(t *testing.T) {
	ips := getLocalIPs()
	if ips == "" {
		t.Log("No local IPs found (could be normal in some environments)")
	}
}

func TestDetectExternalIPFallback(t *testing.T) {
	ip := detectExternalIP()
	if ip == "" {
		t.Log("detectExternalIP returned empty (expected if no network or all services fail)")
	}
}
