package controller

import (
	"github.com/drunkleen/l-ui/agent/service"
	"github.com/gofiber/fiber/v3"
)

var sysSvc = service.NewSystemService()

func (s *MetricsController) GetMetrics(c fiber.Ctx) error {
	metrics, err := sysSvc.GetMetrics()
	if err != nil {
		return abortJSONError(c, fiber.StatusInternalServerError, err.Error())
	}
	return c.Status(fiber.StatusOK).JSON(metrics)
}
