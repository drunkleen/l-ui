package controller

import (
	"net"
	"testing"

	"github.com/gofiber/fiber/v3"
	"github.com/valyala/fasthttp"
)

func TestGetRemoteIpIgnoresForwardedHeadersFromUntrustedRemote(t *testing.T) {
	app := fiber.New()
	rawCtx := &fasthttp.RequestCtx{}
	rawCtx.SetRemoteAddr(&net.TCPAddr{IP: net.ParseIP("203.0.113.10"), Port: 12345})
	c := app.AcquireCtx(rawCtx)
	defer app.ReleaseCtx(c)

	c.Request().Header.SetMethod("GET")
	c.Request().SetRequestURI("/")
	c.Request().Header.Set("X-Real-IP", "198.51.100.9")
	c.Request().Header.Set("X-Forwarded-For", "198.51.100.8")

	if got := getRemoteIp(c); got != "203.0.113.10" {
		t.Fatalf("remote IP = %q, want request remote address", got)
	}
}

func TestGetRemoteIpHonorsForwardedHeadersFromTrustedLoopbackProxy(t *testing.T) {
	app := fiber.New()
	rawCtx := &fasthttp.RequestCtx{}
	rawCtx.SetRemoteAddr(&net.TCPAddr{IP: net.ParseIP("127.0.0.1"), Port: 12345})
	c := app.AcquireCtx(rawCtx)
	defer app.ReleaseCtx(c)

	c.Request().Header.SetMethod("GET")
	c.Request().SetRequestURI("/")
	c.Request().Header.Set("X-Forwarded-For", "198.51.100.8, 127.0.0.1")

	if got := getRemoteIp(c); got != "198.51.100.8" {
		t.Fatalf("remote IP = %q, want forwarded client IP", got)
	}
}
