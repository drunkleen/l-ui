package middleware

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/drunkleen/l-ui/hub/web/session"

	"github.com/gofiber/fiber/v3"
	"github.com/gofiber/fiber/v3/extractors"
	fiberSession "github.com/gofiber/fiber/v3/middleware/session"
)

func TestCSRFMiddlewareAllowsSafeMethods(t *testing.T) {
	app := fiber.New()
	app.Use(CSRFMiddleware())
	app.Get("/safe", func(c fiber.Ctx) error {
		return c.Status(http.StatusOK).SendString("ok")
	})

	resp, err := app.Test(httptest.NewRequest(http.MethodGet, "/safe", nil))
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusOK)
	}
}

func TestCSRFMiddlewareRejectsMissingTokenAndAcceptsValidToken(t *testing.T) {
	app := fiber.New()

	cfg := fiberSession.Config{
		CookieHTTPOnly: true,
		CookieSameSite: "Lax",
		CookieSecure:   false,
		CookiePath:     "/",
	}
	cfg.Extractor = extractors.FromCookie("l-ui")
	handler, _ := fiberSession.NewWithStore(cfg)
	app.Use(handler)

	app.Get("/token", func(c fiber.Ctx) error {
		token, err := session.EnsureCSRFToken(c)
		if err != nil {
			t.Fatal(err)
		}
		return c.Status(http.StatusOK).SendString(token)
	})
	app.Post("/submit", CSRFMiddleware(), func(c fiber.Ctx) error {
		return c.Status(http.StatusOK).SendString("ok")
	})

	// Get CSRF token
	tokenResp, err := app.Test(httptest.NewRequest(http.MethodGet, "/token", nil))
	if err != nil {
		t.Fatalf("token request failed: %v", err)
	}
	defer tokenResp.Body.Close()

	if tokenResp.StatusCode != http.StatusOK {
		t.Fatalf("token status = %d, want %d", tokenResp.StatusCode, http.StatusOK)
	}
	cookies := tokenResp.Cookies()
	tokenBytes, err := io.ReadAll(tokenResp.Body)
	if err != nil {
		t.Fatalf("read token body: %v", err)
	}
	token := strings.TrimSpace(string(tokenBytes))

	// Missing token should be rejected
	missingReq := httptest.NewRequest(http.MethodPost, "/submit", nil)
	for _, cookie := range cookies {
		missingReq.AddCookie(cookie)
	}
	missingResp, err := app.Test(missingReq)
	if err != nil {
		t.Fatalf("missing token request failed: %v", err)
	}
	defer missingResp.Body.Close()

	if missingResp.StatusCode != http.StatusForbidden {
		t.Fatalf("missing token status = %d, want %d", missingResp.StatusCode, http.StatusForbidden)
	}

	// Valid token should be accepted
	validReq := httptest.NewRequest(http.MethodPost, "/submit", nil)
	for _, cookie := range cookies {
		validReq.AddCookie(cookie)
	}
	validReq.Header.Set(session.CSRFHeaderName, token)
	validResp, err := app.Test(validReq)
	if err != nil {
		t.Fatalf("valid token request failed: %v", err)
	}
	defer validResp.Body.Close()

	if validResp.StatusCode != http.StatusOK {
		t.Fatalf("valid token status = %d, want %d", validResp.StatusCode, http.StatusOK)
	}
}

func TestSecurityHeadersMiddleware(t *testing.T) {
	app := fiber.New()
	app.Use(SecurityHeadersMiddleware(true))
	app.Get("/", func(c fiber.Ctx) error {
		return c.Status(http.StatusOK).SendString("ok")
	})

	resp, err := app.Test(httptest.NewRequest(http.MethodGet, "/", nil))
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()

	headers := resp.Header
	if got := headers.Get("X-Content-Type-Options"); got != "nosniff" {
		t.Fatalf("X-Content-Type-Options = %q", got)
	}
	if got := headers.Get("X-Frame-Options"); got != "DENY" {
		t.Fatalf("X-Frame-Options = %q", got)
	}
	if got := headers.Get("Referrer-Policy"); got != "no-referrer" {
		t.Fatalf("Referrer-Policy = %q", got)
	}
	if got := headers.Get("Strict-Transport-Security"); got == "" {
		t.Fatal("Strict-Transport-Security should be set for direct HTTPS")
	}
}

func TestSecurityHeadersMiddlewareSkipsHSTSWithoutDirectHTTPS(t *testing.T) {
	app := fiber.New()
	app.Use(SecurityHeadersMiddleware(false))
	app.Get("/", func(c fiber.Ctx) error {
		return c.Status(http.StatusOK).SendString("ok")
	})

	resp, err := app.Test(httptest.NewRequest(http.MethodGet, "/", nil))
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()

	if got := resp.Header.Get("Strict-Transport-Security"); got != "" {
		t.Fatalf("Strict-Transport-Security = %q, want empty", got)
	}
}
