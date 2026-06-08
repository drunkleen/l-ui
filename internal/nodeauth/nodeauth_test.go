package nodeauth

import (
	"testing"
	"time"
)

func TestSignAndVerify(t *testing.T) {
	secret := "s3cr3t"
	body := []byte(`{"ok":true}`)
	ts := time.Now().Unix()
	nonce := "nonce-123"
	sig := Sign(secret, "POST", "/api/v1/inbounds/add", body, ts, nonce)
	if !Verify(secret, "POST", "/api/v1/inbounds/add", body, ts, nonce, sig, time.Now(), 5*time.Minute) {
		t.Fatal("expected signature to verify")
	}
	if Verify("wrong", "POST", "/api/v1/inbounds/add", body, ts, nonce, sig, time.Now(), 5*time.Minute) {
		t.Fatal("unexpected verification with wrong secret")
	}
	if Verify(secret, "POST", "/api/v1/inbounds/add", body, ts-4000, nonce, sig, time.Now(), 5*time.Minute) {
		t.Fatal("unexpected verification with expired timestamp")
	}
}
