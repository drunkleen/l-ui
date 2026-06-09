package controller

import (
	"github.com/drunkleen/l-ui/internal/logger"
	"github.com/drunkleen/l-ui/hub/web/locale"
	"github.com/drunkleen/l-ui/hub/web/session"

	"github.com/gofiber/fiber/v3"
)

// BaseController provides common functionality for all controllers.
type BaseController struct{}

func (a *BaseController) checkLogin(c fiber.Ctx) error {
	if !session.IsLogin(c) {
		if isAjax(c) {
			pureJsonMsg(c, fiber.StatusUnauthorized, false, I18nWeb(c, "pages.login.loginAgain"))
		} else {
			c.Set("Cache-Control", "no-store")
			basePath, _ := c.Locals("base_path").(string)
			return c.Redirect().Status(fiber.StatusTemporaryRedirect).To(basePath)
		}
		return nil
	}
	return c.Next()
}

func I18nWeb(c fiber.Ctx, name string, params ...string) string {
	anyfunc := c.Locals("I18n")
	if anyfunc == nil {
		logger.Warning("I18n function not exists in fiber context!")
		return ""
	}
	i18nFunc, ok := anyfunc.(func(i18nType locale.I18nType, key string, keyParams ...string) string)
	if !ok {
		logger.Warning("I18n function type assertion failed!")
		return ""
	}
	msg := i18nFunc(locale.Web, name, params...)
	return msg
}
