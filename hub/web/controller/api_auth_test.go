package controller

import (
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/drunkleen/l-ui/internal/nodeauth"
	"github.com/gin-gonic/gin"
)

func TestAcceptReplayRejectsDuplicateNonce(t *testing.T) {
	secret := "secret-1"
	nonce := "nonce-1"
	ts := time.Now().Unix()
	if !acceptReplay(secret, nonce, ts) {
		t.Fatal("expected first nonce to be accepted")
	}
	if acceptReplay(secret, nonce, ts) {
		t.Fatal("expected duplicate nonce to be rejected")
	}
}

func TestAcceptReplayScopesBySecret(t *testing.T) {
	nonce := "nonce-shared"
	ts := time.Now().Unix()
	if !acceptReplay("secret-a", nonce, ts) {
		t.Fatal("expected first secret to be accepted")
	}
	if !acceptReplay("secret-b", nonce, ts) {
		t.Fatal("expected same nonce under different secret to be accepted")
	}
}

func TestVerifySignedAPIRequestRejectsBodyDigestMismatch(t *testing.T) {
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	req := httptest.NewRequest("POST", "/api/v1/server/reinstall", strings.NewReader(`{"bundle":"x"}`))
	c.Request = req
	timestamp := time.Now().Unix()
	nonce := "nonce-2"
	secret := "secret-2"
	req.Header.Set(nodeauth.HeaderTimestamp, strconv.FormatInt(timestamp, 10))
	req.Header.Set(nodeauth.HeaderNonce, nonce)
	req.Header.Set(nodeauth.HeaderSignature, nodeauth.Sign(secret, req.Method, req.URL.Path, []byte(`{"bundle":"x"}`), timestamp, nonce))
	req.Header.Set(nodeauth.HeaderBodyDigest, "deadbeef")

	if (&APIController{}).verifySignedAPIRequest(c, secret) {
		t.Fatal("expected digest mismatch to be rejected")
	}
}

func TestVerifySignedAPIRequestRejectsMissingBodyDigest(t *testing.T) {
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	req := httptest.NewRequest("POST", "/api/v1/server/reinstall", strings.NewReader(`{"bundle":"x"}`))
	c.Request = req
	timestamp := time.Now().Unix()
	nonce := "nonce-3"
	secret := "secret-3"
	req.Header.Set(nodeauth.HeaderTimestamp, strconv.FormatInt(timestamp, 10))
	req.Header.Set(nodeauth.HeaderNonce, nonce)
	req.Header.Set(nodeauth.HeaderSignature, nodeauth.Sign(secret, req.Method, req.URL.Path, []byte(`{"bundle":"x"}`), timestamp, nonce))

	if (&APIController{}).verifySignedAPIRequest(c, secret) {
		t.Fatal("expected missing digest to be rejected")
	}
}
