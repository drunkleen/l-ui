package arch

import (
	"runtime"
	"testing"
)

// ── SSHBootstrapArch ───────────────────────────────────────────────

func TestSSHBootstrapArch_All(t *testing.T) {
	cases := map[string]string{
		"x86_64":   "amd64",
		"amd64":    "amd64",
		"aarch64":  "arm64",
		"arm64":    "arm64",
		"armv7l":   "armv7",
		"armv7":    "armv7",
		"armv6l":   "armv6",
		"armv6":    "armv6",
		"armv5tel": "armv5",
		"armv5":    "armv5",
		"i386":     "386",
		"i686":     "386",
		"386":      "386",
		"s390x":    "s390x",
	}
	for input, want := range cases {
		got, err := SSHBootstrapArch(input)
		if err != nil {
			t.Errorf("SSHBootstrapArch(%q) error: %v", input, err)
		}
		if got != want {
			t.Errorf("SSHBootstrapArch(%q) = %q, want %q", input, got, want)
		}
	}
}

func TestSSHBootstrapArch_Whitespace(t *testing.T) {
	got, err := SSHBootstrapArch("  x86_64  ")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != "amd64" {
		t.Errorf("got %q, want 'amd64'", got)
	}
}

func TestSSHBootstrapArch_Empty(t *testing.T) {
	_, err := SSHBootstrapArch("")
	if err == nil {
		t.Fatal("expected error for empty input")
	}
}

func TestSSHBootstrapArch_Unknown(t *testing.T) {
	_, err := SSHBootstrapArch("riscv64")
	if err == nil {
		t.Fatal("expected error for unknown arch")
	}
}

// ── SupportedTargetArchs completeness ──────────────────────────────

func TestSupportedTargetArchs_IncludesAllSSHArches(t *testing.T) {
	// Every arch that SSHBootstrapArch can return should be in SupportedTargetArchs.
	sshArches := map[string]bool{
		"amd64": true,
		"arm64": true,
		"armv7": true,
		"armv6": true,
		"armv5": true,
		"386":   true,
		"s390x": true,
	}
	for arch := range sshArches {
		if _, ok := SupportedTargetArchs[arch]; !ok {
			t.Errorf("SSHBootstrapArch can return %q but it's not in SupportedTargetArchs", arch)
		}
	}
	for arch := range SupportedTargetArchs {
		if !sshArches[arch] {
			t.Errorf("SupportedTargetArchs has %q but SSHBootstrapArch can't return it", arch)
		}
	}
	if len(SupportedTargetArchs) != len(sshArches) {
		t.Errorf("arch count mismatch: SupportedTargetArchs=%d, sshArches=%d",
			len(SupportedTargetArchs), len(sshArches))
	}
}

// ── CaddyAssetArch ─────────────────────────────────────────────────

func TestCaddyAssetArch_All(t *testing.T) {
	cases := map[string]string{
		"amd64": "amd64",
		"arm64": "arm64",
		"armv7": "armv7",
		"armv6": "armv6",
		"386":   "386",
		"s390x": "s390x",
	}
	for input, want := range cases {
		got, err := CaddyAssetArch(input)
		if err != nil {
			t.Errorf("CaddyAssetArch(%q) error: %v", input, err)
		}
		if got != want {
			t.Errorf("CaddyAssetArch(%q) = %q, want %q", input, got, want)
		}
	}
}

func TestCaddyAssetArch_Unsupported(t *testing.T) {
	// Caddy doesn't support armv5 — should error.
	_, err := CaddyAssetArch("armv5")
	if err == nil {
		t.Error("expected error for armv5 (Caddy doesn't support it)")
	}
}

func TestCaddyAssetArch_Empty(t *testing.T) {
	_, err := CaddyAssetArch("")
	if err == nil {
		t.Fatal("expected error for empty input")
	}
}

func TestCaddyAssetArch_Whitespace(t *testing.T) {
	got, err := CaddyAssetArch("  amd64  ")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != "amd64" {
		t.Errorf("got %q, want 'amd64'", got)
	}
}

// ── NormalizeBundleArch ────────────────────────────────────────────

func TestNormalizeBundleArch(t *testing.T) {
	cases := []struct {
		input string
		want  string
	}{
		{"amd64", "amd64"},
		{"  arm64  ", "arm64"},
		{"", runtime.GOARCH},
	}
	for _, c := range cases {
		got := NormalizeBundleArch(c.input)
		if got != c.want {
			t.Errorf("NormalizeBundleArch(%q) = %q, want %q", c.input, got, c.want)
		}
	}
}

// ── Cross-function consistency ─────────────────────────────────────

func TestArchConsistency(t *testing.T) {
	// Every arch that CaddyAssetArch supports should also be in SupportedTargetArchs.
	caddyArches := []string{"amd64", "arm64", "armv7", "armv6", "386", "s390x"}
	for _, a := range caddyArches {
		if _, ok := SupportedTargetArchs[a]; !ok {
			t.Errorf("Caddy supports %q but it's not in SupportedTargetArchs", a)
		}
	}
}
