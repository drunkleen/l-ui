package controller

import (
	"encoding/json"
	"net/http"
	"testing"
	"time"

	"github.com/gofiber/fiber/v3"
)

func TestStatusController_GetStatus(t *testing.T) {
	ctrl := &StatusController{}

	app := fiber.New()
	app.Get("/api/v1/status", ctrl.GetStatus)

	resp, err := app.Test(testRequest("GET", "/api/v1/status", ""), fiber.TestConfig{Timeout: 5 * time.Second})
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
	var result struct {
		Success bool `json:"success"`
		Obj     *struct {
			CPU     float64 `json:"cpu"`
			Mem     struct {
				Current uint64 `json:"current"`
				Total   uint64 `json:"total"`
			} `json:"mem"`
			Disk struct {
				Current uint64 `json:"current"`
				Total   uint64 `json:"total"`
			} `json:"disk"`
		} `json:"obj"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}
	if !result.Success {
		t.Fatal("expected success=true")
	}
	if result.Obj == nil {
		t.Fatal("expected non-nil obj")
	}
}
