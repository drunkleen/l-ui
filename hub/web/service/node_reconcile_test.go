package service

import (
	"encoding/json"
	"net"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	goruntime "runtime"
	"strconv"
	"sync/atomic"
	"testing"

	"github.com/drunkleen/l-ui/internal/database"
	"github.com/drunkleen/l-ui/internal/database/model"
	"github.com/drunkleen/l-ui/hub/web/runtime"
)

func TestReconcileRepairsWithRestartThenReinstall(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("LUI_NODE_BUNDLE_DIR", dir)
	t.Setenv("LUI_LOG_FOLDER", dir)
	t.Setenv("LUI_BIN_FOLDER", dir)
	archive := makeReleaseBundleArchive(t, goruntime.GOARCH)
	assetServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if _, err := w.Write(archive); err != nil {
			t.Fatalf("write archive: %v", err)
		}
	}))
	defer assetServer.Close()
	t.Setenv("LUI_NODE_BUNDLE_RELEASE_BASE", assetServer.URL)

	var statusCalls int32
	var restartCalls int32
	var reinstallCalls int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/v1/status":
			atomic.AddInt32(&statusCalls, 1)
			if atomic.LoadInt32(&reinstallCalls) == 0 {
				_ = json.NewEncoder(w).Encode(map[string]any{"success": false, "msg": "offline"})
				return
			}
			_ = json.NewEncoder(w).Encode(map[string]any{
				"success": true,
				"obj": map[string]any{
					"cpu":          1.5,
					"mem":          map[string]any{"current": 100, "total": 200},
					"xray":         map[string]any{"version": "v1.0.0"},
					"panelVersion": "v1.0.0",
					"uptime":       42,
				},
			})
		case "/api/v1/xray/restart":
			atomic.AddInt32(&restartCalls, 1)
			_ = json.NewEncoder(w).Encode(map[string]any{"success": true})
		case "/api/v1/server/reinstall":
			atomic.AddInt32(&reinstallCalls, 1)
			_ = json.NewEncoder(w).Encode(map[string]any{"success": true})
		default:
			t.Fatalf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
	}))
	defer server.Close()

	host, portStr, err := net.SplitHostPort(server.Listener.Addr().String())
	if err != nil {
		t.Fatalf("split host/port: %v", err)
	}
	port, err := strconv.Atoi(portStr)
	if err != nil {
		t.Fatalf("parse port: %v", err)
	}

	dbDir := t.TempDir()
	if err := database.InitDB(filepath.Join(dbDir, "l-ui.db")); err != nil {
		t.Fatalf("InitDB: %v", err)
	}
	defer func() { _ = database.CloseDB() }()

	oldMgr := runtime.GetManager()
	runtime.SetManager(runtime.NewManager(runtime.LocalDeps{}))
	defer runtime.SetManager(oldMgr)

	svc := &NodeService{}
	node := model.Node{
		Name:                "node-1",
		Address:             host,
		Scheme:              "http",
		Port:                port,
		BasePath:            "/",
		Enable:              true,
		AllowPrivateAddress: true,
		ApiToken:            "token-123",
	}
	if err := database.GetDB().Create(&node).Error; err != nil {
		t.Fatalf("create node: %v", err)
	}

	if err := svc.Reconcile(node.Id); err != nil {
		t.Fatalf("Reconcile returned error: %v", err)
	}
	if atomic.LoadInt32(&restartCalls) != 1 {
		t.Fatalf("restart calls = %d, want 1", restartCalls)
	}
	if atomic.LoadInt32(&reinstallCalls) != 1 {
		t.Fatalf("reinstall calls = %d, want 1", reinstallCalls)
	}
	if atomic.LoadInt32(&statusCalls) < 3 {
		t.Fatalf("status calls = %d, want at least 3", statusCalls)
	}
	if err := database.GetDB().First(&node, node.Id).Error; err != nil {
		t.Fatalf("reload node: %v", err)
	}
	if node.Status != "online" {
		t.Fatalf("node status = %q, want online", node.Status)
	}
}
