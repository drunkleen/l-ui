package controller

import (
	"encoding/json"
	"net/http"
	"path/filepath"
	"testing"

	agentdb "github.com/drunkleen/l-ui/agent/database"
	"github.com/drunkleen/l-ui/agent/service"
	"github.com/gofiber/fiber/v3"
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
	ctrl := &ConfigController{}

	app := fiber.New()
	app.Get("/api/v1/config", ctrl.GetConfig)

	resp, err := app.Test(testRequest("GET", "/api/v1/config", ""))
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", resp.StatusCode, resp.Body)
	}
	var body map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}
	if body["obj"] != nil {
		t.Fatalf("expected null obj, got %v", body["obj"])
	}
}

func TestConfigController_GetConfig_WithConfig(t *testing.T) {
	setupConfigControllerTestDB(t)

	cfgSvc.PushConfig("node-1", "https://hub.example.com",
		json.RawMessage(`{"log": {"loglevel": "debug"}}`),
		json.RawMessage(`[{"email": "test@example.com"}]`),
	)

	ctrl := &ConfigController{}

	app := fiber.New()
	app.Get("/api/v1/config", ctrl.GetConfig)

	resp, err := app.Test(testRequest("GET", "/api/v1/config", ""))
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", resp.StatusCode, resp.Body)
	}
	var env struct {
		Obj service.NodeConfigData `json:"obj"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&env); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}
	if env.Obj.HubNodeID != "node-1" {
		t.Fatalf("expected hub_node_id 'node-1', got %q", env.Obj.HubNodeID)
	}
}

func TestConfigController_PushConfig_ValidRequest(t *testing.T) {
	setupConfigControllerTestDB(t)

	body := `{"hub_node_id": "node-2", "hub_endpoint": "https://hub2.example.com", "xray_config": {"port": 10086}, "client_list": [{"email": "a@b.com"}]}`
	ctrl := &ConfigController{}

	app := fiber.New()
	app.Post("/api/v1/config/push", ctrl.PushConfig)

	resp, err := app.Test(testRequest("POST", "/api/v1/config/push", body))
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", resp.StatusCode, resp.Body)
	}
	var result map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}
	if result["success"] != true {
		t.Fatalf("expected success true, got %v", result["success"])
	}
}

func TestConfigController_PushConfig_InvalidJSON(t *testing.T) {
	setupConfigControllerTestDB(t)

	ctrl := &ConfigController{}

	app := fiber.New()
	app.Post("/api/v1/config/push", ctrl.PushConfig)

	resp, err := app.Test(testRequest("POST", "/api/v1/config/push", `not-json`))
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", resp.StatusCode)
	}
}

func TestConfigController_PushConfig_EmptyBody(t *testing.T) {
	setupConfigControllerTestDB(t)

	ctrl := &ConfigController{}

	app := fiber.New()
	app.Post("/api/v1/config/push", ctrl.PushConfig)

	resp, err := app.Test(testRequest("POST", "/api/v1/config/push", ""))
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", resp.StatusCode)
	}
}

func TestConfigController_ApplyConfig(t *testing.T) {
	ctrl := &ConfigController{}

	app := fiber.New()
	app.Post("/api/v1/config/apply", ctrl.ApplyConfig)

	resp, err := app.Test(testRequest("POST", "/api/v1/config/apply", ""))
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
	var result map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}
	if result["success"] != true {
		t.Fatalf("expected success true, got %v", result["success"])
	}
}

func TestConfigController_PushConfig_UpdatesVersion(t *testing.T) {
	setupConfigControllerTestDB(t)

	body := `{"hub_node_id": "node-3", "hub_endpoint": "https://hub3.example.com"}`
	ctrl := &ConfigController{}

	app := fiber.New()
	app.Post("/api/v1/config/push", ctrl.PushConfig)

	resp, err := app.Test(testRequest("POST", "/api/v1/config/push", body))
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("first push expected 200, got %d", resp.StatusCode)
	}

	resp2, err := app.Test(testRequest("POST", "/api/v1/config/push", body))
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp2.Body.Close()

	if resp2.StatusCode != http.StatusOK {
		t.Fatalf("second push expected 200, got %d", resp2.StatusCode)
	}
	var result map[string]any
	if err := json.NewDecoder(resp2.Body).Decode(&result); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}
	obj, ok := result["obj"].(map[string]any)
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
	ctrl := &ConfigController{}

	app := fiber.New()
	app.Post("/api/v1/config/push", ctrl.PushConfig)
	app.Get("/api/v1/config", ctrl.GetConfig)

	resp, err := app.Test(testRequest("POST", "/api/v1/config/push", body))
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", resp.StatusCode, resp.Body)
	}

	getResp, err := app.Test(testRequest("GET", "/api/v1/config", ""))
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer getResp.Body.Close()

	var env struct {
		Obj service.NodeConfigData `json:"obj"`
	}
	if err := json.NewDecoder(getResp.Body).Decode(&env); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	getRespData := env.Obj
	if getRespData.HubNodeID != "node-full" {
		t.Errorf("hub_node_id mismatch: got %q", getRespData.HubNodeID)
	}
	if getRespData.HubEndpoint != "https://full.hub.example.com" {
		t.Errorf("hub_endpoint mismatch: got %q", getRespData.HubEndpoint)
	}
	if getRespData.XrayConfig == nil {
		t.Error("expected xray_config in response")
	}
	if getRespData.ClientList == nil {
		t.Error("expected client_list in response")
	}
}
