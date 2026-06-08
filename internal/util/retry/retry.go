package retry

import (
	"context"
	"math"
	"math/rand"
	"strings"
	"time"
)

type Config struct {
	MaxAttempts    int
	InitialBackoff time.Duration
	MaxBackoff     time.Duration
	JitterFactor   float64
}

var DefaultConfig = Config{
	MaxAttempts:    3,
	InitialBackoff: 200 * time.Millisecond,
	MaxBackoff:     5 * time.Second,
	JitterFactor:   0.2,
}

func (c Config) backoff(attempt int) time.Duration {
	if attempt <= 0 {
		attempt = 1
	}
	delay := float64(c.InitialBackoff) * math.Pow(2, float64(attempt-1))
	if delay > float64(c.MaxBackoff) {
		delay = float64(c.MaxBackoff)
	}
	if c.JitterFactor > 0 {
		jitter := delay * c.JitterFactor * (2*rand.Float64() - 1)
		delay += jitter
	}
	if delay < 0 {
		delay = 0
	}
	return time.Duration(delay)
}

func Do(ctx context.Context, cfg Config, fn func(context.Context) error) error {
	if cfg.MaxAttempts <= 0 {
		cfg.MaxAttempts = DefaultConfig.MaxAttempts
	}
	if cfg.InitialBackoff <= 0 {
		cfg.InitialBackoff = DefaultConfig.InitialBackoff
	}
	if cfg.MaxBackoff <= 0 {
		cfg.MaxBackoff = DefaultConfig.MaxBackoff
	}
	if cfg.JitterFactor <= 0 {
		cfg.JitterFactor = DefaultConfig.JitterFactor
	}

	var lastErr error
	for attempt := 0; attempt < cfg.MaxAttempts; attempt++ {
		if err := ctx.Err(); err != nil {
			return err
		}

		lastErr = fn(ctx)
		if lastErr == nil {
			return nil
		}

		if !IsTransient(lastErr) {
			return lastErr
		}

		if attempt == cfg.MaxAttempts-1 {
			return lastErr
		}

		delay := cfg.backoff(attempt + 1)
		timer := time.NewTimer(delay)
		select {
		case <-ctx.Done():
			timer.Stop()
			return ctx.Err()
		case <-timer.C:
		}
	}
	return lastErr
}

func IsTransient(err error) bool {
	if err == nil {
		return false
	}
	errStr := strings.ToLower(err.Error())
	if strings.Contains(errStr, "connection refused") ||
		strings.Contains(errStr, "no such host") ||
		strings.Contains(errStr, "i/o timeout") ||
		strings.Contains(errStr, "tls handshake") ||
		strings.Contains(errStr, "reset by peer") ||
		strings.Contains(errStr, "broken pipe") ||
		strings.Contains(errStr, "connection closed") ||
		strings.Contains(errStr, "eof") ||
		strings.Contains(errStr, "http 429") ||
		strings.Contains(errStr, "http 5") ||
		strings.Contains(errStr, "remote returned http 5") ||
		strings.Contains(errStr, "remote returned http 429") {
		return true
	}
	return false
}
