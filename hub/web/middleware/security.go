package middleware

import (
	"crypto/rand"
	"encoding/base64"

	"github.com/drunkleen/l-ui/hub/web/session"

	"github.com/gofiber/fiber/v3"
)

// SecurityHeadersMiddleware adds browser hardening headers to panel responses.
func SecurityHeadersMiddleware(directHTTPS bool) fiber.Handler {
	return func(c fiber.Ctx) error {
		nonce := newCSPNonce()
		c.Locals("csp_nonce", nonce)
		c.Set("X-Content-Type-Options", "nosniff")
		c.Set("X-Frame-Options", "DENY")
		c.Set("Referrer-Policy", "no-referrer")
		c.Set("Content-Security-Policy", "default-src 'self'; script-src 'self' 'nonce-"+nonce+"'; style-src 'self' 'unsafe-inline'; img-src 'self' data: blob:; font-src 'self' data:; connect-src 'self' ws: wss:; object-src 'none'; frame-ancestors 'none'; base-uri 'self'; form-action 'self'")
		if directHTTPS {
			c.Set("Strict-Transport-Security", "max-age=31536000; includeSubDomains")
		}
		return c.Next()
	}
}

func newCSPNonce() string {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		return ""
	}
	return base64.RawStdEncoding.EncodeToString(b[:])
}

// CSRFMiddleware rejects unsafe requests that do not include the session CSRF token.
// Bearer-token-authenticated callers (api_authed flag set by APIController.checkAPIAuth)
// short-circuit the CSRF check — they are not browser sessions, so the
// cross-site request forgery threat model doesn't apply to them.
func CSRFMiddleware() fiber.Handler {
	return func(c fiber.Ctx) error {
		if v, ok := c.Locals("api_authed").(bool); ok && v {
			return c.Next()
		}
		if isSafeMethod(c.Method()) {
			return c.Next()
		}
		if !session.ValidateCSRFToken(c) {
			return c.SendStatus(fiber.StatusForbidden)
		}
		return c.Next()
	}
}

func isSafeMethod(method string) bool {
	switch method {
	case fiber.MethodGet, fiber.MethodHead, fiber.MethodOptions, fiber.MethodTrace:
		return true
	default:
		return false
	}
}
