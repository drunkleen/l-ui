package controller

import (
	"encoding/json"
	"net/http"
	"testing"
)

func TestMetricsController_GetMetrics_ReturnsValidJSON(t *testing.T) {
	c, w := newTestContext("GET", "/api/v1/metrics", "")
	ctrl := &MetricsController{}
	ctrl.GetMetrics(c)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	var resp map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}
	expectedFields := []string{
		"cpu_percent", "memory_total", "memory_used", "memory_percent",
		"disk_total", "disk_used", "disk_percent", "uptime",
		"load_1", "load_5", "load_15", "net_sent", "net_recv", "timestamp",
	}
	for _, field := range expectedFields {
		if _, ok := resp[field]; !ok {
			t.Errorf("expected field %q in response", field)
		}
	}
}

func TestMetricsController_GetMetrics_TimestampIsString(t *testing.T) {
	c, w := newTestContext("GET", "/api/v1/metrics", "")
	ctrl := &MetricsController{}
	ctrl.GetMetrics(c)

	var resp map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}
	ts, ok := resp["timestamp"].(string)
	if !ok {
		t.Fatal("expected timestamp to be a string")
	}
	if ts == "" {
		t.Fatal("expected non-empty timestamp")
	}
}
