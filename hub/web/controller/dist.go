package controller

import (
	"bytes"
	"embed"
	htmlpkg "html"
	"strings"
	"time"

	"github.com/drunkleen/l-ui/hub/web/global"
	"github.com/drunkleen/l-ui/hub/web/session"
	"github.com/drunkleen/l-ui/internal/config"
	"github.com/drunkleen/l-ui/internal/logger"

	"github.com/gofiber/fiber/v3"
)

var distFS embed.FS

func SetDistFS(fs embed.FS) {
	distFS = fs
}

var distPageBuildTime = time.Now()

func ServeOpenAPISpec(c fiber.Ctx) error {
	body, err := distFS.ReadFile("dist/openapi.json")
	if err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"success": false, "msg": "openapi.json not found"})
	}
	c.Set("Content-Type", "application/json")
	c.Set("Cache-Control", "public, max-age=300")
	return c.Status(fiber.StatusOK).Send(body)
}

func serveDistPage(c fiber.Ctx, name string) error {
	body, err := distFS.ReadFile("dist/" + name)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).SendString("missing embedded page: " + name)
	}

	basePath, _ := c.Locals("base_path").(string)
	if basePath == "" {
		basePath = "/"
	}

	if basePath != "/" {
		body = bytes.ReplaceAll(body, []byte(`src="/assets/`), []byte(`src="`+basePath+`assets/`))
		body = bytes.ReplaceAll(body, []byte(`href="/assets/`), []byte(`href="`+basePath+`assets/`))
	}

	jsEscape := strings.NewReplacer(
		`\`, `\\`,
		`"`, `\"`,
		"\n", `\n`,
		"\r", `\r`,
		"<", `\u003C`,
		">", `\u003E`,
		"&", `\u0026`,
	)
	escapedBase := jsEscape.Replace(basePath)
	mode := "hub"
	apiPrefix := "/panel/api"
	if server := global.GetWebServer(); server != nil {
		mode = server.ModeString()
		if mode == "agent" {
			apiPrefix = "/api/v1"
		}
	}
	csrfToken, err := session.EnsureCSRFToken(c)
	if err != nil {
		logger.Warning("Unable to mint CSRF token for", name+":", err)
		csrfToken = ""
	}
	csrfMeta := []byte(`<meta name="csrf-token" content="` + htmlpkg.EscapeString(csrfToken) + `">`)
	basePathMeta := []byte(`<meta name="base-path" content="` + htmlpkg.EscapeString(basePath) + `">`)

	nonceAttr := ""
	if nonce, ok := c.Locals("csp_nonce").(string); ok && nonce != "" {
		nonceAttr = ` nonce="` + htmlpkg.EscapeString(nonce) + `"`
	}
	script := `<script` + nonceAttr + `>window.L_UI_BASE_PATH="` + escapedBase + `"`
	script += `;window.L_UI_MODE="` + jsEscape.Replace(mode) + `"`
	script += `;window.L_UI_API_PREFIX="` + jsEscape.Replace(apiPrefix) + `"`
	if name != "login.html" {
		escapedVer := jsEscape.Replace(config.GetVersion())
		script += `;window.L_UI_CUR_VER="` + escapedVer + `"`
		script += `;window.L_UI_DB_TYPE="` + config.GetDBKind() + `"`
	}
	script += `;</script>`
	inject := []byte(script)
	inject = append(inject, csrfMeta...)
	inject = append(inject, basePathMeta...)
	inject = append(inject, []byte(`</head>`)...)
	out := bytes.Replace(body, []byte("</head>"), inject, 1)

	c.Set("Cache-Control", "no-cache, no-store, must-revalidate")
	c.Set("Pragma", "no-cache")
	c.Set("Expires", "0")
	c.Set("Last-Modified", distPageBuildTime.UTC().Format("Mon, 02 Jan 2006 15:04:05 GMT"))
	c.Set("Content-Type", "text/html; charset=utf-8")
	return c.Status(fiber.StatusOK).Send(out)
}
