package controller

import (
	"github.com/drunkleen/l-ui/internal/database"

	"github.com/gofiber/fiber/v3"
)

func Healthz(c fiber.Ctx) error {
	return c.Status(fiber.StatusOK).JSON(fiber.Map{"status": "ok"})
}

func Readyz(c fiber.Ctx) error {
	db := database.GetDB()
	if db == nil {
		return c.Status(fiber.StatusServiceUnavailable).JSON(fiber.Map{"status": "error", "msg": "database not initialized"})
	}
	sqlDB, err := db.DB()
	if err != nil {
		return c.Status(fiber.StatusServiceUnavailable).JSON(fiber.Map{"status": "error", "msg": "failed to get database connection"})
	}
	if err := sqlDB.Ping(); err != nil {
		return c.Status(fiber.StatusServiceUnavailable).JSON(fiber.Map{"status": "error", "msg": "database ping failed"})
	}
	return c.Status(fiber.StatusOK).JSON(fiber.Map{"status": "ok"})
}
