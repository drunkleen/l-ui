package controller

import (
	"encoding/json"
	"errors"
	"net/http"
	"testing"
)

func TestRestartController_RestartAgent_ReturnsImmediately(t *testing.T) {
	orig := restartAgentFn
	restartAgentFn = func() {}
	defer func() { restartAgentFn = orig }()

	c, w := newTestContext("POST", "/api/v1/restart", "")
	ctrl := &RestartController{}
	ctrl.RestartAgent(c)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	var resp struct {
		Success bool   `json:"success"`
		Msg     string `json:"msg"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if !resp.Success {
		t.Fatal("expected success=true")
	}
	if resp.Msg != "restarting" {
		t.Fatalf("expected msg 'restarting', got %q", resp.Msg)
	}
}

func TestRestartController_RestartXray_Success(t *testing.T) {
	orig := restartXrayFn
	restartXrayFn = func() error { return nil }
	defer func() { restartXrayFn = orig }()

	c, w := newTestContext("POST", "/api/v1/xray/restart", "")
	ctrl := &RestartController{}
	ctrl.RestartXray(c)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	var resp struct {
		Success bool   `json:"success"`
		Status  string `json:"status"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if !resp.Success {
		t.Fatal("expected success=true")
	}
	if resp.Status != "ok" {
		t.Fatalf("expected status 'ok', got %q", resp.Status)
	}
}

func TestRestartController_RestartXray_Error(t *testing.T) {
	orig := restartXrayFn
	restartXrayFn = func() error { return errors.New("systemctl not found") }
	defer func() { restartXrayFn = orig }()

	c, w := newTestContext("POST", "/api/v1/xray/restart", "")
	ctrl := &RestartController{}
	ctrl.RestartXray(c)

	if w.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d: %s", w.Code, w.Body.String())
	}
	var resp struct {
		Success bool   `json:"success"`
		Status  string `json:"status"`
		Error   string `json:"error"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if resp.Success {
		t.Fatal("expected success=false on error")
	}
	if resp.Status != "error" {
		t.Fatalf("expected status 'error', got %q", resp.Status)
	}
	if resp.Error == "" {
		t.Fatal("expected error message in response")
	}
}

func TestRestartController_RestartAgent_ResponseStructure(t *testing.T) {
	orig := restartAgentFn
	restartAgentFn = func() {}
	defer func() { restartAgentFn = orig }()

	c, w := newTestContext("POST", "/api/v1/restart", "")
	ctrl := &RestartController{}
	ctrl.RestartAgent(c)

	var resp struct {
		Success bool   `json:"success"`
		Msg     string `json:"msg"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if !resp.Success {
		t.Fatal("expected success=true")
	}
}
