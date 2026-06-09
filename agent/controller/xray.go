package controller

import (
	"encoding/json"

	"github.com/drunkleen/l-ui/agent/service"

	"github.com/gofiber/fiber/v3"
)

type XrayController struct {
	svc *service.XrayService
}

func NewXrayController() *XrayController {
	return &XrayController{svc: &service.XrayService{}}
}

func (c *XrayController) GetVersion(ctx fiber.Ctx) error {
	version := c.svc.GetXrayVersion()
	return ctx.Status(fiber.StatusOK).JSON(fiber.Map{
		"success": true,
		"obj": fiber.Map{"version": version},
	})
}

type xrayStatusResponse struct {
	Success bool   `json:"success"`
	Obj     struct {
		Version string `json:"version"`
		Running bool   `json:"running"`
	} `json:"obj"`
}

func (c *XrayController) GetStatus(ctx fiber.Ctx) error {
	version := c.svc.GetXrayVersion()
	running := c.svc.IsXrayRunning()
	return ctx.Status(fiber.StatusOK).JSON(fiber.Map{
		"success": true,
		"obj": fiber.Map{
			"version": version,
			"running": running,
		},
	})
}

type installXrayRequest struct {
	Version string `json:"version"`
}

func (c *XrayController) Install(ctx fiber.Ctx) error {
	var req installXrayRequest
	if err := ctx.Bind().JSON(&req); err != nil {
		return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"success": false,
			"msg":     "invalid request body",
		})
	}
	if req.Version == "" {
		return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"success": false,
			"msg":     "version is required",
		})
	}
	if err := c.svc.InstallXray(req.Version); err != nil {
		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"success": false,
			"msg":     err.Error(),
		})
	}
	return ctx.Status(fiber.StatusOK).JSON(fiber.Map{
		"success": true,
		"msg":     "xray " + req.Version + " installed",
	})
}

type applyConfigRequest struct {
	XrayConfig json.RawMessage `json:"xray_config"`
}

func (c *XrayController) ApplyConfig(ctx fiber.Ctx) error {
	var req applyConfigRequest
	if err := ctx.Bind().JSON(&req); err != nil {
		return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"success": false,
			"msg":     "invalid request body",
		})
	}
	if len(req.XrayConfig) == 0 {
		return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"success": false,
			"msg":     "xray_config is required",
		})
	}
	if err := c.svc.ApplyConfig(req.XrayConfig); err != nil {
		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"success": false,
			"msg":     err.Error(),
		})
	}
	return ctx.Status(fiber.StatusOK).JSON(fiber.Map{
		"success": true,
		"msg":     "config applied and xray restarted",
	})
}
