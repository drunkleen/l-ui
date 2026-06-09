package controller

import (
	"github.com/gofiber/fiber/v3"
)

func (s *SysInfoController) GetSysInfo(c fiber.Ctx) error {
	info, err := sysSvc.GetInfo()
	if err != nil {
		return abortJSONError(c, fiber.StatusInternalServerError, err.Error())
	}
	return c.Status(fiber.StatusOK).JSON(info)
}
