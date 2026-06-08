package service

import (
	"encoding/json"
	"path/filepath"
	"testing"

	agentdb "github.com/drunkleen/l-ui/agent/database"
)

func setupConfigTestDB(t *testing.T) {
	t.Helper()
	dbDir := t.TempDir()
	dbPath := filepath.Join(dbDir, "l-ui-agent.db")
	if err := agentdb.InitDB(dbPath); err != nil {
		t.Fatalf("InitDB failed: %v", err)
	}
	t.Cleanup(func() { agentdb.CloseDB() })
	t.Setenv("LUI_DB_FOLDER", dbDir)
}

func TestConfigService_New(t *testing.T) {
	svc := NewConfigService()
	if svc == nil {
		t.Fatal("expected non-nil ConfigService")
	}
}

func TestConfigService_PushAndGetConfig(t *testing.T) {
	setupConfigTestDB(t)
	svc := NewConfigService()

	xrayCfg := json.RawMessage(`{"log": {"loglevel": "debug"}}`)
	clientList := json.RawMessage(`[{"email": "test@example.com"}]`)

	err := svc.PushConfig("node-1", "https://hub.example.com", xrayCfg, clientList)
	if err != nil {
		t.Fatalf("PushConfig failed: %v", err)
	}

	ver := svc.GetConfigVersion()
	if ver != 1 {
		t.Fatalf("expected config version 1, got %d", ver)
	}

	cfg, err := svc.GetConfig()
	if err != nil {
		t.Fatalf("GetConfig failed: %v", err)
	}
	if cfg == nil {
		t.Fatal("expected non-nil config")
	}
	if cfg.HubNodeID != "node-1" {
		t.Fatalf("expected HubNodeID 'node-1', got %q", cfg.HubNodeID)
	}
	if cfg.HubEndpoint != "https://hub.example.com" {
		t.Fatalf("expected HubEndpoint 'https://hub.example.com', got %q", cfg.HubEndpoint)
	}
	if string(cfg.XrayConfig) != string(xrayCfg) {
		t.Fatalf("xray config mismatch:\nwant: %s\ngot:  %s", string(xrayCfg), string(cfg.XrayConfig))
	}
	if string(cfg.ClientList) != string(clientList) {
		t.Fatalf("client list mismatch")
	}
}

func TestConfigService_PushConfig_IncrementsVersion(t *testing.T) {
	setupConfigTestDB(t)
	svc := NewConfigService()

	xrayCfg := json.RawMessage(`{"log": {"loglevel": "info"}}`)
	clientList := json.RawMessage(`[]`)

	if err := svc.PushConfig("node-1", "https://hub.example.com", xrayCfg, clientList); err != nil {
		t.Fatalf("first PushConfig failed: %v", err)
	}
	if v := svc.GetConfigVersion(); v != 1 {
		t.Fatalf("expected version 1, got %d", v)
	}

	if err := svc.PushConfig("node-1", "https://hub.example.com", xrayCfg, clientList); err != nil {
		t.Fatalf("second PushConfig failed: %v", err)
	}
	if v := svc.GetConfigVersion(); v != 2 {
		t.Fatalf("expected version 2 after second push, got %d", v)
	}

	if err := svc.PushConfig("node-1", "https://hub.example.com", xrayCfg, clientList); err != nil {
		t.Fatalf("third PushConfig failed: %v", err)
	}
	if v := svc.GetConfigVersion(); v != 3 {
		t.Fatalf("expected version 3 after third push, got %d", v)
	}
}

func TestConfigService_GetConfig_ReturnsNilWhenNoConfig(t *testing.T) {
	setupConfigTestDB(t)
	svc := NewConfigService()

	cfg, err := svc.GetConfig()
	if err != nil {
		t.Fatalf("GetConfig on empty DB returned error: %v", err)
	}
	if cfg != nil {
		t.Fatal("expected nil config when no data exists")
	}
}

func TestConfigService_GetConfigVersion_ReturnsZeroWhenNoConfig(t *testing.T) {
	setupConfigTestDB(t)
	svc := NewConfigService()

	if v := svc.GetConfigVersion(); v != 0 {
		t.Fatalf("expected version 0 for empty DB, got %d", v)
	}
}

func TestConfigService_PushConfig_EmptyLists(t *testing.T) {
	setupConfigTestDB(t)
	svc := NewConfigService()

	err := svc.PushConfig("node-2", "https://hub2.example.com", nil, nil)
	if err != nil {
		t.Fatalf("PushConfig with nil raw messages failed: %v", err)
	}

	cfg, err := svc.GetConfig()
	if err != nil {
		t.Fatalf("GetConfig failed: %v", err)
	}
	if cfg.HubNodeID != "node-2" {
		t.Fatalf("expected HubNodeID 'node-2', got %q", cfg.HubNodeID)
	}
}

func TestConfigService_NoDB(t *testing.T) {
	oldDB := agentdb.GetDB()
	if oldDB != nil {
		agentdb.CloseDB()
	}
	defer func() {
		if oldDB != nil {
			dir := t.TempDir()
			agentdb.InitDB(filepath.Join(dir, "test.db"))
		}
	}()

	svc := NewConfigService()
	cfg, err := svc.GetConfig()
	if err != nil {
		t.Fatalf("GetConfig with nil DB should not error: %v", err)
	}
	if cfg != nil {
		t.Fatal("expected nil config when DB is nil")
	}
	if v := svc.GetConfigVersion(); v != 0 {
		t.Fatalf("expected 0, got %d", v)
	}
}

func TestConfigService_SupportsMultipleNodes(t *testing.T) {
	setupConfigTestDB(t)
	svc := NewConfigService()

	xrayCfg := json.RawMessage(`{"inbounds": [{"port": 10086}]}`)
	clients := json.RawMessage(`[{"email": "a@b.com"}]`)

	if err := svc.PushConfig("alpha", "https://alpha.hub", xrayCfg, clients); err != nil {
		t.Fatalf("PushConfig alpha: %v", err)
	}

	xrayCfg2 := json.RawMessage(`{"inbounds": [{"port": 10087}]}`)
	clients2 := json.RawMessage(`[{"email": "c@d.com"}]`)
	if err := svc.PushConfig("beta", "https://beta.hub", xrayCfg2, clients2); err != nil {
		t.Fatalf("PushConfig beta: %v", err)
	}

	cfg, err := svc.GetConfig()
	if err != nil {
		t.Fatalf("GetConfig: %v", err)
	}
	if cfg.HubNodeID != "beta" {
		t.Fatalf("expected last config to be 'beta', got %q", cfg.HubNodeID)
	}
}

func TestConfigService_ConcurrentAccess(t *testing.T) {
	setupConfigTestDB(t)
	svc := NewConfigService()

	errs := make(chan error, 10)
	for i := 0; i < 10; i++ {
		go func() {
			xrayCfg := json.RawMessage(`{"dummy": true}`)
			if err := svc.PushConfig("node", "hub", xrayCfg, nil); err != nil {
				errs <- err
				return
			}
			svc.GetConfig()
			svc.GetConfigVersion()
			errs <- nil
		}()
	}
	for range 10 {
		if err := <-errs; err != nil {
			t.Errorf("concurrent PushConfig failed: %v", err)
		}
	}
}
