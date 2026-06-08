package service

import "testing"

func TestIsNewerVersion(t *testing.T) {
	cases := []struct {
		latest  string
		current string
		want    bool
	}{
		{"v2.9.4", "2.9.3", true},
		{"v2.10.0", "2.9.9", true},
		{"v2.9.3", "2.9.3", false},
		{"v2.9.2", "2.9.3", false},
		{"v3.0.0", "2.9.3", true},
	}

	for _, tc := range cases {
		if got := isNewerVersion(tc.latest, tc.current); got != tc.want {
			t.Fatalf("isNewerVersion(%q, %q) = %v, want %v", tc.latest, tc.current, got, tc.want)
		}
	}
}

func TestCompareVersionStringsRejectsUnexpectedFormats(t *testing.T) {
	if _, ok := compareVersionStrings("latest", "2.9.3"); ok {
		t.Fatal("expected non-semver latest tag to be rejected")
	}
	if _, ok := compareVersionStrings("v2.9", "2.9.3"); ok {
		t.Fatal("expected short version to be rejected")
	}
}

func TestExtractLatestReleaseTagFromHTML(t *testing.T) {
	html := `<a href="/drunkleen/l-ui/releases/tag/v0.0.1">v0.0.1</a>`
	if got := extractLatestReleaseTagFromHTML(html); got != "v0.0.1" {
		t.Fatalf("extractLatestReleaseTagFromHTML() = %q, want %q", got, "v0.0.1")
	}
}
