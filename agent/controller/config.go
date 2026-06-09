package controller

import (
	"encoding/json"

	"github.com/drunkleen/l-ui/agent/service"
	"github.com/gofiber/fiber/v3"
)

var cfgSvc = service.NewConfigService()

func (s *ConfigController) GetConfig(c fiber.Ctx) error {
	cfg, err := cfgSvc.GetConfig()
	if err != nil {
		return abortJSONError(c, fiber.StatusInternalServerError, err.Error())
	}
	return c.Status(fiber.StatusOK).JSON(fiber.Map{"success": true, "obj": cfg})
}

type pushConfigRequest struct {
	HubNodeID   string          `json:"hub_node_id"`
	HubEndpoint string          `json:"hub_endpoint"`
	XrayConfig  json.RawMessage `json:"xray_config"`
	ClientList  json.RawMessage `json:"client_list"`
}

func (s *ConfigController) PushConfig(c fiber.Ctx) error {
	var req pushConfigRequest
	if err := c.Bind().JSON(&req); err != nil {
		return abortJSONError(c, fiber.StatusBadRequest, "invalid request body")
	}
	if err := cfgSvc.PushConfig(req.HubNodeID, req.HubEndpoint, req.XrayConfig, req.ClientList); err != nil {
		return abortJSONError(c, fiber.StatusInternalServerError, err.Error())
	}
	return c.Status(fiber.StatusOK).JSON(fiber.Map{
		"success": true,
		"obj": fiber.Map{
			"config_version": cfgSvc.GetConfigVersion(),
		},
	})
}

func (s *ConfigController) ApplyConfig(c fiber.Ctx) error {
	return c.Status(fiber.StatusOK).JSON(fiber.Map{"success": true, "msg": "config applied"})
}
