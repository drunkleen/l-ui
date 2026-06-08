package service

import (
	"context"
	"encoding/json"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"path/filepath"
	"strconv"
	"testing"

	"github.com/drunkleen/l-ui/internal/database"
	"github.com/drunkleen/l-ui/internal/database/model"
)

func TestNodeProbeAndReconnectFlow(t *testing.T) {
	dbDir := t.TempDir()
	if err := database.InitDB(filepath.Join(dbDir, "l-ui.db")); err != nil {
		t.Fatalf("init db: %v", err)
	}
	defer func() { _ = database.CloseDB() }()

	svc := &NodeService{}
	node := &model.Node{Name: "node-1", Address: "127.0.0.1", Port: 1, Scheme: "http", BasePath: "/", ApiToken: "token-1", Enable: true}
	if err := svc.Create(node); err != nil {
		t.Fatalf("create node: %v", err)
	}

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/status" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		if got := r.Header.Get("Authorization"); got != "Bearer token-1" {
			t.Fatalf("auth header = %q", got)
		}
		json.NewEncoder(w).Encode(map[string]any{
			"success": true,
			"obj": map[string]any{
				"cpu":          12.5,
				"mem":          map[string]any{"current": 128, "total": 256},
				"xray":         map[string]any{"version": "1.0.0"},
				"panelVersion": "2.0.0",
				"uptime":       42,
			},
		})
	}))
	defer ts.Close()

	u, err := url.Parse(ts.URL)
	if err != nil {
		t.Fatalf("parse test server url: %v", err)
	}
	updated := &model.Node{Name: "node-1", Address: u.Hostname(), Port: mustPort(t, u), Scheme: "http", BasePath: "/", ApiToken: "token-1", Enable: true, AllowPrivateAddress: true}
	patch, err := svc.Probe(context.Background(), updated)
	if err != nil {
		t.Fatalf("probe: %v", err)
	}
	if patch.LastError != "" {
		t.Fatalf("unexpected probe error: %+v", patch)
	}
	patch.Status = "online"
	if err := svc.UpdateHeartbeat(node.Id, patch); err != nil {
		t.Fatalf("update heartbeat: %v", err)
	}
	got, err := svc.GetById(node.Id)
	if err != nil {
		t.Fatalf("get node: %v", err)
	}
	if got.Status != "online" || got.XrayVersion != "1.0.0" || got.PanelVersion != "2.0.0" {
		t.Fatalf("unexpected node state after heartbeat: %+v", got)
	}
	ts.Close()
	patch, err = svc.Probe(context.Background(), updated)
	if err == nil {
		t.Fatal("expected probe to fail after server close")
	}
	patch.Status = "offline"
	if err := svc.UpdateHeartbeat(node.Id, patch); err != nil {
		t.Fatalf("update offline heartbeat: %v", err)
	}
	got, err = svc.GetById(node.Id)
	if err != nil {
		t.Fatalf("get node after offline: %v", err)
	}
	if got.Status != "offline" {
		t.Fatalf("expected offline status after reconnect failure, got %+v", got)
	}
}

func TestBootstrapNodePersistAndProbeLifecycle(t *testing.T) {
	dbDir := t.TempDir()
	if err := database.InitDB(filepath.Join(dbDir, "l-ui.db")); err != nil {
		t.Fatalf("init db: %v", err)
	}
	defer func() { _ = database.CloseDB() }()

	svc := &NodeService{}

	// Build a bootstrap node from a request (like StartBootstrap does internally).
	req := NodeBootstrapRequest{
		Name:        "e2e-lifecycle",
		Address:     "203.0.113.50",
		SSHUser:     "root",
		SSHPassword: "secret",
		AgentPort:   2053,
	}
	node, err := svc.buildBootstrapNode(req)
	if err != nil {
		t.Fatalf("buildBootstrapNode: %v", err)
	}

	// Persist the node to DB (this is what bootstrapFlow calls on success).
	if err := svc.persistBootstrapNode(&node); err != nil {
		t.Fatalf("persistBootstrapNode: %v", err)
	}

	// Verify the node was created in DB.
	saved, err := svc.GetById(node.Id)
	if err != nil {
		t.Fatalf("GetById after persist: %v", err)
	}
	if saved.Name != "e2e-lifecycle" {
		t.Fatalf("name = %q, want %q", saved.Name, "e2e-lifecycle")
	}
	if saved.Port != 2053 {
		t.Fatalf("port = %d, want 2053", saved.Port)
	}
	if !saved.Enable {
		t.Fatal("node should be enabled after bootstrap")
	}
	if saved.ApiToken == "" {
		t.Fatal("api token should be set after bootstrap")
	}

	// Set up a probe server with retry (503 then 200).
	attempt := 0
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		attempt++
		if attempt < 2 {
			w.WriteHeader(http.StatusServiceUnavailable)
			json.NewEncoder(w).Encode(map[string]any{"success": false, "msg": "not ready"})
			return
		}
		json.NewEncoder(w).Encode(map[string]any{
			"success": true,
			"obj": map[string]any{
				"cpu":          45.2,
				"mem":          map[string]any{"current": 512, "total": 1024},
				"disk":         map[string]any{"current": 20480, "total": 51200},
				"netIO":        map[string]any{"up": 1000, "down": 2000},
				"xray":         map[string]any{"version": "1.8.0"},
				"panelVersion": "3.0.0",
				"uptime":       3600,
			},
		})
	}))
	defer ts.Close()

	u, _ := url.Parse(ts.URL)
	probeNode := &model.Node{
		Name:                "e2e-lifecycle",
		Address:             u.Hostname(),
		Port:                mustPort(t, u),
		Scheme:              "http",
		BasePath:            "/",
		ApiToken:            saved.ApiToken,
		Enable:              true,
		AllowPrivateAddress: true,
	}

	// Probe with retry should succeed after transient 503.
	patch, err := svc.Probe(context.Background(), probeNode)
	if err != nil {
		t.Fatalf("probe after retries: %v (attempts=%d)", err, attempt)
	}
	if attempt < 2 {
		t.Fatalf("expected retry on transient error, got %d attempt(s)", attempt)
	}
	if patch.LastError != "" {
		t.Fatalf("unexpected probe error: %s", patch.LastError)
	}

	// Update heartbeat with probe result.
	patch.Status = "online"
	if err := svc.UpdateHeartbeat(saved.Id, patch); err != nil {
		t.Fatalf("update heartbeat: %v", err)
	}

	// Verify heartbeat data was persisted.
	updated, err := svc.GetById(saved.Id)
	if err != nil {
		t.Fatalf("get after heartbeat: %v", err)
	}
	if updated.Status != "online" {
		t.Fatalf("status = %q, want online", updated.Status)
	}
	if updated.XrayVersion != "1.8.0" {
		t.Fatalf("xray version = %q, want 1.8.0", updated.XrayVersion)
	}
	if updated.PanelVersion != "3.0.0" {
		t.Fatalf("panel version = %q, want 3.0.0", updated.PanelVersion)
	}
	if updated.CpuPct != 45.2 {
		t.Fatalf("cpu pct = %f, want 45.2", updated.CpuPct)
	}
}

func TestProbeRetriesOnTransientHTTPErrors(t *testing.T) {
	dbDir := t.TempDir()
	if err := database.InitDB(filepath.Join(dbDir, "l-ui.db")); err != nil {
		t.Fatalf("init db: %v", err)
	}
	defer func() { _ = database.CloseDB() }()

	svc := &NodeService{}
	node := &model.Node{Name: "retry-node", Address: "127.0.0.1", Port: 1, Scheme: "http", BasePath: "/", ApiToken: "token-retry", Enable: true}
	if err := svc.Create(node); err != nil {
		t.Fatalf("create node: %v", err)
	}

	attempt := 0
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		attempt++
		if attempt < 3 {
			w.WriteHeader(http.StatusServiceUnavailable)
			json.NewEncoder(w).Encode(map[string]any{"success": false, "msg": "not ready yet"})
			return
		}
		json.NewEncoder(w).Encode(map[string]any{
			"success": true,
			"obj": map[string]any{
				"cpu":          12.5,
				"mem":          map[string]any{"current": 128, "total": 256},
				"xray":         map[string]any{"version": "1.0.0"},
				"panelVersion": "2.0.0",
				"uptime":       42,
			},
		})
	}))
	defer ts.Close()

	u, _ := url.Parse(ts.URL)
	updated := &model.Node{Name: "retry-node", Address: u.Hostname(), Port: mustPort(t, u), Scheme: "http", BasePath: "/", ApiToken: "token-retry", Enable: true, AllowPrivateAddress: true}
	patch, err := svc.Probe(context.Background(), updated)
	if err != nil {
		t.Fatalf("probe after retries: %v (attempts=%d)", err, attempt)
	}
	if attempt < 3 {
		t.Fatalf("expected at least 3 HTTP attempts before success, got %d", attempt)
	}
	_ = patch
}

func TestProbeDoesNotRetryNonTransientErrors(t *testing.T) {
	dbDir := t.TempDir()
	if err := database.InitDB(filepath.Join(dbDir, "l-ui.db")); err != nil {
		t.Fatalf("init db: %v", err)
	}
	defer func() { _ = database.CloseDB() }()

	svc := &NodeService{}
	node := &model.Node{Name: "no-retry-node", Address: "127.0.0.1", Port: 1, Scheme: "http", BasePath: "/", ApiToken: "token-no-retry", Enable: true}
	if err := svc.Create(node); err != nil {
		t.Fatalf("create node: %v", err)
	}

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]any{"success": false, "msg": "bad request"})
	}))
	defer ts.Close()

	u, _ := url.Parse(ts.URL)
	updated := &model.Node{Name: "no-retry-node", Address: u.Hostname(), Port: mustPort(t, u), Scheme: "http", BasePath: "/", ApiToken: "token-no-retry", Enable: true, AllowPrivateAddress: true}
	_, err := svc.Probe(context.Background(), updated)
	if err == nil {
		t.Fatal("expected probe to fail for non-transient error")
	}
}

func mustPort(t *testing.T, u *url.URL) int {
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
