package middleware

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
)

// RedirectMiddleware returns a Gin middleware that handles URL redirections.
// It provides backward compatibility by redirecting old '/lui' paths to new '/panel' paths,
// including API endpoints. The middleware performs permanent redirects (301) for SEO purposes.
func RedirectMiddleware(basePath string) gin.HandlerFunc {
	return func(c *gin.Context) {
		// Redirect from old '/lui' path to '/panel'
		redirects := map[string]string{
			"panel/API": "panel/api",
			"lui/API":   "panel/api",
			"lui":       "panel",
		}

		path := c.Request.URL.Path
		for from, to := range redirects {
			from, to = basePath+from, basePath+to

			if strings.HasPrefix(path, from) {
				newPath := to + path[len(from):]

				c.Redirect(http.StatusMovedPermanently, newPath)
				c.Abort()
				return
			}
		}

		c.Next()
	}
}
