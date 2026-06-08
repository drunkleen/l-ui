package controller

import (
	"encoding/json"
	"net/http"
	"path/filepath"
	"testing"

	agentdb "github.com/drunkleen/l-ui/agent/database"
	"github.com/drunkleen/l-ui/agent/service"
)

func setupConfigControllerTestDB(t *testing.T) {
	t.Helper()
	dbDir := t.TempDir()
	dbPath := filepath.Join(dbDir, "l-ui-agent.db")
	if err := agentdb.InitDB(dbPath); err != nil {
		t.Fatalf("InitDB failed: %v", err)
	}
	t.Cleanup(func() { agentdb.CloseDB() })
}

func TestConfigController_GetConfig_NoConfig(t *testing.T) {
	setupConfigControllerTestDB(t)
	c, w := newTestContext("GET", "/api/v1/config", "")
	ctrl := &ConfigController{}
	ctrl.GetConfig(c)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	var resp map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}
	if resp["obj"] != nil {
		t.Fatalf("expected null obj, got %v", resp["obj"])
	}
}

func TestConfigController_GetConfig_WithConfig(t *testing.T) {
	setupConfigControllerTestDB(t)

	cfgSvc.PushConfig("node-1", "https://hub.example.com",
		json.RawMessage(`{"log": {"loglevel": "debug"}}`),
		json.RawMessage(`[{"email": "test@example.com"}]`),
	)

	c, w := newTestContext("GET", "/api/v1/config", "")
	ctrl := &ConfigController{}
	ctrl.GetConfig(c)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	var env struct {
		Obj service.NodeConfigData `json:"obj"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &env); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}
	if env.Obj.HubNodeID != "node-1" {
		t.Fatalf("expected hub_node_id 'node-1', got %q", env.Obj.HubNodeID)
	}
}

func TestConfigController_PushConfig_ValidRequest(t *testing.T) {
	setupConfigControllerTestDB(t)

	body := `{"hub_node_id": "node-2", "hub_endpoint": "https://hub2.example.com", "xray_config": {"port": 10086}, "client_list": [{"email": "a@b.com"}]}`
	c, w := newTestContext("POST", "/api/v1/config/push", body)
	ctrl := &ConfigController{}
	ctrl.PushConfig(c)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	var resp map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}
	if resp["success"] != true {
		t.Fatalf("expected success true, got %v", resp["success"])
	}
}

func TestConfigController_PushConfig_InvalidJSON(t *testing.T) {
	setupConfigControllerTestDB(t)

	c, w := newTestContext("POST", "/api/v1/config/push", `not-json`)
	ctrl := &ConfigController{}
	ctrl.PushConfig(c)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}
}

func TestConfigController_PushConfig_EmptyBody(t *testing.T) {
	setupConfigControllerTestDB(t)

	c, w := newTestContext("POST", "/api/v1/config/push", "")
	ctrl := &ConfigController{}
	ctrl.PushConfig(c)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}
}

func TestConfigController_ApplyConfig(t *testing.T) {
	c, w := newTestContext("POST", "/api/v1/config/apply", "")
	ctrl := &ConfigController{}
	ctrl.ApplyConfig(c)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	var resp map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}
	if resp["success"] != true {
		t.Fatalf("expected success true, got %v", resp["success"])
	}
}

func TestConfigController_PushConfig_UpdatesVersion(t *testing.T) {
	setupConfigControllerTestDB(t)

	body := `{"hub_node_id": "node-3", "hub_endpoint": "https://hub3.example.com"}`
	c, w := newTestContext("POST", "/api/v1/config/push", body)
	ctrl := &ConfigController{}
	ctrl.PushConfig(c)

	if w.Code != http.StatusOK {
		t.Fatalf("first push expected 200, got %d", w.Code)
	}

	c2, w2 := newTestContext("POST", "/api/v1/config/push", body)
	ctrl.PushConfig(c2)

	if w2.Code != http.StatusOK {
		t.Fatalf("second push expected 200, got %d", w2.Code)
	}
	var resp map[string]any
	if err := json.Unmarshal(w2.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}
	obj, ok := resp["obj"].(map[string]any)
	if !ok {
		t.Fatal("expected obj in response")
	}
	ver, ok := obj["config_version"].(float64)
	if !ok {
		t.Fatal("expected config_version in obj")
	}
	if ver != 2 {
		t.Fatalf("expected config_version 2, got %v", ver)
	}
}

func TestConfigController_PushConfig_AllFields(t *testing.T) {
	setupConfigControllerTestDB(t)

	body := `{
		"hub_node_id": "node-full",
		"hub_endpoint": "https://full.hub.example.com",
		"xray_config": {"inbounds": [{"port": 443}]},
		"client_list": [{"email": "full@test.com", "enable": true}]
	}`
	c, w := newTestContext("POST", "/api/v1/config/push", body)
	ctrl := &ConfigController{}
	ctrl.PushConfig(c)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var env struct {
		Obj service.NodeConfigData `json:"obj"`
	}
	getC, getW := newTestContext("GET", "/api/v1/config", "")
	ctrl.GetConfig(getC)
	if err := json.Unmarshal(getW.Body.Bytes(), &env); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	getResp := env.Obj
	if getResp.HubNodeID != "node-full" {
		t.Errorf("hub_node_id mismatch: got %q", getResp.HubNodeID)
	}
	if getResp.HubEndpoint != "https://full.hub.example.com" {
		t.Errorf("hub_endpoint mismatch: got %q", getResp.HubEndpoint)
	}
	if getResp.XrayConfig == nil {
		t.Error("expected xray_config in response")
	}
	if getResp.ClientList == nil {
		t.Error("expected client_list in response")
	}
}
