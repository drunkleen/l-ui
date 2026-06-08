package controller

import (
	"encoding/json"
	"net/http"
	"testing"
)

func TestFirewallController_GetStatus_ReturnsValidJSON(t *testing.T) {
	c, w := newTestContext("GET", "/api/v1/firewall/status", "")
	ctrl := &FirewallController{}
	ctrl.GetStatus(c)

	// UFW may not be installed in test environment, so either 200 or 500 is acceptable
	if w.Code != http.StatusOK && w.Code != http.StatusInternalServerError {
		t.Fatalf("expected 200 or 500, got %d", w.Code)
	}
	if w.Code == http.StatusOK {
		var resp map[string]any
		if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
			t.Fatalf("unmarshal response: %v", err)
		}
		if _, ok := resp["success"]; !ok {
			t.Error("expected 'success' field in response")
		}
		if obj, ok := resp["obj"]; ok {
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
	// This tests the case when GetStatus returns nil (no ufw available)
	// We just verify the handler doesn't panic
	c, w := newTestContext("GET", "/api/v1/firewall/status", "")
	ctrl := &FirewallController{}
	ctrl.GetStatus(c)
	_ = w
}

func TestFirewallController_GetRules_ReturnsArray(t *testing.T) {
	c, w := newTestContext("GET", "/api/v1/firewall/rules", "")
	ctrl := &FirewallController{}
	ctrl.GetRules(c)

	if w.Code != http.StatusOK && w.Code != http.StatusInternalServerError {
		t.Fatalf("expected 200 or 500, got %d", w.Code)
	}
	if w.Code == http.StatusOK {
		var resp map[string]any
		if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
			t.Fatalf("unmarshal response: %v", err)
		}
		obj, ok := resp["obj"].(map[string]any)
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
	c, w := newTestContext("POST", "/api/v1/firewall/rules", `{"port": "22", "action": "allow", "protocol": "tcp", "comment": "SSH"}`)
	ctrl := &FirewallController{}
	ctrl.AddRule(c)

	// May fail if ufw is not installed, but should return 200 or 500
	if w.Code != http.StatusOK && w.Code != http.StatusInternalServerError {
		t.Fatalf("expected 200 or 500, got %d: %s", w.Code, w.Body.String())
	}
}

func TestFirewallController_AddRule_MissingPort(t *testing.T) {
	c, w := newTestContext("POST", "/api/v1/firewall/rules", `{"action": "allow"}`)
	ctrl := &FirewallController{}
	ctrl.AddRule(c)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for missing port, got %d: %s", w.Code, w.Body.String())
	}
}

func TestFirewallController_AddRule_InvalidAction(t *testing.T) {
	c, w := newTestContext("POST", "/api/v1/firewall/rules", `{"port": "80", "action": "invalid"}`)
	ctrl := &FirewallController{}
	ctrl.AddRule(c)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for invalid action, got %d: %s", w.Code, w.Body.String())
	}
}

func TestFirewallController_AddRule_InvalidPortNumber(t *testing.T) {
	c, w := newTestContext("POST", "/api/v1/firewall/rules", `{"port": "99999", "action": "allow"}`)
	ctrl := &FirewallController{}
	ctrl.AddRule(c)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for invalid port, got %d: %s", w.Code, w.Body.String())
	}
}

func TestFirewallController_AddRule_NonNumericPort(t *testing.T) {
	c, w := newTestContext("POST", "/api/v1/firewall/rules", `{"port": "abc", "action": "allow"}`)
	ctrl := &FirewallController{}
	ctrl.AddRule(c)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for non-numeric port, got %d: %s", w.Code, w.Body.String())
	}
}

func TestFirewallController_AddRule_InvalidProtocol(t *testing.T) {
	c, w := newTestContext("POST", "/api/v1/firewall/rules", `{"port": "80", "action": "allow", "protocol": "icmp"}`)
	ctrl := &FirewallController{}
	ctrl.AddRule(c)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for invalid protocol, got %d: %s", w.Code, w.Body.String())
	}
}

func TestFirewallController_AddRule_EmptyBody(t *testing.T) {
	c, w := newTestContext("POST", "/api/v1/firewall/rules", "")
	ctrl := &FirewallController{}
	ctrl.AddRule(c)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for empty body, got %d", w.Code)
	}
}

func TestFirewallController_DeleteRule_ValidRequest(t *testing.T) {
	c, w := newTestContext("DELETE", "/api/v1/firewall/rules", `{"rule_number": "1"}`)
	ctrl := &FirewallController{}
	ctrl.DeleteRule(c)

	if w.Code != http.StatusOK && w.Code != http.StatusInternalServerError {
		t.Fatalf("expected 200 or 500, got %d: %s", w.Code, w.Body.String())
	}
}

func TestFirewallController_DeleteRule_MissingRuleNumber(t *testing.T) {
	c, w := newTestContext("DELETE", "/api/v1/firewall/rules", `{}`)
	ctrl := &FirewallController{}
	ctrl.DeleteRule(c)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for missing rule_number, got %d: %s", w.Code, w.Body.String())
	}
}

func TestFirewallController_DeleteRule_EmptyRuleNumber(t *testing.T) {
	c, w := newTestContext("DELETE", "/api/v1/firewall/rules", `{"rule_number": ""}`)
	ctrl := &FirewallController{}
	ctrl.DeleteRule(c)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for empty rule_number, got %d: %s", w.Code, w.Body.String())
	}
}
