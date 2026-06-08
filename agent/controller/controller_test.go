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
	"github.com/gin-gonic/gin"
)

func init() {
	gin.SetMode(gin.TestMode)
}

func newTestContext(method, path, body string) (*gin.Context, *httptest.ResponseRecorder) {
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(method, path, strings.NewReader(body))
	c.Request.Header.Set("Content-Type", "application/json")
	return c, w
}

func TestAuthMiddleware_NoSecret(t *testing.T) {
	c, w := newTestContext("GET", "/api/v1/status", "")
	handler := AuthMiddleware("")
	handler(c)

	if w.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", w.Code)
	}
	var resp map[string]string
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}
	if resp["error"] != "node not registered" {
		t.Fatalf("expected 'node not registered', got %q", resp["error"])
	}
}

func TestAuthMiddleware_MissingAuthHeader(t *testing.T) {
	c, w := newTestContext("GET", "/api/v1/status", "")
	handler := AuthMiddleware("s3cr3t")
	handler(c)

	if w.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", w.Code)
	}
}

func TestAuthMiddleware_InvalidAuthHeader(t *testing.T) {
	c, w := newTestContext("GET", "/api/v1/status", "")
	c.Request.Header.Set(nodeauth.HeaderAuth, "NotBearer token")
	handler := AuthMiddleware("s3cr3t")
	handler(c)

	if w.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", w.Code)
	}
}

func TestAuthMiddleware_EmptyToken(t *testing.T) {
	c, w := newTestContext("GET", "/api/v1/status", "")
	c.Request.Header.Set(nodeauth.HeaderAuth, "Bearer ")
	handler := AuthMiddleware("s3cr3t")
	handler(c)

	if w.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", w.Code)
	}
}

func TestAuthMiddleware_DirectTokenMatch(t *testing.T) {
	c, w := newTestContext("GET", "/api/v1/status", "")
	c.Request.Header.Set(nodeauth.HeaderAuth, "Bearer s3cr3t")
	handler := AuthMiddleware("s3cr3t")
	handler(c)
	if c.IsAborted() {
		t.Fatal("request should not be aborted")
	}
	_ = w
}

func TestAuthMiddleware_ValidSignature(t *testing.T) {
	secret := "s3cr3t"
	method := "POST"
	path := "/api/v1/config/push"
	body := `{"hub_node_id": "node-1"}`
	ts := time.Now().Unix()
	nonce := "unique-nonce-456"
	sig := nodeauth.Sign(secret, method, path, []byte(body), ts, nonce)

	c, w := newTestContext(method, path, body)
	c.Request.Header.Set(nodeauth.HeaderAuth, "Bearer some-other-token")
	c.Request.Header.Set(nodeauth.HeaderTimestamp, fmt.Sprintf("%d", ts))
	c.Request.Header.Set(nodeauth.HeaderNonce, nonce)
	c.Request.Header.Set(nodeauth.HeaderSignature, sig)

	handler := AuthMiddleware(secret)
	handler(c)

	if c.IsAborted() {
		t.Fatalf("expected valid signature to pass, got aborted with code %d", w.Code)
	}
}

func TestAuthMiddleware_ExpiredTimestamp(t *testing.T) {
	secret := "s3cr3t"
	method := "POST"
	path := "/api/v1/config/push"
	body := `{"hub_node_id": "node-1"}`
	ts := time.Now().Add(-10 * time.Minute).Unix()
	nonce := "stale-nonce"
	sig := nodeauth.Sign(secret, method, path, []byte(body), ts, nonce)

	c, w := newTestContext(method, path, body)
	c.Request.Header.Set(nodeauth.HeaderAuth, "Bearer some-other-token")
	c.Request.Header.Set(nodeauth.HeaderTimestamp, fmt.Sprintf("%d", ts))
	c.Request.Header.Set(nodeauth.HeaderNonce, nonce)
	c.Request.Header.Set(nodeauth.HeaderSignature, sig)

	handler := AuthMiddleware(secret)
	handler(c)

	if w.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 for expired timestamp, got %d", w.Code)
	}
}

func TestAuthMiddleware_MissingSignatureHeaders(t *testing.T) {
	secret := "s3cr3t"
	c, w := newTestContext("POST", "/api/v1/config/push", `{"foo": "bar"}`)
	c.Request.Header.Set(nodeauth.HeaderAuth, "Bearer some-other-token")

	handler := AuthMiddleware(secret)
	handler(c)

	if w.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 for missing signature headers, got %d", w.Code)
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
