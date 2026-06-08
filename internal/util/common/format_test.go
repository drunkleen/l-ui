package common

import "testing"

func TestNormalizeBasePath(t *testing.T) {
	cases := []struct {
		in   string
		want string
	}{
		{"", "/"},
		{"   ", "/"},
		{"/", "/"},
		{"/panel", "/panel/"},
		{"panel", "/panel/"},
		{"panel/", "/panel/"},
		{"/panel/", "/panel/"},
		{"  /panel  ", "/panel/"},
		{"/a/b/c", "/a/b/c/"},
	}
	for _, c := range cases {
		t.Run(c.in, func(t *testing.T) {
			got := NormalizeBasePath(c.in)
			if got != c.want {
				t.Fatalf("NormalizeBasePath(%q) = %q, want %q", c.in, got, c.want)
			}
		})
	}
}

func TestShellQuote(t *testing.T) {
	if got := ShellQuote("/usr/bin/curl"); got != "'/usr/bin/curl'" {
		t.Fatalf("ShellQuote simple path = %q, want %q", got, "'/usr/bin/curl'")
	}
	if got := ShellQuote("/tmp/a'b"); got != "'/tmp/a'\\''b'" {
		t.Fatalf("ShellQuote with embedded quote = %q, want %q", got, "'/tmp/a'\\''b'")
	}
}

func TestFormatTraffic(t *testing.T) {
	cases := []struct {
		name  string
		bytes int64
		want  string
	}{
		{"zero", 0, "0.00B"},
		{"under_one_kb", 512, "512.00B"},
		{"exactly_one_kb", 1024, "1.00KB"},
		{"one_and_a_half_kb", 1536, "1.50KB"},
		{"one_mb", 1024 * 1024, "1.00MB"},
		{"one_gb", 1024 * 1024 * 1024, "1.00GB"},
		{"one_tb", 1024 * 1024 * 1024 * 1024, "1.00TB"},
		{"one_pb", 1024 * 1024 * 1024 * 1024 * 1024, "1.00PB"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := FormatTraffic(c.bytes)
			if got != c.want {
				t.Fatalf("FormatTraffic(%d) = %q, want %q", c.bytes, got, c.want)
			}
		})
	}
}
