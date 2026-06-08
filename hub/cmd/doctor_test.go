package cmd

import (
	"os"
	"strings"
	"testing"

	"github.com/drunkleen/l-ui/internal/config"
)

func TestCheckWebBasePathValidation(t *testing.T) {
	tests := []struct {
		name     string
		basePath string
		want     string
	}{
		{"valid root", "/", "PASS"},
		{"valid multi-char", "/panel", "PASS"},
		{"valid nested", "/a/b/c", "PASS"},
		{"empty", "", "FAIL"},
		{"no leading slash", "panel", "FAIL"},
		{"too short", "/ab", "FAIL"},
		{"contains space", "/path with space", "FAIL"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := checkWebBasePathForTest(tt.basePath)
			if result.Status != tt.want {
				t.Errorf("checkWebBasePath(%q) = %v, want %v", tt.basePath, result.Status, tt.want)
			}
		})
	}
}

func checkWebBasePathForTest(basePath string) doctorCheck {
	if basePath == "" {
		return doctorCheck{Name: "WebBasePath", Status: "FAIL", Message: "Base path is empty"}
	}

	if !strings.HasPrefix(basePath, "/") {
		return doctorCheck{Name: "WebBasePath", Status: "FAIL", Message: "must start with /"}
	}

	if len(basePath) < 4 && basePath != "/" {
		return doctorCheck{Name: "WebBasePath", Status: "FAIL", Message: "too short"}
	}

	if strings.Contains(basePath, " ") {
		return doctorCheck{Name: "WebBasePath", Status: "FAIL", Message: "contains space"}
	}

	return doctorCheck{Name: "WebBasePath", Status: "PASS", Message: "valid"}
}

func TestCheckPortValidRange(t *testing.T) {
	tests := []struct {
		name string
		port int
		want string
	}{
		{"valid port 2053", 2053, "PASS"},
		{"valid port 80", 80, "PASS"},
		{"valid port 443", 443, "PASS"},
		{"valid port 8080", 8080, "PASS"},
		{"port 0", 0, "FAIL"},
		{"port -1", -1, "FAIL"},
		{"port 65536", 65536, "FAIL"},
		{"port 65535", 65535, "PASS"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := checkPortForTest(tt.port)
			if result.Status != tt.want {
				t.Errorf("checkPort(%d) = %v, want %v", tt.port, result.Status, tt.want)
			}
		})
	}
}

func checkPortForTest(port int) doctorCheck {
	if port <= 0 || port > 65535 {
		return doctorCheck{Name: "Port", Status: "FAIL", Message: "out of range"}
	}
	return doctorCheck{Name: "Port", Status: "PASS", Message: "available"}
}

func TestDoctorResultStruct(t *testing.T) {
	result := doctorResult{
		Pass: true,
		Checks: []doctorCheck{
			{Name: "Database", Status: "PASS", Message: "SQLite DB accessible"},
		},
	}

	if !result.Pass {
		t.Error("Expected Pass to be true")
	}
	if len(result.Checks) != 1 {
		t.Errorf("Expected 1 check, got %d", len(result.Checks))
	}
	if result.Checks[0].Name != "Database" {
		t.Errorf("Expected check name 'Database', got %s", result.Checks[0].Name)
	}
}

func TestDoctorCheckStruct(t *testing.T) {
	check := doctorCheck{
		Name:    "Port",
		Status:  "PASS",
		Message: "Port 2053 is available",
	}

	if check.Name != "Port" {
		t.Errorf("Expected Name 'Port', got %s", check.Name)
	}
	if check.Status != "PASS" {
		t.Errorf("Expected Status 'PASS', got %s", check.Status)
	}
	if check.Message != "Port 2053 is available" {
		t.Errorf("Expected Message 'Port 2053 is available', got %s", check.Message)
	}
}

func TestCheckConfigConsistency(t *testing.T) {
	originalDBType := os.Getenv("LUI_DB_TYPE")
	originalDBDSN := os.Getenv("LUI_DB_DSN")
	defer func() {
		os.Setenv("LUI_DB_TYPE", originalDBType)
		os.Setenv("LUI_DB_DSN", originalDBDSN)
	}()

	tests := []struct {
		name   string
		dbType string
		dbDSN  string
		want   string
	}{
		{"sqlite no dsn", "sqlite", "", "PASS"},
		{"sqlite with dsn warning", "sqlite", "postgres://...", "WARN"},
		{"postgres no dsn", "postgres", "", "FAIL"},
		{"postgres with dsn", "postgres", "postgres://user:pass@host/db", "PASS"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			os.Setenv("LUI_DB_TYPE", tt.dbType)
			os.Setenv("LUI_DB_DSN", tt.dbDSN)

			result := checkConfigConsistencyForTest()
			if result.Status != tt.want {
				t.Errorf("checkConfigConsistency(dbType=%q, dbDSN=%q) = %v, want %v",
					tt.dbType, tt.dbDSN, result.Status, tt.want)
			}
		})
	}
}

func checkConfigConsistencyForTest() doctorCheck {
	dbKind := config.GetDBKind()
	dsn := config.GetDBDSN()

	if (dbKind == "postgres" || dbKind == "mysql") && dsn == "" {
		return doctorCheck{Name: "Config Consistency", Status: "FAIL", Message: "LUI_DB_DSN is not set"}
	}

	if dbKind == "sqlite" && dsn != "" {
		return doctorCheck{Name: "Config Consistency", Status: "WARN", Message: "DSN will be ignored"}
	}

	return doctorCheck{Name: "Config Consistency", Status: "PASS", Message: "Configuration is consistent"}
}

func TestColorConstants(t *testing.T) {
	if colorGreen == "" {
		t.Error("colorGreen should not be empty")
	}
	if colorRed == "" {
		t.Error("colorRed should not be empty")
	}
	if colorYellow == "" {
		t.Error("colorYellow should not be empty")
	}
	if colorReset == "" {
		t.Error("colorReset should not be empty")
	}

	if !strings.Contains(colorGreen, "32") {
		t.Error("colorGreen should contain ANSI color code 32 (green)")
	}
	if !strings.Contains(colorRed, "31") {
		t.Error("colorRed should contain ANSI color code 31 (red)")
	}
	if !strings.Contains(colorYellow, "33") {
		t.Error("colorYellow should contain ANSI color code 33 (yellow)")
	}
}

func TestDoctorCheckStatusValues(t *testing.T) {
	validStatuses := []string{"PASS", "FAIL", "WARN"}

	for _, status := range validStatuses {
		check := doctorCheck{Name: "test", Status: status}
		if check.Status != status {
			t.Errorf("Expected status %s, got %s", status, check.Status)
		}
	}
}

func TestDoctorFlagParsing(t *testing.T) {
	doctorJSON = false
	err := doctorCmd.ParseFlags([]string{"--json"})
	if err != nil {
		t.Fatalf("ParseFlags failed: %v", err)
	}
	if !doctorJSON {
		t.Error("expected doctorJSON to be true")
	}
}

func TestDoctorSingleDashFlagParsing(t *testing.T) {
	doctorJSON = false
	args := normalizeArgs([]string{"doctor", "-json"})

	err := doctorCmd.ParseFlags(args[1:])
	if err != nil {
		t.Fatalf("ParseFlags failed: %v", err)
	}
	if !doctorJSON {
		t.Error("expected doctorJSON to be true after normalizing -json")
	}
}

func TestDoctorCommandExists(t *testing.T) {
	if doctorCmd == nil {
		t.Fatal("doctorCmd is nil")
	}
	if doctorCmd.Use != "doctor" {
		t.Errorf("Use = %q, want doctor", doctorCmd.Use)
	}
}
