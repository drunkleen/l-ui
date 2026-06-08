package util

import (
	"testing"
)

func TestSanitizeHTTPURL_Valid(t *testing.T) {
	cases := []struct {
		input string
		want  string
	}{
		{"https://example.com", "https://example.com"},
		{"http://example.com/path", "http://example.com/path"},
		{"https://example.com:8443/path?q=1", "https://example.com:8443/path?q=1"},
		{"  https://example.com  ", "https://example.com"},
	}
	for _, c := range cases {
		got, err := SanitizeHTTPURL(c.input)
		if err != nil {
			t.Errorf("SanitizeHTTPURL(%q) error: %v", c.input, err)
			continue
		}
		if got != c.want {
			t.Errorf("SanitizeHTTPURL(%q) = %q, want %q", c.input, got, c.want)
		}
	}
}

func TestSanitizeHTTPURL_Empty(t *testing.T) {
	got, err := SanitizeHTTPURL("")
	if err != nil {
		t.Fatalf("SanitizeHTTPURL('') error: %v", err)
	}
	if got != "" {
		t.Errorf("got %q, want ''", got)
	}
}

func TestSanitizeHTTPURL_Invalid(t *testing.T) {
	cases := []string{
		"ftp://example.com",
		"not-a-url",
		"",
	}
	for _, input := range cases {
		got, err := SanitizeHTTPURL(input)
		if input != "" && err == nil {
			t.Errorf("SanitizeHTTPURL(%q) expected error, got %q", input, got)
		}
	}
}

func TestSanitizePublicHTTPURL_PrivateBlocked(t *testing.T) {
	_, err := SanitizePublicHTTPURL("http://127.0.0.1:2053", false)
	if err == nil {
		t.Error("expected error for loopback address without allowPrivate")
	}
}

func TestSanitizePublicHTTPURL_PrivateAllowed(t *testing.T) {
	got, err := SanitizePublicHTTPURL("http://127.0.0.1:2053", true)
	if err != nil {
		t.Fatalf("unexpected error when allowPrivate=true: %v", err)
	}
	if got != "http://127.0.0.1:2053" {
		t.Errorf("got %q, want 'http://127.0.0.1:2053'", got)
	}
}

func TestSanitizePublicHTTPURL_Public(t *testing.T) {
	got, err := SanitizePublicHTTPURL("https://example.com", false)
	if err != nil {
		t.Fatalf("unexpected error for public URL: %v", err)
	}
	if got != "https://example.com" {
		t.Errorf("got %q, want 'https://example.com'", got)
	}
}

func TestIsBlockedIP_Loopback(t *testing.T) {
	// isBlockedIP is unexported, tested indirectly via SanitizePublicHTTPURL
}
