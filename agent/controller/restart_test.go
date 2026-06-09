package controller

import (
	"encoding/json"
	"errors"
	"net/http"
	"testing"

	"github.com/gofiber/fiber/v3"
)

func TestRestartController_RestartAgent_ReturnsImmediately(t *testing.T) {
	orig := restartAgentFn
	restartAgentFn = func() {}
	defer func() { restartAgentFn = orig }()

	ctrl := &RestartController{}

	app := fiber.New()
	app.Post("/api/v1/restart", ctrl.RestartAgent)

	resp, err := app.Test(testRequest("POST", "/api/v1/restart", ""))
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", resp.StatusCode, resp.Body)
	}
	var result struct {
		Success bool   `json:"success"`
		Msg     string `json:"msg"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if !result.Success {
		t.Fatal("expected success=true")
	}
	if result.Msg != "restarting" {
		t.Fatalf("expected msg 'restarting', got %q", result.Msg)
	}
}

func TestRestartController_RestartXray_Success(t *testing.T) {
	orig := restartXrayFn
	restartXrayFn = func() error { return nil }
	defer func() { restartXrayFn = orig }()

	ctrl := &RestartController{}

	app := fiber.New()
	app.Post("/api/v1/xray/restart", ctrl.RestartXray)

	resp, err := app.Test(testRequest("POST", "/api/v1/xray/restart", ""))
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", resp.StatusCode, resp.Body)
	}
	var result struct {
		Success bool   `json:"success"`
		Status  string `json:"status"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if !result.Success {
		t.Fatal("expected success=true")
	}
	if result.Status != "ok" {
		t.Fatalf("expected status 'ok', got %q", result.Status)
	}
}

func TestRestartController_RestartXray_Error(t *testing.T) {
	orig := restartXrayFn
	restartXrayFn = func() error { return errors.New("systemctl not found") }
	defer func() { restartXrayFn = orig }()

	ctrl := &RestartController{}

	app := fiber.New()
	app.Post("/api/v1/xray/restart", ctrl.RestartXray)

	resp, err := app.Test(testRequest("POST", "/api/v1/xray/restart", ""))
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d: %s", resp.StatusCode, resp.Body)
	}
	var result struct {
		Success bool   `json:"success"`
		Status  string `json:"status"`
		Error   string `json:"error"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if result.Success {
		t.Fatal("expected success=false on error")
	}
	if result.Status != "error" {
		t.Fatalf("expected status 'error', got %q", result.Status)
	}
	if result.Error == "" {
		t.Fatal("expected error message in response")
	}
}

func TestRestartController_RestartAgent_ResponseStructure(t *testing.T) {
	orig := restartAgentFn
	restartAgentFn = func() {}
	defer func() { restartAgentFn = orig }()

	ctrl := &RestartController{}

	app := fiber.New()
	app.Post("/api/v1/restart", ctrl.RestartAgent)

	resp, err := app.Test(testRequest("POST", "/api/v1/restart", ""))
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()

	var result struct {
		Success bool   `json:"success"`
		Msg     string `json:"msg"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if !result.Success {
		t.Fatal("expected success=true")
	}
}
