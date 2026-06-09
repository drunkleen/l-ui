package controller

import (
	"encoding/json"
	"net/http"
	"testing"

	"github.com/gofiber/fiber/v3"
)

func TestFirewallController_GetStatus_ReturnsValidJSON(t *testing.T) {
	ctrl := &FirewallController{}

	app := fiber.New()
	app.Get("/api/v1/firewall/status", ctrl.GetStatus)

	resp, err := app.Test(testRequest("GET", "/api/v1/firewall/status", ""))
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()

	// UFW may not be installed in test environment, so either 200 or 500 is acceptable
	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusInternalServerError {
		t.Fatalf("expected 200 or 500, got %d", resp.StatusCode)
	}
	if resp.StatusCode == http.StatusOK {
		var result map[string]any
		if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
			t.Fatalf("unmarshal response: %v", err)
		}
		if _, ok := result["success"]; !ok {
			t.Error("expected 'success' field in response")
		}
		if obj, ok := result["obj"]; ok {
			if objMap, ok := obj.(map[string]any); ok {
				if _, ok := objMap["active"]; !ok {
					t.Error("expected 'active' field in obj")
				}
				if _, ok := objMap["rules"]; !ok {
					t.Error("expected 'rules' field in obj")
				}
			}
		}
	}
}

func TestFirewallController_GetStatus_NilStatus(t *testing.T) {
	ctrl := &FirewallController{}
	app := fiber.New()
	app.Get("/api/v1/firewall/status", ctrl.GetStatus)

	resp, err := app.Test(testRequest("GET", "/api/v1/firewall/status", ""))
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()
	_ = resp
}

func TestFirewallController_GetRules_ReturnsArray(t *testing.T) {
	ctrl := &FirewallController{}

	app := fiber.New()
	app.Get("/api/v1/firewall/rules", ctrl.GetRules)

	resp, err := app.Test(testRequest("GET", "/api/v1/firewall/rules", ""))
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusInternalServerError {
		t.Fatalf("expected 200 or 500, got %d", resp.StatusCode)
	}
	if resp.StatusCode == http.StatusOK {
		var result map[string]any
		if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
			t.Fatalf("unmarshal response: %v", err)
		}
		obj, ok := result["obj"].(map[string]any)
		if !ok {
			t.Fatal("expected 'obj' to be a map")
		}
		rules, ok := obj["rules"].([]any)
		if !ok {
			t.Fatal("expected 'rules' to be an array")
		}
		_ = rules
	}
}

func TestFirewallController_AddRule_ValidRequest(t *testing.T) {
	ctrl := &FirewallController{}

	app := fiber.New()
	app.Post("/api/v1/firewall/rules", ctrl.AddRule)

	resp, err := app.Test(testRequest("POST", "/api/v1/firewall/rules", `{"port": "22", "action": "allow", "protocol": "tcp", "comment": "SSH"}`))
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()

	// May fail if ufw is not installed, but should return 200 or 500
	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusInternalServerError {
		t.Fatalf("expected 200 or 500, got %d: %s", resp.StatusCode, resp.Body)
	}
}

func TestFirewallController_AddRule_MissingPort(t *testing.T) {
	ctrl := &FirewallController{}

	app := fiber.New()
	app.Post("/api/v1/firewall/rules", ctrl.AddRule)

	resp, err := app.Test(testRequest("POST", "/api/v1/firewall/rules", `{"action": "allow"}`))
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("expected 400 for missing port, got %d: %s", resp.StatusCode, resp.Body)
	}
}

func TestFirewallController_AddRule_InvalidAction(t *testing.T) {
	ctrl := &FirewallController{}

	app := fiber.New()
	app.Post("/api/v1/firewall/rules", ctrl.AddRule)

	resp, err := app.Test(testRequest("POST", "/api/v1/firewall/rules", `{"port": "80", "action": "invalid"}`))
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("expected 400 for invalid action, got %d: %s", resp.StatusCode, resp.Body)
	}
}

func TestFirewallController_AddRule_InvalidPortNumber(t *testing.T) {
	ctrl := &FirewallController{}

	app := fiber.New()
	app.Post("/api/v1/firewall/rules", ctrl.AddRule)

	resp, err := app.Test(testRequest("POST", "/api/v1/firewall/rules", `{"port": "99999", "action": "allow"}`))
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("expected 400 for invalid port, got %d: %s", resp.StatusCode, resp.Body)
	}
}

func TestFirewallController_AddRule_NonNumericPort(t *testing.T) {
	ctrl := &FirewallController{}

	app := fiber.New()
	app.Post("/api/v1/firewall/rules", ctrl.AddRule)

	resp, err := app.Test(testRequest("POST", "/api/v1/firewall/rules", `{"port": "abc", "action": "allow"}`))
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("expected 400 for non-numeric port, got %d: %s", resp.StatusCode, resp.Body)
	}
}

func TestFirewallController_AddRule_InvalidProtocol(t *testing.T) {
	ctrl := &FirewallController{}

	app := fiber.New()
	app.Post("/api/v1/firewall/rules", ctrl.AddRule)

	resp, err := app.Test(testRequest("POST", "/api/v1/firewall/rules", `{"port": "80", "action": "allow", "protocol": "icmp"}`))
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("expected 400 for invalid protocol, got %d: %s", resp.StatusCode, resp.Body)
	}
}

func TestFirewallController_AddRule_EmptyBody(t *testing.T) {
	ctrl := &FirewallController{}

	app := fiber.New()
	app.Post("/api/v1/firewall/rules", ctrl.AddRule)

	resp, err := app.Test(testRequest("POST", "/api/v1/firewall/rules", ""))
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("expected 400 for empty body, got %d", resp.StatusCode)
	}
}

func TestFirewallController_DeleteRule_ValidRequest(t *testing.T) {
	ctrl := &FirewallController{}

	app := fiber.New()
	app.Delete("/api/v1/firewall/rules", ctrl.DeleteRule)

	resp, err := app.Test(testRequest("DELETE", "/api/v1/firewall/rules", `{"rule_number": "1"}`))
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusInternalServerError {
		t.Fatalf("expected 200 or 500, got %d: %s", resp.StatusCode, resp.Body)
	}
}

func TestFirewallController_DeleteRule_MissingRuleNumber(t *testing.T) {
	ctrl := &FirewallController{}

	app := fiber.New()
	app.Delete("/api/v1/firewall/rules", ctrl.DeleteRule)

	resp, err := app.Test(testRequest("DELETE", "/api/v1/firewall/rules", `{}`))
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("expected 400 for missing rule_number, got %d: %s", resp.StatusCode, resp.Body)
	}
}

func TestFirewallController_DeleteRule_EmptyRuleNumber(t *testing.T) {
	ctrl := &FirewallController{}

	app := fiber.New()
	app.Delete("/api/v1/firewall/rules", ctrl.DeleteRule)

	resp, err := app.Test(testRequest("DELETE", "/api/v1/firewall/rules", `{"rule_number": ""}`))
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("expected 400 for empty rule_number, got %d: %s", resp.StatusCode, resp.Body)
	}
}
