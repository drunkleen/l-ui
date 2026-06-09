package controller

import (
	"strconv"

	"github.com/drunkleen/l-ui/agent/service"
	"github.com/gofiber/fiber/v3"
)

var fwSvc = service.NewFirewallService()

func (s *FirewallController) GetStatus(c fiber.Ctx) error {
	status, err := fwSvc.GetStatus()
	if err != nil {
		return abortJSONError(c, fiber.StatusInternalServerError, err.Error())
	}
	if status == nil {
		return abortJSONError(c, fiber.StatusInternalServerError, "unable to determine firewall status")
	}
	return c.Status(fiber.StatusOK).JSON(fiber.Map{"success": true, "obj": status})
}

func (s *FirewallController) GetRules(c fiber.Ctx) error {
	rules, err := fwSvc.GetRules()
	if err != nil {
		return abortJSONError(c, fiber.StatusInternalServerError, err.Error())
	}
	if rules == nil {
		rules = []service.FirewallRule{}
	}
	return c.Status(fiber.StatusOK).JSON(fiber.Map{"success": true, "obj": fiber.Map{"rules": rules}})
}

type addRuleRequest struct {
	Port     string `json:"port" binding:"required"`
	Protocol string `json:"protocol"`
	Action   string `json:"action" binding:"required,oneof=allow deny reject limit"`
	Comment  string `json:"comment"`
}

func (s *FirewallController) AddRule(c fiber.Ctx) error {
	var req addRuleRequest
	if err := c.Bind().JSON(&req); err != nil {
		return abortJSONError(c, fiber.StatusBadRequest, "invalid request: "+err.Error())
	}
	if req.Port == "" {
		return abortJSONError(c, fiber.StatusBadRequest, "port is required")
	}
	if req.Action != "allow" && req.Action != "deny" && req.Action != "reject" && req.Action != "limit" {
		return abortJSONError(c, fiber.StatusBadRequest, "action must be one of: allow, deny, reject, limit")
	}
	if req.Protocol != "" && req.Protocol != "tcp" && req.Protocol != "udp" {
		return abortJSONError(c, fiber.StatusBadRequest, "protocol must be tcp or udp")
	}
	portInt, err := strconv.Atoi(req.Port)
	if err != nil || portInt < 1 || portInt > 65535 {
		return abortJSONError(c, fiber.StatusBadRequest, "port must be a valid number between 1 and 65535")
	}

	if err := fwSvc.AddRule(req.Port, req.Protocol, req.Action, req.Comment); err != nil {
		return abortJSONError(c, fiber.StatusInternalServerError, "failed to add rule: "+err.Error())
	}
	return c.Status(fiber.StatusOK).JSON(fiber.Map{"success": true, "msg": "rule added"})
}

type deleteRuleRequest struct {
	RuleNumber string `json:"rule_number" binding:"required"`
}

func (s *FirewallController) DeleteRule(c fiber.Ctx) error {
	var req deleteRuleRequest
	if err := c.Bind().JSON(&req); err != nil {
		return abortJSONError(c, fiber.StatusBadRequest, "invalid request: "+err.Error())
	}
	if req.RuleNumber == "" {
		return abortJSONError(c, fiber.StatusBadRequest, "rule_number is required")
	}

	if err := fwSvc.DeleteRule(req.RuleNumber); err != nil {
		return abortJSONError(c, fiber.StatusInternalServerError, "failed to delete rule: "+err.Error())
	}
	return c.Status(fiber.StatusOK).JSON(fiber.Map{"success": true, "msg": "rule deleted"})
}

func (s *FirewallController) Enable(c fiber.Ctx) error {
	if err := fwSvc.Enable(); err != nil {
		return abortJSONError(c, fiber.StatusInternalServerError, "failed to enable ufw: "+err.Error())
	}
	return c.Status(fiber.StatusOK).JSON(fiber.Map{"success": true, "msg": "ufw enabled"})
}

func (s *FirewallController) Disable(c fiber.Ctx) error {
	if err := fwSvc.Disable(); err != nil {
		return abortJSONError(c, fiber.StatusInternalServerError, "failed to disable ufw: "+err.Error())
	}
	return c.Status(fiber.StatusOK).JSON(fiber.Map{"success": true, "msg": "ufw disabled"})
}
