package session

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/base64"
	"io"

	"github.com/gofiber/fiber/v3"
)

const csrfTokenKey = "CSRF_TOKEN"

// CSRFHeaderName is the request header used by browser clients for unsafe methods.
const CSRFHeaderName = "X-CSRF-Token"

// EnsureCSRFToken returns the current session CSRF token or creates one.
func EnsureCSRFToken(c fiber.Ctx) (string, error) {
	s := sessionForContext(c)
	if s == nil {
		return "", nil
	}
	if token, ok := s.Get(csrfTokenKey).(string); ok && token != "" {
		return token, nil
	}
	token, err := newCSRFToken()
	if err != nil {
		return "", err
	}
	s.Set(csrfTokenKey, token)
	return token, nil
}

// ValidateCSRFToken checks the submitted CSRF token against the session token.
func ValidateCSRFToken(c fiber.Ctx) bool {
	s := sessionForContext(c)
	if s == nil {
		return false
	}
	expected, ok := s.Get(csrfTokenKey).(string)
	if !ok || expected == "" {
		return false
	}
	actual := c.Get("X-CSRF-Token")
	if actual == "" {
		actual = c.FormValue("_csrf")
	}
	if len(actual) != len(expected) {
		return false
	}
	return subtle.ConstantTimeCompare([]byte(actual), []byte(expected)) == 1
}

func newCSRFToken() (string, error) {
	buf := make([]byte, 32)
	if _, err := io.ReadFull(rand.Reader, buf); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(buf), nil
}
