package controller

import (
	"net"
	"net/url"
	"strings"

	"github.com/drunkleen/l-ui/internal/logger"
	"github.com/drunkleen/l-ui/hub/web/service"
	"github.com/drunkleen/l-ui/hub/web/session"

	"github.com/gofiber/fiber/v3"
	ws "github.com/fasthttp/websocket"
	"github.com/valyala/fasthttp"
)

var upgrader = ws.FastHTTPUpgrader{
	ReadBufferSize:    32768,
	WriteBufferSize:   32768,
	EnableCompression: true,
	CheckOrigin:       checkSameOrigin,
}

// checkSameOrigin allows requests with no Origin header (same-origin or non-browser
// clients) and otherwise requires the Origin hostname to match the request hostname.
// Comparison is case-insensitive (RFC 7230 §2.7.3) and ignores port differences
// (the panel often sits behind a reverse proxy on a different port).
func checkSameOrigin(ctx *fasthttp.RequestCtx) bool {
	origin := string(ctx.Request.Header.Peek("Origin"))
	if origin == "" {
		return true
	}
	u, err := url.Parse(origin)
	if err != nil || u.Hostname() == "" {
		return false
	}
	host := string(ctx.Host())
	hostname, _, err := net.SplitHostPort(host)
	if err != nil {
		// IPv6 literals without a port arrive as "[::1]"; net.SplitHostPort
		// fails in that case while url.Hostname() returns the address without
		// brackets. Strip them so same-origin checks pass for bare IPv6 hosts.
		hostname = host
		if len(hostname) >= 2 && hostname[0] == '[' && hostname[len(hostname)-1] == ']' {
			hostname = hostname[1 : len(hostname)-1]
		}
	}
	return strings.EqualFold(u.Hostname(), hostname)
}

// WebSocketController handles the HTTP→WebSocket upgrade for real-time updates.
// All per-connection lifecycle (pumps, hub registration) lives in
// service.WebSocketService — this controller is HTTP-layer only.
type WebSocketController struct {
	BaseController
	service *service.WebSocketService
}

// NewWebSocketController creates a controller wired to the given service.
func NewWebSocketController(svc *service.WebSocketService) *WebSocketController {
	return &WebSocketController{service: svc}
}

// HandleWebSocket authenticates the request, upgrades the HTTP connection, and
// hands ownership of the connection off to the service.
func (w *WebSocketController) HandleWebSocket(c fiber.Ctx) error {
	if !session.IsLogin(c) {
		logger.Warningf("Unauthorized WebSocket connection attempt from %s", getRemoteIp(c))
		return c.SendStatus(fiber.StatusUnauthorized)
	}

	err := upgrader.Upgrade(c.RequestCtx(), func(conn *ws.Conn) {
		w.service.HandleConnection(conn, getRemoteIp(c))
	})
	if err != nil {
		logger.Error("Failed to upgrade WebSocket connection:", err)
		return nil
	}
	return nil
}
