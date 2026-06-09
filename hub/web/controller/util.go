package controller

import (
	"fmt"
	"net"
	"net/netip"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/drunkleen/l-ui/internal/logger"
	"github.com/drunkleen/l-ui/internal/util/common"
	"github.com/drunkleen/l-ui/hub/web/entity"
	"github.com/drunkleen/l-ui/hub/web/service"

	"github.com/gofiber/fiber/v3"
)

func getRemoteIp(c fiber.Ctx) string {
	remoteIP, ok := extractTrustedIP(c.IP())
	if !ok {
		return "unknown"
	}

	if isTrustedProxy(remoteIP) {
		if ip, ok := extractTrustedIP(c.Get("X-Real-IP")); ok {
			return ip
		}

		if xff := c.Get("X-Forwarded-For"); xff != "" {
			for part := range strings.SplitSeq(xff, ",") {
				if ip, ok := extractTrustedIP(part); ok {
					return ip
				}
			}
		}
	}

	return remoteIP
}

func isTrustedForwardedRequest(c fiber.Ctx) bool {
	remoteIP, ok := extractTrustedIP(c.IP())
	return ok && isTrustedProxy(remoteIP)
}

func isTrustedProxy(ip string) bool {
	addr, err := netip.ParseAddr(ip)
	if err != nil {
		return false
	}

	trusted := trustedProxyCIDRs()
	for value := range strings.SplitSeq(trusted, ",") {
		value = strings.TrimSpace(value)
		if value == "" {
			continue
		}
		if prefix, err := netip.ParsePrefix(value); err == nil {
			if prefix.Contains(addr) {
				return true
			}
			continue
		}
		if proxyIP, err := netip.ParseAddr(value); err == nil && proxyIP.Unmap() == addr.Unmap() {
			return true
		}
	}
	return false
}

func trustedProxyCIDRs() (trusted string) {
	trusted = "127.0.0.1/32,::1/128"
	defer func() {
		_ = recover()
	}()
	settingService := service.SettingService{}
	if value, err := settingService.GetTrustedProxyCIDRs(); err == nil && strings.TrimSpace(value) != "" {
		trusted = value
	}
	return trusted
}

func extractTrustedIP(value string) (string, bool) {
	candidate := strings.TrimSpace(value)
	if candidate == "" {
		return "", false
	}

	if ip, ok := parseIPCandidate(candidate); ok {
		return ip.String(), true
	}

	if host, _, err := net.SplitHostPort(candidate); err == nil {
		if ip, ok := parseIPCandidate(host); ok {
			return ip.String(), true
		}
	}

	if strings.Count(candidate, ":") == 1 {
		if host, _, err := net.SplitHostPort(fmt.Sprintf("[%s]", candidate)); err == nil {
			if ip, ok := parseIPCandidate(host); ok {
				return ip.String(), true
			}
		}
	}

	return "", false
}

func parseIPCandidate(value string) (netip.Addr, bool) {
	ip, err := netip.ParseAddr(strings.TrimSpace(value))
	if err != nil {
		return netip.Addr{}, false
	}
	return ip.Unmap(), true
}

func jsonMsg(c fiber.Ctx, msg string, err error) {
	jsonMsgObj(c, msg, nil, err)
}

func jsonObj(c fiber.Ctx, obj any, err error) {
	jsonMsgObj(c, "", obj, err)
}

func requestErrorContext(c fiber.Ctx) string {
	handler, loc := callerOutsideUtil()
	return fmt.Sprintf("[%s %s handler=%s %s]", c.Method(), c.Path(), handler, loc)
}

func callerOutsideUtil() (string, string) {
	var pcs [12]uintptr
	n := runtime.Callers(2, pcs[:])
	frames := runtime.CallersFrames(pcs[:n])
	for {
		frame, more := frames.Next()
		base := filepath.Base(frame.File)
		if base != "util.go" {
			name := frame.Function
			if idx := strings.LastIndex(name, "/"); idx >= 0 {
				name = name[idx+1:]
			}
			return name, fmt.Sprintf("%s:%d", base, frame.Line)
		}
		if !more {
			break
		}
	}
	return "unknown", "unknown"
}

func jsonMsgObj(c fiber.Ctx, msg string, obj any, err error) {
	m := entity.Msg{
		Obj: obj,
	}
	if err == nil {
		m.Success = true
		if msg != "" {
			m.Msg = msg
		}
	} else {
		m.Success = false
		m.Code = common.ErrorCode(err)
		ctx := requestErrorContext(c)
		fail := I18nWeb(c, "fail")
		errStr := err.Error()
		if errStr != "" {
			m.Msg = msg + " (" + errStr + ")"
			logger.Warningf("%s %s %s: %v", ctx, msg, fail, err)
		} else if msg != "" {
			m.Msg = msg
			logger.Warningf("%s %s %s", ctx, msg, fail)
		} else {
			m.Msg = I18nWeb(c, "somethingWentWrong")
			logger.Warningf("%s %s %s", ctx, m.Msg, fail)
		}
	}
	c.Status(fiber.StatusOK).JSON(m)
}

func pureJsonMsg(c fiber.Ctx, statusCode int, success bool, msg string) {
	c.Status(statusCode).JSON(entity.Msg{
		Success: success,
		Msg:     msg,
	})
}

func isAjax(c fiber.Ctx) bool {
	return c.Get("X-Requested-With") == "XMLHttpRequest"
}
