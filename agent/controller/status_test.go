package controller

import (
	"encoding/json"
	"net/http"
	"testing"
)

func TestStatusController_GetStatus(t *testing.T) {
	c, w := newTestContext("GET", "/api/v1/status", "")
	ctrl := &StatusController{}
	ctrl.GetStatus(c)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	var resp struct {
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
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}
	if !resp.Success {
		t.Fatal("expected success=true")
	}
	if resp.Obj == nil {
		t.Fatal("expected non-nil obj")
	}
}
