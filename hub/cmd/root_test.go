package cmd

import (
	"testing"
)

func TestNormalizeArgs(t *testing.T) {
	tests := []struct {
		name string
		args []string
		want []string
	}{
		{"no change for non-flags", []string{"l-ui", "run"}, []string{"l-ui", "run"}},
		{"convert -port", []string{"l-ui", "setting", "-port", "2053"}, []string{"l-ui", "setting", "--port", "2053"}},
		{"convert -port=value", []string{"l-ui", "setting", "-port=2053"}, []string{"l-ui", "setting", "--port=2053"}},
		{"preserve -v", []string{"l-ui", "-v"}, []string{"l-ui", "-v"}},
		{"preserve --port", []string{"l-ui", "setting", "--port", "2053"}, []string{"l-ui", "setting", "--port", "2053"}},
		{"convert multiple flags", []string{"l-ui", "setting", "-port", "2053", "-username", "admin"}, []string{"l-ui", "setting", "--port", "2053", "--username", "admin"}},
		{"convert -dsn", []string{"l-ui", "migrate-db", "-dsn", "postgres://..."}, []string{"l-ui", "migrate-db", "--dsn", "postgres://..."}},
		{"convert -json", []string{"l-ui", "doctor", "-json"}, []string{"l-ui", "doctor", "--json"}},
		{"preserve unknown single dash", []string{"l-ui", "setting", "-unknown"}, []string{"l-ui", "setting", "-unknown"}},
		{"preserve negative number", []string{"l-ui", "setting", "-port", "-1"}, []string{"l-ui", "setting", "--port", "-1"}},
		{"convert -webBasePath", []string{"l-ui", "setting", "-webBasePath", "/panel"}, []string{"l-ui", "setting", "--webBasePath", "/panel"}},
		{"convert -resetTwoFactor", []string{"l-ui", "setting", "-resetTwoFactor"}, []string{"l-ui", "setting", "--resetTwoFactor"}},
		{"convert -getApiToken", []string{"l-ui", "setting", "-getApiToken"}, []string{"l-ui", "setting", "--getApiToken"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := normalizeArgs(tt.args)
			if len(got) != len(tt.want) {
				t.Fatalf("normalizeArgs(%v) = %v, want %v", tt.args, got, tt.want)
			}
			for i := range got {
				if got[i] != tt.want[i] {
					t.Errorf("normalizeArgs(%v)[%d] = %q, want %q", tt.args, i, got[i], tt.want[i])
				}
			}
		})
	}
}

func TestVersionFlagParsing(t *testing.T) {
	showVersion = false
	err := rootCmd.ParseFlags([]string{"-v"})
	if err != nil {
		t.Fatalf("ParseFlags failed: %v", err)
	}
	if !showVersion {
		t.Error("expected showVersion to be true after parsing -v")
	}
}

func TestVersionLongFlagParsing(t *testing.T) {
	showVersion = false
	err := rootCmd.ParseFlags([]string{"--version"})
	if err != nil {
		t.Fatalf("ParseFlags failed: %v", err)
	}
	if !showVersion {
		t.Error("expected showVersion to be true after parsing --version")
	}
}

func TestRootCommandExists(t *testing.T) {
	if rootCmd == nil {
		t.Fatal("rootCmd is nil")
	}
	if rootCmd.Use == "" {
		t.Error("rootCmd.Use should not be empty")
	}
}
