package controller

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/json"
	"encoding/pem"
	"math/big"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/drunkleen/l-ui/internal/logger"
	"github.com/gin-gonic/gin"
)

func init() {
	logger.InitLogger("info")
}

func mustMarshal(t *testing.T, v any) string {
	t.Helper()
	b, err := json.Marshal(v)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	return string(b)
}

func generateTestCertPEM(t *testing.T) (certPEM, keyPEM string) {
	t.Helper()
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatalf("generate key: %v", err)
	}
	serial, _ := rand.Int(rand.Reader, new(big.Int).Lsh(big.NewInt(1), 128))
	tmpl := &x509.Certificate{
		SerialNumber: serial,
		Subject:      pkix.Name{CommonName: "test"},
		NotBefore:    time.Now().Add(-1 * time.Hour),
		NotAfter:     time.Now().Add(365 * 24 * time.Hour),
	}
	certDER, err := x509.CreateCertificate(rand.Reader, tmpl, tmpl, &key.PublicKey, key)
	if err != nil {
		t.Fatalf("create cert: %v", err)
	}
	certPEM = string(pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: certDER}))
	keyDER, err := x509.MarshalPKCS8PrivateKey(key)
	if err != nil {
		t.Fatalf("marshal key: %v", err)
	}
	keyPEM = string(pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: keyDER}))
	return
}

func setupCertTest(t *testing.T) (*CertController, string) {
	t.Helper()
	dir := t.TempDir()
	ctrl := NewCertController(dir)
	return ctrl, dir
}

func TestCertPush_Success(t *testing.T) {
	ctrl, dir := setupCertTest(t)
	certPEM, keyPEM := generateTestCertPEM(t)

	body := mustMarshal(t, map[string]string{"certPEM": certPEM, "keyPEM": keyPEM})

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodPost, "/api/v1/certs", strings.NewReader(body))
	c.Request.Header.Set("Content-Type", "application/json")

	ctrl.Push(c)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	certPath := filepath.Join(dir, certFile)
	keyPath := filepath.Join(dir, keyFile)

	if _, err := os.Stat(certPath); os.IsNotExist(err) {
		t.Fatal("cert file was not written")
	}
	if _, err := os.Stat(keyPath); os.IsNotExist(err) {
		t.Fatal("key file was not written")
	}
}

func TestCertPush_InvalidCertPEM(t *testing.T) {
	ctrl, _ := setupCertTest(t)
	_, keyPEM := generateTestCertPEM(t)

	body := mustMarshal(t, map[string]string{"certPEM": "invalid", "keyPEM": keyPEM})

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodPost, "/api/v1/certs", strings.NewReader(body))
	c.Request.Header.Set("Content-Type", "application/json")

	ctrl.Push(c)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", w.Code, w.Body.String())
	}
}

func TestCertPush_InvalidKeyPEM(t *testing.T) {
	ctrl, _ := setupCertTest(t)
	certPEM, _ := generateTestCertPEM(t)

	body := mustMarshal(t, map[string]string{"certPEM": certPEM, "keyPEM": "invalid"})

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodPost, "/api/v1/certs", strings.NewReader(body))
	c.Request.Header.Set("Content-Type", "application/json")

	ctrl.Push(c)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", w.Code, w.Body.String())
	}
}

func TestCertPush_BadJSON(t *testing.T) {
	ctrl, _ := setupCertTest(t)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodPost, "/api/v1/certs", strings.NewReader(`{invalid`))
	c.Request.Header.Set("Content-Type", "application/json")

	ctrl.Push(c)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", w.Code, w.Body.String())
	}
}

func TestCertStatus_NoCertInstalled(t *testing.T) {
	ctrl, _ := setupCertTest(t)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodGet, "/api/v1/certs/status", nil)

	ctrl.Status(c)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
}

func TestCertStatus_WithCert(t *testing.T) {
	ctrl, dir := setupCertTest(t)
	certPEM, keyPEM := generateTestCertPEM(t)

	os.MkdirAll(dir, 0700)
	os.WriteFile(filepath.Join(dir, certFile), []byte(certPEM), 0600)
	os.WriteFile(filepath.Join(dir, keyFile), []byte(keyPEM), 0600)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodGet, "/api/v1/certs/status", nil)

	ctrl.Status(c)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	body := w.Body.String()
	if !strings.Contains(body, "test") {
		t.Fatalf("response should contain subject 'test': %s", body)
	}
	if !strings.Contains(body, "serial") {
		t.Fatalf("response should contain serial: %s", body)
	}
}

func TestCertStatus_CorruptedCertFile(t *testing.T) {
	ctrl, dir := setupCertTest(t)

	os.MkdirAll(dir, 0700)
	os.WriteFile(filepath.Join(dir, certFile), []byte("not a cert"), 0600)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodGet, "/api/v1/certs/status", nil)

	ctrl.Status(c)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
}
