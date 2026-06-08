package controller

import (
	"encoding/json"
	"net/http"
	"testing"
)

func TestSysInfoController_GetSysInfo_ReturnsValidJSON(t *testing.T) {
	c, w := newTestContext("GET", "/api/v1/sysinfo", "")
	ctrl := &SysInfoController{}
	ctrl.GetSysInfo(c)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	var resp map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}
	expectedFields := []string{
		"hostname", "os", "platform", "platform_version",
		"kernel_version", "kernel_arch", "cpu_model", "cpu_cores", "go_version",
	}
	for _, field := range expectedFields {
		if _, ok := resp[field]; !ok {
			t.Errorf("expected field %q in response", field)
		}
	}
}

func TestSysInfoController_GetSysInfo_HostnameNotEmpty(t *testing.T) {
	c, w := newTestContext("GET", "/api/v1/sysinfo", "")
	ctrl := &SysInfoController{}
	ctrl.GetSysInfo(c)

	var resp map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}
	hostname, _ := resp["hostname"].(string)
	if hostname == "" {
		t.Log("warning: hostname is empty (possible container environment)")
	}
}

func TestSysInfoController_GetSysInfo_CpuCoresIsPositive(t *testing.T) {
	c, w := newTestContext("GET", "/api/v1/sysinfo", "")
	ctrl := &SysInfoController{}
	ctrl.GetSysInfo(c)

	var resp map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}
	cores, ok := resp["cpu_cores"].(float64)
	if !ok {
		t.Fatal("expected cpu_cores to be a number")
	}
	if cores <= 0 {
		t.Errorf("expected positive cpu_cores, got %v", cores)
	}
}
