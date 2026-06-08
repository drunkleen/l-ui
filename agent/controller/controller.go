package controller

import (
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/drunkleen/l-ui/internal/nodeauth"
	"github.com/gin-gonic/gin"
)

func abortJSONError(c *gin.Context, code int, msg string) {
	c.JSON(code, gin.H{"error": msg})
	c.Abort()
}

func AuthMiddleware(secret string) gin.HandlerFunc {
	return func(c *gin.Context) {
		if secret == "" {
			abortJSONError(c, http.StatusUnauthorized, "node not registered")
			return
		}

		auth := c.GetHeader(nodeauth.HeaderAuth)
		if !strings.HasPrefix(auth, "Bearer ") {
			abortJSONError(c, http.StatusUnauthorized, "missing or invalid authorization header")
			return
		}
		token := strings.TrimSpace(strings.TrimPrefix(auth, "Bearer "))
		if token == "" {
			abortJSONError(c, http.StatusUnauthorized, "empty token")
			return
		}

		if token == secret {
			c.Set("authed", true)
			c.Next()
			return
		}

		method := c.Request.Method
		path := c.Request.URL.Path
		body, _ := c.GetRawData()

		timestampStr := c.GetHeader(nodeauth.HeaderTimestamp)
		nonce := c.GetHeader(nodeauth.HeaderNonce)
		signature := c.GetHeader(nodeauth.HeaderSignature)

		if timestampStr == "" || nonce == "" || signature == "" {
			abortJSONError(c, http.StatusUnauthorized, "missing auth headers")
			return
		}

		tsInt, err := parseInt64(timestampStr)
		if err != nil {
			abortJSONError(c, http.StatusUnauthorized, "invalid timestamp")
			return
		}

		if !nodeauth.Verify(secret, method, path, body, tsInt, nonce, signature, time.Now(), 5*time.Minute) {
			abortJSONError(c, http.StatusUnauthorized, "invalid signature")
			return
		}

		c.Set("authed", true)
		c.Next()
	}
}

func parseInt64(s string) (int64, error) {
	return strconv.ParseInt(strings.TrimSpace(s), 10, 64)
}

type StatusController struct{}
type MetricsController struct{}
type SysInfoController struct{}
type ConfigController struct{}
type FirewallController struct{}
type LogsController struct{}
type RestartController struct{}
