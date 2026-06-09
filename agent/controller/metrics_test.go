package controller

import (
	"encoding/json"
	"net/http"
	"testing"
	"time"

	"github.com/gofiber/fiber/v3"
)

func TestMetricsController_GetMetrics_ReturnsValidJSON(t *testing.T) {
	ctrl := &MetricsController{}

	app := fiber.New()
	app.Get("/api/v1/metrics", ctrl.GetMetrics)

	resp, err := app.Test(testRequest("GET", "/api/v1/metrics", ""), fiber.TestConfig{Timeout: 5 * time.Second})
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
	var body map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}
	expectedFields := []string{
		"cpu_percent", "memory_total", "memory_used", "memory_percent",
		"disk_total", "disk_used", "disk_percent", "uptime",
		"load_1", "load_5", "load_15", "net_sent", "net_recv", "timestamp",
	}
	for _, field := range expectedFields {
		if _, ok := body[field]; !ok {
			t.Errorf("expected field %q in response", field)
		}
	}
}

func TestMetricsController_GetMetrics_TimestampIsString(t *testing.T) {
	ctrl := &MetricsController{}

	app := fiber.New()
	app.Get("/api/v1/metrics", ctrl.GetMetrics)

	resp, err := app.Test(testRequest("GET", "/api/v1/metrics", ""), fiber.TestConfig{Timeout: 5 * time.Second})
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()

	var body map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}
	ts, ok := body["timestamp"].(string)
	if !ok {
		t.Fatal("expected timestamp to be a string")
	}
	if ts == "" {
		t.Fatal("expected non-empty timestamp")
	}
}
