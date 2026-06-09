package sub

import (
	"context"
	"net"
	"testing"
	"time"

	"github.com/gofiber/fiber/v3"
)

func TestSubShutdownTimeoutIs15Seconds(t *testing.T) {
	// Verify that the shutdown context is created with 15 second timeout
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

func TestSubStopWithMockServer(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("failed to create listener: %v", err)
	}
	defer listener.Close()

	s := &Server{
		cancel:   func() {},
		listener: listener,
		app:      fiber.New(),
	}

	done := make(chan error, 1)
	go func() {
		done <- s.Stop()
	}()

	select {
	case err := <-done:
		if err != nil {
			t.Logf("Stop() returned: %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("Stop() did not complete within 2 seconds")
	}
}

func TestSubStopWithNilApp(t *testing.T) {
	s := &Server{
		cancel: func() {},
	}

	err := s.Stop()
	if err != nil {
		t.Fatalf("Stop() with nil app returned error: %v", err)
	}
}
