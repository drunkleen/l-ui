package controller

import (
	"text/template"
	"time"

	"github.com/drunkleen/l-ui/internal/logger"
	"github.com/drunkleen/l-ui/hub/web/middleware"
	"github.com/drunkleen/l-ui/hub/web/service"
	"github.com/drunkleen/l-ui/hub/web/session"

	"github.com/gofiber/fiber/v3"
)

type LoginForm struct {
	Username      string `json:"username" form:"username"`
	Password      string `json:"password" form:"password"`
	TwoFactorCode string `json:"twoFactorCode" form:"twoFactorCode"`
}

type IndexController struct {
	BaseController

	settingService service.SettingService
	userService    service.UserService
	tgbot          service.Tgbot
}

func NewIndexController(router fiber.Router) *IndexController {
	a := &IndexController{}
	a.initRouter(router)
	return a
}

func (a *IndexController) initRouter(router fiber.Router) {
	router.Get("/", a.index)
	router.Get("/csrf-token", a.csrfToken)

	router.Post("/login", middleware.CSRFMiddleware(), a.login)
	router.Post("/logout", middleware.CSRFMiddleware(), a.logout)
	router.Post("/getTwoFactorEnable", middleware.CSRFMiddleware(), a.getTwoFactorEnable)
}

func (a *IndexController) index(c fiber.Ctx) error {
	if session.IsLogin(c) {
		c.Set("Cache-Control", "no-store")
		basePath, _ := c.Locals("base_path").(string)
		return c.Redirect().Status(fiber.StatusTemporaryRedirect).To(basePath + "panel/")
	}
	return serveDistPage(c, "login.html")
}

func (a *IndexController) login(c fiber.Ctx) error {
	var form LoginForm

	if err := c.Bind().Body(&form); err != nil {
		pureJsonMsg(c, fiber.StatusOK, false, I18nWeb(c, "pages.login.toasts.invalidFormData"))
		return nil
	}
	if form.Username == "" {
		pureJsonMsg(c, fiber.StatusOK, false, I18nWeb(c, "pages.login.toasts.emptyUsername"))
		return nil
	}
	if form.Password == "" {
		pureJsonMsg(c, fiber.StatusOK, false, I18nWeb(c, "pages.login.toasts.emptyPassword"))
		return nil
	}

	remoteIP := getRemoteIp(c)
	safeUser := template.HTMLEscapeString(form.Username)
	timeStr := time.Now().Format("2006-01-02 15:04:05")
	if blockedUntil, ok := defaultLoginLimiter.allow(remoteIP, form.Username); !ok {
		reason := "too many failed attempts"
		logger.Warningf("failed login: username=%q, IP=%q, reason=%q, blocked_until=%s", safeUser, remoteIP, reason, blockedUntil.Format(time.RFC3339))
		a.tgbot.UserLoginNotify(service.LoginAttempt{
			Username: safeUser,
			IP:       remoteIP,
			Time:     timeStr,
			Status:   service.LoginFail,
			Reason:   reason,
		})
		pureJsonMsg(c, fiber.StatusOK, false, I18nWeb(c, "pages.login.toasts.wrongUsernameOrPassword"))
		return nil
	}

	user, checkErr := a.userService.CheckUser(form.Username, form.Password, form.TwoFactorCode)

	if user == nil {
		reason := loginFailureReason(checkErr)
		if blockedUntil, blocked := defaultLoginLimiter.registerFailure(remoteIP, form.Username); blocked {
			logger.Warningf("failed login: username=%q, IP=%q, reason=%q, blocked_until=%s", safeUser, remoteIP, reason, blockedUntil.Format(time.RFC3339))
		} else {
			logger.Warningf("failed login: username=%q, IP=%q, reason=%q", safeUser, remoteIP, reason)
		}
		a.tgbot.UserLoginNotify(service.LoginAttempt{
			Username: safeUser,
			IP:       remoteIP,
			Time:     timeStr,
			Status:   service.LoginFail,
			Reason:   reason,
		})
		pureJsonMsg(c, fiber.StatusOK, false, I18nWeb(c, "pages.login.toasts.wrongUsernameOrPassword"))
		return nil
	}

	defaultLoginLimiter.registerSuccess(remoteIP, form.Username)
	logger.Infof("%s logged in successfully, Ip Address: %s\n", safeUser, remoteIP)
	a.tgbot.UserLoginNotify(service.LoginAttempt{
		Username: safeUser,
		IP:       remoteIP,
		Time:     timeStr,
		Status:   service.LoginSuccess,
	})

	if err := session.SetLoginUser(c, user); err != nil {
		logger.Warning("Unable to save session:", err)
		return nil
	}

	logger.Infof("%s logged in successfully", safeUser)
	jsonMsg(c, I18nWeb(c, "pages.login.toasts.successLogin"), nil)
	return nil
}

func loginFailureReason(err error) string {
	if err != nil && err.Error() == "invalid 2fa code" {
		return "invalid 2FA code"
	}
	return "invalid credentials"
}

func (a *IndexController) logout(c fiber.Ctx) error {
	user := session.GetLoginUser(c)
	if user != nil {
		logger.Infof("%s logged out successfully", user.Username)
	}
	if err := session.ClearSession(c); err != nil {
		logger.Warning("Unable to clear session on logout:", err)
	}
	c.Set("Cache-Control", "no-store")
	return c.Status(fiber.StatusOK).JSON(fiber.Map{"success": true})
}

func (a *IndexController) csrfToken(c fiber.Ctx) error {
	token, err := session.EnsureCSRFToken(c)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"success": false, "msg": err.Error()})
	}
	return c.Status(fiber.StatusOK).JSON(fiber.Map{"success": true, "obj": token})
}

func (a *IndexController) getTwoFactorEnable(c fiber.Ctx) error {
	status, err := a.settingService.GetTwoFactorEnable()
	if err != nil {
		jsonMsg(c, "", err)
		return nil
	}
	jsonObj(c, status, nil)
	return nil
}
