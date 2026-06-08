package retry

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestDo_SuccessOnFirstAttempt(t *testing.T) {
	attempts := 0
	err := Do(context.Background(), DefaultConfig, func(ctx context.Context) error {
		attempts++
		return nil
	})
	if err != nil {
		t.Fatalf("expected nil, got %v", err)
	}
	if attempts != 1 {
		t.Fatalf("expected 1 attempt, got %d", attempts)
	}
}

func TestDo_RetriesOnTransientFailure(t *testing.T) {
	attempts := 0
	err := Do(context.Background(), Config{
		MaxAttempts:    3,
		InitialBackoff: 1 * time.Millisecond,
		MaxBackoff:     10 * time.Millisecond,
		JitterFactor:   0,
	}, func(ctx context.Context) error {
		attempts++
		if attempts < 3 {
			return errors.New("connection refused")
		}
		return nil
	})
	if err != nil {
		t.Fatalf("expected nil, got %v", err)
	}
	if attempts != 3 {
		t.Fatalf("expected 3 attempts, got %d", attempts)
	}
}

func TestDo_DoesNotRetryNonTransient(t *testing.T) {
	attempts := 0
	err := Do(context.Background(), Config{
		MaxAttempts:    3,
		InitialBackoff: 1 * time.Millisecond,
	}, func(ctx context.Context) error {
		attempts++
		return errors.New("invalid request")
	})
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if attempts != 1 {
		t.Fatalf("expected 1 attempt, got %d", attempts)
	}
}

func TestDo_ExhaustsRetries(t *testing.T) {
	attempts := 0
	err := Do(context.Background(), Config{
		MaxAttempts:    3,
		InitialBackoff: 1 * time.Millisecond,
		MaxBackoff:     5 * time.Millisecond,
		JitterFactor:   0,
	}, func(ctx context.Context) error {
		attempts++
		return errors.New("connection refused")
	})
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if attempts != 3 {
		t.Fatalf("expected 3 attempts, got %d", attempts)
	}
}

func TestDo_ContextCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	err := Do(ctx, Config{
		MaxAttempts:    5,
		InitialBackoff: 100 * time.Millisecond,
	}, func(ctx context.Context) error {
		return errors.New("connection refused")
	})
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("expected context.Canceled, got %v", err)
	}
}

func TestIsTransient(t *testing.T) {
	tests := []struct {
		err    error
		expect bool
	}{
		{errors.New("connection refused"), true},
		{errors.New("no such host"), true},
		{errors.New("i/o timeout"), true},
		{errors.New("TLS handshake error"), true},
		{errors.New("connection reset by peer"), true},
		{errors.New("broken pipe"), true},
		{errors.New("connection closed"), true},
		{errors.New("EOF"), true},
		{errors.New("remote returned http 503"), true},
		{errors.New("remote returned http 429"), true},
		{errors.New("HTTP 500 Internal Server"), true},
		{errors.New("invalid request"), false},
		{errors.New("not found"), false},
		{errors.New("unauthorized"), false},
		{nil, false},
	}

	for _, tt := range tests {
		got := IsTransient(tt.err)
		if got != tt.expect {
			t.Errorf("IsTransient(%q) = %v, want %v", tt.err, got, tt.expect)
		}
	}
}

func TestBackoff_Exponential(t *testing.T) {
	cfg := Config{
		InitialBackoff: 100 * time.Millisecond,
		MaxBackoff:     10 * time.Second,
		JitterFactor:   0,
	}

	b1 := cfg.backoff(1)
	b2 := cfg.backoff(2)
	b3 := cfg.backoff(3)

	if b1 != 100*time.Millisecond {
		t.Fatalf("backoff(1) = %v, want 100ms", b1)
	}
	if b2 != 200*time.Millisecond {
		t.Fatalf("backoff(2) = %v, want 200ms", b2)
	}
	if b3 != 400*time.Millisecond {
		t.Fatalf("backoff(3) = %v, want 400ms", b3)
	}
}

func TestBackoff_ClampsToMax(t *testing.T) {
	cfg := Config{
		InitialBackoff: 1 * time.Second,
		MaxBackoff:     3 * time.Second,
		JitterFactor:   0,
	}

	b := cfg.backoff(10)
	if b > 3*time.Second {
		t.Fatalf("backoff(10) = %v, want <= 3s", b)
	}
}
