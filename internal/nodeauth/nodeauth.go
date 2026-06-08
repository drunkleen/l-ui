package nodeauth

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strings"
	"time"
)

const (
	HeaderAuth       = "Authorization"
	HeaderTimestamp  = "X-LUI-Timestamp"
	HeaderNonce      = "X-LUI-Nonce"
	HeaderSignature  = "X-LUI-Signature"
	HeaderBodyDigest = "X-LUI-Body-SHA256"
)

func BodyDigest(body []byte) string {
	sum := sha256.Sum256(body)
	return hex.EncodeToString(sum[:])
}

func CanonicalString(method, path string, body []byte, timestamp int64, nonce string) string {
	return strings.Join([]string{
		strings.ToUpper(strings.TrimSpace(method)),
		strings.TrimSpace(path),
		BodyDigest(body),
		fmt.Sprintf("%d", timestamp),
		strings.TrimSpace(nonce),
	}, "\n")
}

func Sign(secret, method, path string, body []byte, timestamp int64, nonce string) string {
	mac := hmac.New(sha256.New, []byte(secret))
	_, _ = mac.Write([]byte(CanonicalString(method, path, body, timestamp, nonce)))
	return hex.EncodeToString(mac.Sum(nil))
}

func Verify(secret, method, path string, body []byte, timestamp int64, nonce, signature string, now time.Time, skew time.Duration) bool {
	if secret == "" || nonce == "" || signature == "" {
		return false
	}
	if timestamp <= 0 {
		return false
	}
	if skew <= 0 {
		skew = 5 * time.Minute
	}
	delta := now.Unix() - timestamp
	if delta < 0 {
		delta = -delta
	}
	if time.Duration(delta)*time.Second > skew {
		return false
	}
	expected := Sign(secret, method, path, body, timestamp, nonce)
	return hmac.Equal([]byte(expected), []byte(strings.TrimSpace(signature)))
}
