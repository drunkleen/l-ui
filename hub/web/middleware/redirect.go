package middleware

import (
	"strings"

	"github.com/gofiber/fiber/v3"
)

// RedirectMiddleware returns a middleware that handles URL redirections.
func RedirectMiddleware(basePath string) fiber.Handler {
	return func(c fiber.Ctx) error {
		redirects := map[string]string{
			"panel/API": "panel/api",
			"lui/API":   "panel/api",
			"lui":       "panel",
		}

		path := c.Path()
		for from, to := range redirects {
			from, to = basePath+from, basePath+to

			if strings.HasPrefix(path, from) {
				newPath := to + path[len(from):]
				return c.Redirect().Status(fiber.StatusMovedPermanently).To(newPath)
			}
		}

		return c.Next()
	}
}
