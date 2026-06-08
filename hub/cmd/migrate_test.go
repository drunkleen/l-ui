package cmd

import "testing"

func TestMigrateCommandExists(t *testing.T) {
	if migrateCmd == nil {
		t.Fatal("migrateCmd is nil")
	}
	if migrateCmd.Use != "migrate" {
		t.Errorf("Use = %q, want migrate", migrateCmd.Use)
	}
}
