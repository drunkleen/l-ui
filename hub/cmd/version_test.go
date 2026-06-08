package cmd

import "testing"

func TestVersionCommandExists(t *testing.T) {
	if versionCmd == nil {
		t.Fatal("versionCmd is nil")
	}
	if versionCmd.Use != "version" {
		t.Errorf("Use = %q, want version", versionCmd.Use)
	}
}
