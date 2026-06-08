package cmd

import "testing"

func TestRunCommandExists(t *testing.T) {
	if runCmd == nil {
		t.Fatal("runCmd is nil")
	}
	if runCmd.Use != "run" {
		t.Errorf("Use = %q, want run", runCmd.Use)
	}
}
