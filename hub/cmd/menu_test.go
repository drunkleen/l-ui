package cmd

import (
	"os"
	"testing"
)

func TestIsDocker(t *testing.T) {
	originalDockerEnv := os.Getenv("LUI_IN_DOCKER")
	defer os.Setenv("LUI_IN_DOCKER", originalDockerEnv)

	tests := []struct {
		name     string
		envValue string
		want     bool
	}{
		{"LUI_IN_DOCKER=true", "true", true},
		{"LUI_IN_DOCKER=false", "false", false},
		{"LUI_IN_DOCKER empty", "", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.envValue != "" {
				os.Setenv("LUI_IN_DOCKER", tt.envValue)
			} else {
				os.Unsetenv("LUI_IN_DOCKER")
			}
			got := isDocker()
			if got != tt.want {
				t.Errorf("isDocker() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestBuildMainMenu(t *testing.T) {
	items := buildMainMenu()
	if len(items) < 3 {
		t.Fatalf("main menu too short: %d", len(items))
	}
	if items[0].label != "Service" {
		t.Errorf("first item = %q, want 'Service'", items[0].label)
	}
	last := items[len(items)-1]
	if last.label != "Exit" {
		t.Errorf("last item = %q, want 'Exit'", last.label)
	}
	if last.run == nil {
		t.Error("Exit item should have a run function")
	}
}

func TestBuildServiceMenu(t *testing.T) {
	items := buildServiceMenu()
	if len(items) < 3 {
		t.Fatalf("service menu too short: %d", len(items))
	}
	if items[0].label != "← Back" {
		t.Errorf("first item = %q, want '← Back'", items[0].label)
	}
}

func TestBuildSecurityMenu(t *testing.T) {
	items := buildSecurityMenu()
	if len(items) < 2 {
		t.Fatalf("security menu too short: %d", len(items))
	}
	if items[0].label != "← Back" {
		t.Errorf("first security item = %q, want '← Back'", items[0].label)
	}
}

func TestPushSubMenu(t *testing.T) {
	main := newMenu("Main", buildMainMenu())
	m := tuiModel{stack: []menuModel{main}}
	// Manually push a Service sub-menu (same logic as Update does)
	svcItems := buildServiceMenu()
	m.stack = append(m.stack, newMenu("Service", svcItems))
	if len(m.stack) != 2 {
		t.Fatalf("stack length = %d, want 2", len(m.stack))
	}
	cur := m.current()
	if cur == nil {
		t.Fatal("current menu is nil")
	}
	if cur.title != "Service" {
		t.Errorf("sub-menu title = %q, want 'Service'", cur.title)
	}
	// First item should be "← Back"
	first := cur.items[0]
	if first.label != "← Back" {
		t.Errorf("first sub-menu item = %q, want '← Back'", first.label)
	}
}

func TestNewMenu(t *testing.T) {
	m := newMenu("Test", []menuItem{{label: "A"}})
	if m.title != "Test" {
		t.Errorf("title = %q, want 'Test'", m.title)
	}
	if len(m.items) != 1 {
		t.Errorf("items = %d, want 1", len(m.items))
	}
	if m.cursor != 0 {
		t.Errorf("cursor = %d, want 0", m.cursor)
	}
}

func TestCurrentMenu(t *testing.T) {
	m := tuiModel{}
	if m.current() != nil {
		t.Error("current() on empty stack should return nil")
	}

	m1 := newMenu("M1", []menuItem{})
	m.stack = []menuModel{m1}
	if m.current() == nil {
		t.Fatal("current() should return menu")
	}
	if m.current().title != "M1" {
		t.Errorf("title = %q, want 'M1'", m.current().title)
	}

	m2 := newMenu("M2", []menuItem{})
	m.stack = append(m.stack, m2)
	if m.current().title != "M2" {
		t.Errorf("title = %q, want 'M2'", m.current().title)
	}
}

func TestBuildSubMenus(t *testing.T) {
	subs := map[string]func() []menuItem{
		"Service":    buildServiceMenu,
		"Settings":   buildSettingsMenu,
		"Security":   buildSecurityMenu,
		"SSL certificate": buildSSLMenu,
		"IP limit (fail2ban)": buildIPLimitMenu,
		"Firewall (UFW)":  buildFirewallMenu,
		"System":     buildSystemMenu,
		"BBR management":  buildBBRMenu,
	}
	for name, builder := range subs {
		items := builder()
		if len(items) == 0 {
			t.Errorf("sub-menu %q is empty", name)
		}
	}
}
