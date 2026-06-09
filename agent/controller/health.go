package controller

import (
	"github.com/gofiber/fiber/v3"
)

// Healthz returns 200 if the agent process is running.
func Healthz(c fiber.Ctx) error {
	return c.Status(fiber.StatusOK).JSON(fiber.Map{"status": "ok"})
}

// Readyz returns 200 since the agent has no database dependency.
// The agent is considered ready as long as it is running.
func Readyz(c fiber.Ctx) error {
	return c.Status(fiber.StatusOK).JSON(fiber.Map{"status": "ok"})
}
