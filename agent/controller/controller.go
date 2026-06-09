package controller

import (
	"strconv"
	"strings"
	"time"

	"github.com/drunkleen/l-ui/internal/nodeauth"
	"github.com/gofiber/fiber/v3"
)

func abortJSONError(c fiber.Ctx, code int, msg string) error {
	return c.Status(code).JSON(fiber.Map{"error": msg})
}

func AuthMiddleware(secret string) fiber.Handler {
	return func(c fiber.Ctx) error {
		if secret == "" {
			return abortJSONError(c, fiber.StatusUnauthorized, "node not registered")
		}

		auth := c.Get(nodeauth.HeaderAuth)
		if !strings.HasPrefix(auth, "Bearer ") {
			return abortJSONError(c, fiber.StatusUnauthorized, "missing or invalid authorization header")
		}
		token := strings.TrimSpace(strings.TrimPrefix(auth, "Bearer "))
		if token == "" {
			return abortJSONError(c, fiber.StatusUnauthorized, "empty token")
		}

		if token == secret {
			c.Locals("authed", true)
			return c.Next()
		}

		method := c.Method()
		path := c.Path()
		body := c.Body()

		timestampStr := c.Get(nodeauth.HeaderTimestamp)
		nonce := c.Get(nodeauth.HeaderNonce)
		signature := c.Get(nodeauth.HeaderSignature)

		if timestampStr == "" || nonce == "" || signature == "" {
			return abortJSONError(c, fiber.StatusUnauthorized, "missing auth headers")
		}

		tsInt, err := parseInt64(timestampStr)
		if err != nil {
			return abortJSONError(c, fiber.StatusUnauthorized, "invalid timestamp")
		}

		if !nodeauth.Verify(secret, method, path, body, tsInt, nonce, signature, time.Now(), 5*time.Minute) {
			return abortJSONError(c, fiber.StatusUnauthorized, "invalid signature")
		}

		c.Locals("authed", true)
		return c.Next()
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
