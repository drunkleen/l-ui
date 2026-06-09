// Package middleware provides HTTP middleware functions for the l-ui web panel.
package middleware

import (
	"net"
	"strings"

	"github.com/gofiber/fiber/v3"
)

// DomainValidatorMiddleware returns a middleware that validates the request domain.
func DomainValidatorMiddleware(domain string) fiber.Handler {
	return func(c fiber.Ctx) error {
		host := c.Hostname()
		if colonIndex := strings.LastIndex(host, ":"); colonIndex != -1 {
			host, _, _ = net.SplitHostPort(c.Hostname())
		}
		if host != domain {
			return c.SendStatus(fiber.StatusForbidden)
		}
		return c.Next()
	}
}
