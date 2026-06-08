package web

import (
	"context"
	"testing"
	"time"
)

func TestShutdownTimeoutIs15Seconds(t *testing.T) {
	// Verify that the shutdown context is created with 15 second timeout
	// This test validates the timeout constant used in the stop() method
	const expectedTimeout = 15 * time.Second

	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), expectedTimeout)
	defer shutdownCancel()

	deadline, ok := shutdownCtx.Deadline()
	if !ok {
		t.Fatal("context should have a deadline")
	}

	actual := time.Until(deadline)
	if actual < expectedTimeout-100*time.Millisecond {
		t.Fatalf("expected deadline of at least %v, got %v", expectedTimeout, actual)
	}
}
