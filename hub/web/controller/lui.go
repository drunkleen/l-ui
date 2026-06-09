package controller

import (
	"github.com/drunkleen/l-ui/hub/web/entity"
	"github.com/drunkleen/l-ui/hub/web/middleware"
	"github.com/drunkleen/l-ui/hub/web/session"

	"github.com/gofiber/fiber/v3"
)

type LUIController struct {
	BaseController

	settingController     *SettingController
	xraySettingController *XraySettingController
}

func NewLUIController(router fiber.Router) *LUIController {
	a := &LUIController{}
	a.initRouter(router)
	return a
}

func (a *LUIController) initRouter(router fiber.Router) {
	g := router.Group("/panel")
	g.Use(a.checkLogin)
	g.Use(middleware.CSRFMiddleware())

	g.Get("/", a.panelSPA)
	g.Get("/inbounds", a.panelSPA)
	g.Get("/clients", a.panelSPA)
	g.Get("/groups", a.panelSPA)
	g.Get("/nodes", a.panelSPA)
	g.Get("/settings", a.panelSPA)
	g.Get("/xray", a.panelSPA)
	g.Get("/api-docs", a.panelSPA)

	g.Get("/csrf-token", a.csrfToken)

	a.settingController = NewSettingController(g)
	a.xraySettingController = NewXraySettingController(g)
}

func (a *LUIController) panelSPA(c fiber.Ctx) error {
	return serveDistPage(c, "index.html")
}

func (a *LUIController) csrfToken(c fiber.Ctx) error {
	token, err := session.EnsureCSRFToken(c)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(entity.Msg{Success: false, Msg: err.Error()})
	}
	return c.Status(fiber.StatusOK).JSON(entity.Msg{Success: true, Obj: token})
}
