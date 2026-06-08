package service

import (
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	goruntime "runtime"
	"strconv"
	"sync/atomic"
	"testing"

	"github.com/drunkleen/l-ui/hub/web/runtime"
	"github.com/drunkleen/l-ui/internal/bundle"
	"github.com/drunkleen/l-ui/internal/database"
	"github.com/drunkleen/l-ui/internal/database/model"
)

func TestReinstallAlwaysCallsRemoteBundleEndpoint(t *testing.T) {
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

	dbDir := t.TempDir()
	if err := database.InitDB(filepath.Join(dbDir, "l-ui.db")); err != nil {
		t.Fatalf("InitDB: %v", err)
	}
	defer func() { _ = database.CloseDB() }()

	bnd, err := bundle.BuildNodeBundle(goruntime.GOARCH)
	if err != nil {
		t.Fatalf("buildNodeBundle returned error: %v", err)
	}

	var called int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/api/v1/server/reinstall" {
			t.Fatalf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
		if ct := r.Header.Get("Content-Type"); ct == "" {
			t.Fatal("expected multipart content type")
		}
		if _, err := w.Write([]byte(`{"success":true}`)); err != nil {
			t.Fatalf("write response: %v", err)
		}
		atomic.AddInt32(&called, 1)
	}))
	defer server.Close()

	hostPort := server.Listener.Addr().String()
	host, portStr, err := net.SplitHostPort(hostPort)
	if err != nil {
		t.Fatalf("split host/port: %v", err)
	}
	port, err := strconv.Atoi(portStr)
	if err != nil {
		t.Fatalf("parse port: %v", err)
	}

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
		BundleSHA256:        bnd.SHA256,
	}
	if err := database.GetDB().Create(&node).Error; err != nil {
		t.Fatalf("create node: %v", err)
	}

	if err := svc.Reinstall(node.Id); err != nil {
		t.Fatalf("Reinstall returned error: %v", err)
	}
	if atomic.LoadInt32(&called) != 1 {
		t.Fatalf("remote reinstall endpoint called %d times, want 1", called)
	}
	if _, err := os.Stat(bnd.Path); err != nil {
		t.Fatalf("bundle missing after build: %v", err)
	}
}
