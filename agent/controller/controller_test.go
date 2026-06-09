package controller

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/drunkleen/l-ui/internal/nodeauth"
	"github.com/gofiber/fiber/v3"
)

// testRequest creates a new HTTP request for testing.
func testRequest(method, path, body string) *http.Request {
	req := httptest.NewRequest(method, path, strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	return req
}

func TestAuthMiddleware_NoSecret(t *testing.T) {
	app := fiber.New()
	app.Use(AuthMiddleware(""))
	app.Get("/test", func(c fiber.Ctx) error {
		return c.Status(fiber.StatusOK).SendString("passed")
	})

	resp, err := app.Test(testRequest("GET", "/test", ""))
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", resp.StatusCode)
	}
	var body map[string]string
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}
	if body["error"] != "node not registered" {
		t.Fatalf("expected 'node not registered', got %q", body["error"])
	}
}

func TestAuthMiddleware_MissingAuthHeader(t *testing.T) {
	app := fiber.New()
	app.Use(AuthMiddleware("s3cr3t"))
	app.Get("/test", func(c fiber.Ctx) error {
		return c.Status(fiber.StatusOK).SendString("passed")
	})

	resp, err := app.Test(testRequest("GET", "/test", ""))
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", resp.StatusCode)
	}
}

func TestAuthMiddleware_InvalidAuthHeader(t *testing.T) {
	app := fiber.New()
	app.Use(AuthMiddleware("s3cr3t"))
	app.Get("/test", func(c fiber.Ctx) error {
		return c.Status(fiber.StatusOK).SendString("passed")
	})

	req := testRequest("GET", "/test", "")
	req.Header.Set(nodeauth.HeaderAuth, "NotBearer token")

	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", resp.StatusCode)
	}
}

func TestAuthMiddleware_EmptyToken(t *testing.T) {
	app := fiber.New()
	app.Use(AuthMiddleware("s3cr3t"))
	app.Get("/test", func(c fiber.Ctx) error {
		return c.Status(fiber.StatusOK).SendString("passed")
	})

	req := testRequest("GET", "/test", "")
	req.Header.Set(nodeauth.HeaderAuth, "Bearer ")

	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", resp.StatusCode)
	}
}

func TestAuthMiddleware_DirectTokenMatch(t *testing.T) {
	app := fiber.New()
	app.Use(AuthMiddleware("s3cr3t"))
	app.Get("/test", func(c fiber.Ctx) error {
		return c.Status(fiber.StatusOK).SendString("passed")
	})

	req := testRequest("GET", "/test", "")
	req.Header.Set(nodeauth.HeaderAuth, "Bearer s3cr3t")

	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200 (not aborted), got %d", resp.StatusCode)
	}
}

func TestAuthMiddleware_ValidSignature(t *testing.T) {
	secret := "s3cr3t"
	method := "POST"
	path := "/test"
	body := `{"hub_node_id": "node-1"}`
	ts := time.Now().Unix()
	nonce := "unique-nonce-456"
	sig := nodeauth.Sign(secret, method, path, []byte(body), ts, nonce)

	app := fiber.New()
	app.Use(AuthMiddleware(secret))
	app.Post("/test", func(c fiber.Ctx) error {
		return c.Status(fiber.StatusOK).SendString("passed")
	})

	req := testRequest(method, path, body)
	req.Header.Set(nodeauth.HeaderAuth, "Bearer some-other-token")
	req.Header.Set(nodeauth.HeaderTimestamp, fmt.Sprintf("%d", ts))
	req.Header.Set(nodeauth.HeaderNonce, nonce)
	req.Header.Set(nodeauth.HeaderSignature, sig)

	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected valid signature to pass (200), got %d", resp.StatusCode)
	}
}

func TestAuthMiddleware_ExpiredTimestamp(t *testing.T) {
	secret := "s3cr3t"
	method := "POST"
	path := "/test"
	body := `{"hub_node_id": "node-1"}`
	ts := time.Now().Add(-10 * time.Minute).Unix()
	nonce := "stale-nonce"
	sig := nodeauth.Sign(secret, method, path, []byte(body), ts, nonce)

	app := fiber.New()
	app.Use(AuthMiddleware(secret))
	app.Post("/test", func(c fiber.Ctx) error {
		return c.Status(fiber.StatusOK).SendString("passed")
	})

	req := testRequest(method, path, body)
	req.Header.Set(nodeauth.HeaderAuth, "Bearer some-other-token")
	req.Header.Set(nodeauth.HeaderTimestamp, fmt.Sprintf("%d", ts))
	req.Header.Set(nodeauth.HeaderNonce, nonce)
	req.Header.Set(nodeauth.HeaderSignature, sig)

	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("expected 401 for expired timestamp, got %d", resp.StatusCode)
	}
}

func TestAuthMiddleware_MissingSignatureHeaders(t *testing.T) {
	secret := "s3cr3t"

	app := fiber.New()
	app.Use(AuthMiddleware(secret))
	app.Post("/test", func(c fiber.Ctx) error {
		return c.Status(fiber.StatusOK).SendString("passed")
	})

	req := testRequest("POST", "/test", `{"foo": "bar"}`)
	req.Header.Set(nodeauth.HeaderAuth, "Bearer some-other-token")

	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("expected 401 for missing signature headers, got %d", resp.StatusCode)
	}
}

func TestParseInt64_Valid(t *testing.T) {
	v, err := parseInt64("1234567890")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if v != 1234567890 {
		t.Fatalf("expected 1234567890, got %d", v)
	}
}

func TestParseInt64_Invalid(t *testing.T) {
	_, err := parseInt64("not-a-number")
	if err == nil {
		t.Fatal("expected error for invalid input")
	}
}

func TestParseInt64_Empty(t *testing.T) {
	_, err := parseInt64("")
	if err == nil {
		t.Fatal("expected error for empty input")
	}
}

func TestParseInt64_WithWhitespace(t *testing.T) {
	v, err := parseInt64("  9876543210  ")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if v != 9876543210 {
		t.Fatalf("expected 9876543210, got %d", v)
	}
}
