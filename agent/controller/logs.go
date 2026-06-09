package controller

import (
	"strconv"
	"strings"

	"github.com/drunkleen/l-ui/agent/service"
	"github.com/gofiber/fiber/v3"
)

var logSvc = service.NewLogService()

func (s *LogsController) TailLog(c fiber.Ctx) error {
	logPath := c.Query("path")
	if logPath == "" {
		return abortJSONError(c, fiber.StatusBadRequest, "path query parameter is required")
	}

	linesStr := c.Query("lines", "50")
	tailLines, err := strconv.Atoi(linesStr)
	if err != nil || tailLines <= 0 {
		tailLines = 50
	}
	if tailLines > 500 {
		tailLines = 500
	}

	lines, err := logSvc.TailLog(logPath, tailLines)
	if err != nil {
		return abortJSONError(c, fiber.StatusInternalServerError, "failed to read log: "+err.Error())
	}
	if lines == nil {
		lines = []string{}
	}

	return c.Status(fiber.StatusOK).JSON(fiber.Map{
		"success": true,
		"obj":     strings.Join(lines, "\n"),
	})
}
