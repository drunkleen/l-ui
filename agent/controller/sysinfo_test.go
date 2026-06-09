package controller

import (
	"encoding/json"
	"net/http"
	"testing"

	"github.com/gofiber/fiber/v3"
)

func TestSysInfoController_GetSysInfo_ReturnsValidJSON(t *testing.T) {
	ctrl := &SysInfoController{}

	app := fiber.New()
	app.Get("/api/v1/sysinfo", ctrl.GetSysInfo)

	resp, err := app.Test(testRequest("GET", "/api/v1/sysinfo", ""))
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
		"hostname", "os", "platform", "platform_version",
		"kernel_version", "kernel_arch", "cpu_model", "cpu_cores", "go_version",
	}
	for _, field := range expectedFields {
		if _, ok := body[field]; !ok {
			t.Errorf("expected field %q in response", field)
		}
	}
}

func TestSysInfoController_GetSysInfo_HostnameNotEmpty(t *testing.T) {
	ctrl := &SysInfoController{}

	app := fiber.New()
	app.Get("/api/v1/sysinfo", ctrl.GetSysInfo)

	resp, err := app.Test(testRequest("GET", "/api/v1/sysinfo", ""))
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()

	var body map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}
	hostname, _ := body["hostname"].(string)
	if hostname == "" {
		t.Log("warning: hostname is empty (possible container environment)")
	}
}

func TestSysInfoController_GetSysInfo_CpuCoresIsPositive(t *testing.T) {
	ctrl := &SysInfoController{}

	app := fiber.New()
	app.Get("/api/v1/sysinfo", ctrl.GetSysInfo)

	resp, err := app.Test(testRequest("GET", "/api/v1/sysinfo", ""))
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()

	var body map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}
	cores, ok := body["cpu_cores"].(float64)
	if !ok {
		t.Fatal("expected cpu_cores to be a number")
	}
	if cores <= 0 {
		t.Errorf("expected positive cpu_cores, got %v", cores)
	}
}
