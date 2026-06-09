package controller

import (
	"strconv"
	"testing"
	"time"

	"github.com/drunkleen/l-ui/internal/nodeauth"

	"github.com/gofiber/fiber/v3"
	"github.com/valyala/fasthttp"
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
	app := fiber.New()
	c := app.AcquireCtx(&fasthttp.RequestCtx{})
	defer app.ReleaseCtx(c)

	c.Request().Header.SetMethod("POST")
	c.Request().SetRequestURI("/api/v1/server/reinstall")
	c.Request().SetBody([]byte(`{"bundle":"x"}`))
	c.Request().Header.SetContentType("application/json")

	timestamp := time.Now().Unix()
	nonce := "nonce-2"
	secret := "secret-2"
	c.Request().Header.Set(nodeauth.HeaderTimestamp, strconv.FormatInt(timestamp, 10))
	c.Request().Header.Set(nodeauth.HeaderNonce, nonce)
	c.Request().Header.Set(nodeauth.HeaderSignature, nodeauth.Sign(secret, "POST", "/api/v1/server/reinstall", []byte(`{"bundle":"x"}`), timestamp, nonce))
	c.Request().Header.Set(nodeauth.HeaderBodyDigest, "deadbeef")

	if (&APIController{}).verifySignedAPIRequest(c, secret) {
		t.Fatal("expected digest mismatch to be rejected")
	}
}

func TestVerifySignedAPIRequestRejectsMissingBodyDigest(t *testing.T) {
	app := fiber.New()
	c := app.AcquireCtx(&fasthttp.RequestCtx{})
	defer app.ReleaseCtx(c)

	c.Request().Header.SetMethod("POST")
	c.Request().SetRequestURI("/api/v1/server/reinstall")
	c.Request().SetBody([]byte(`{"bundle":"x"}`))
	c.Request().Header.SetContentType("application/json")

	timestamp := time.Now().Unix()
	nonce := "nonce-3"
	secret := "secret-3"
	c.Request().Header.Set(nodeauth.HeaderTimestamp, strconv.FormatInt(timestamp, 10))
	c.Request().Header.Set(nodeauth.HeaderNonce, nonce)
	c.Request().Header.Set(nodeauth.HeaderSignature, nodeauth.Sign(secret, "POST", "/api/v1/server/reinstall", []byte(`{"bundle":"x"}`), timestamp, nonce))

	if (&APIController{}).verifySignedAPIRequest(c, secret) {
		t.Fatal("expected missing digest to be rejected")
	}
}
