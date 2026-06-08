package runtime

import (
	"encoding/json"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"testing"

	"github.com/drunkleen/l-ui/internal/database/model"
)

// cacheGetTag must resolve a remote inbound id even when the n<id>- prefix
// sits on only one side: the node may store the bare tag while the central
// panel pushes the prefixed form, or vice versa. Without this a mismatch makes
// the push create a duplicate inbound on the node.
func TestCacheGetTag_PrefixAgnostic(t *testing.T) {
	cases := []struct {
		name      string
		cacheTag  string
		lookup    string
		wantID    int
		wantFound bool
	}{
		{"exact", "n1-in-443-tcp", "n1-in-443-tcp", 7, true},
		{"node bare, lookup prefixed", "in-443-tcp", "n1-in-443-tcp", 7, true},
		{"node prefixed, lookup bare", "n1-in-443-tcp", "in-443-tcp", 7, true},
		{"unrelated tag", "in-443-tcp", "in-999-tcp", 0, false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			r := NewRemote(&model.Node{Id: 1, Name: "n1"})
			r.cacheSet(c.cacheTag, 7)
			id, ok := r.cacheGetTag(c.lookup)
			if ok != c.wantFound || id != c.wantID {
				t.Fatalf("cacheGetTag(%q) = (%d, %v), want (%d, %v)", c.lookup, id, ok, c.wantID, c.wantFound)
			}
		})
	}
}

func TestSanitizeStreamSettingsForRemote(t *testing.T) {
	tests := []struct {
		name  string
		input string
		// wantCertFile / wantKeyFile: expected presence after sanitize
		wantCertFile bool
		wantKeyFile  bool
	}{
		{
			name: "file paths only — kept intact (remote node paths)",
			input: `{
				"tlsSettings": {
					"certificates": [{
						"certificateFile": "/etc/ssl/cert.crt",
						"keyFile": "/etc/ssl/key.key"
					}]
				}
			}`,
			wantCertFile: true,
			wantKeyFile:  true,
		},
		{
			name: "inline content only — unchanged",
			input: `{
				"tlsSettings": {
					"certificates": [{
						"certificate": ["-----BEGIN CERTIFICATE-----"],
						"key": ["-----BEGIN PRIVATE KEY-----"]
					}]
				}
			}`,
			wantCertFile: false,
			wantKeyFile:  false,
		},
		{
			name: "both file paths and inline content — file paths stripped (redundant)",
			input: `{
				"tlsSettings": {
					"certificates": [{
						"certificateFile": "/etc/ssl/cert.crt",
						"keyFile": "/etc/ssl/key.key",
						"certificate": ["-----BEGIN CERTIFICATE-----"],
						"key": ["-----BEGIN PRIVATE KEY-----"]
					}]
				}
			}`,
			wantCertFile: false,
			wantKeyFile:  false,
		},
		{
			name:  "empty stream settings",
			input: "",
			// empty input returns empty, nothing to check
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if tc.input == "" {
				if got := sanitizeStreamSettingsForRemote(tc.input); got != "" {
					t.Errorf("expected empty string, got %q", got)
				}
				return
			}
			got := sanitizeStreamSettingsForRemote(tc.input)
			var out map[string]any
			if err := json.Unmarshal([]byte(got), &out); err != nil {
				t.Fatalf("output is not valid JSON: %v\noutput: %s", err, got)
			}

			tls, _ := out["tlsSettings"].(map[string]any)
			certs, _ := tls["certificates"].([]any)
			if len(certs) == 0 {
				t.Fatal("certificates array missing in output")
			}
			cert, _ := certs[0].(map[string]any)

			_, hasCertFile := cert["certificateFile"]
			_, hasKeyFile := cert["keyFile"]

			if hasCertFile != tc.wantCertFile {
				t.Errorf("certificateFile present=%v, want %v", hasCertFile, tc.wantCertFile)
			}
			if hasKeyFile != tc.wantKeyFile {
				t.Errorf("keyFile present=%v, want %v", hasKeyFile, tc.wantKeyFile)
			}
		})
	}
}

func TestRemoteUfwOperations(t *testing.T) {
	var calls []string
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls = append(calls, r.Method+" "+r.URL.Path)
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/api/v1/firewall/status":
			_, _ = w.Write([]byte(`{"success":true,"obj":{"active":true,"rules":[{"port":"2053/tcp","protocol":"tcp","action":"allow","comment":""}]}}`))
		case r.Method == http.MethodPost && r.URL.Path == "/api/v1/firewall/rules":
			_, _ = w.Write([]byte(`{"success":true,"msg":"rule added"}`))
		default:
			t.Fatalf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
	}))
	defer ts.Close()

	u, err := url.Parse(ts.URL)
	if err != nil {
		t.Fatalf("parse url: %v", err)
	}
	r := NewRemote(&model.Node{Id: 1, Name: "node-1", Address: u.Hostname(), Port: mustPortURL(t, u), Scheme: "http", BasePath: "/", ApiToken: "token-1"})
	oldClient := remoteHTTPClient
	defer func() { remoteHTTPClient = oldClient }()
	remoteHTTPClient = ts.Client()

	ctx := t.Context()
	status, err := r.ListUfwRules(ctx)
	if err != nil {
		t.Fatalf("ListUfwRules: %v", err)
	}
	if !status.Active || len(status.Rules) != 1 || status.Rules[0].Port != "2053/tcp" {
		t.Fatalf("unexpected status: %#v", status)
	}
	if err := r.AllowUfwPort(ctx, 2443, "tcp"); err != nil {
		t.Fatalf("AllowUfwPort: %v", err)
	}
	if err := r.DenyUfwPort(ctx, 2443, "udp"); err != nil {
		t.Fatalf("DenyUfwPort: %v", err)
	}
	want := []string{
		"GET /api/v1/firewall/status",
		"POST /api/v1/firewall/rules",
		"POST /api/v1/firewall/rules",
	}
	if len(calls) != len(want) {
		t.Fatalf("calls = %#v, want %#v", calls, want)
	}
	for i := range want {
		if calls[i] != want[i] {
			t.Fatalf("calls[%d] = %q, want %q", i, calls[i], want[i])
		}
	}
}

func mustPortURL(t *testing.T, u *url.URL) int {
	t.Helper()
	_, portStr, err := net.SplitHostPort(u.Host)
	if err != nil {
		t.Fatalf("split host port: %v", err)
	}
	port, err := strconv.Atoi(portStr)
	if err != nil {
		t.Fatalf("parse port: %v", err)
	}
	return port
}
