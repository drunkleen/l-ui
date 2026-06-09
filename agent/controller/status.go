package controller

import (
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/drunkleen/l-ui/internal/config"
	"github.com/gofiber/fiber/v3"
)

func getXrayVersion() string {
	// Try multiple xray binary locations
	candidates := []string{"xray"}
	if binDir := config.GetBinFolderPath(); binDir != "" {
		candidates = append(candidates, filepath.Join(binDir, "xray"))
	}
	candidates = append(candidates,
		"/usr/local/bin/xray",
		"/usr/local/l-ui/bin/xray",
		"/usr/bin/xray",
	)

	for _, path := range candidates {
		out, err := exec.Command(path, "--version").Output()
		if err != nil {
			continue
		}
		line := strings.TrimSpace(string(out))
		if idx := strings.Index(line, " "); idx > 0 {
			return line[:idx]
		}
		return line
	}
	return ""
}

func (s *StatusController) GetStatus(c fiber.Ctx) error {
	metrics, err := sysSvc.GetMetrics()
	if err != nil {
		return c.Status(fiber.StatusOK).JSON(fiber.Map{
			"success": false,
			"msg":     err.Error(),
		})
	}

	var cpuPct float64
	var memCurrent, memTotal uint64
	var diskCurrent, diskTotal uint64
	var netUp, netDown uint64
	var uptime uint64

	if metrics != nil {
		cpuPct = metrics.CPUPercent
		memCurrent = metrics.MemoryUsed
		memTotal = metrics.MemoryTotal
		diskCurrent = metrics.DiskUsed
		diskTotal = metrics.DiskTotal
		netUp = metrics.NetSent
		netDown = metrics.NetRecv
		uptime = metrics.Uptime
	}

	return c.Status(fiber.StatusOK).JSON(fiber.Map{
		"success": true,
		"obj": fiber.Map{
			"cpu": cpuPct,
			"mem": fiber.Map{
				"current": memCurrent,
				"total":   memTotal,
			},
			"disk": fiber.Map{
				"current": diskCurrent,
				"total":   diskTotal,
			},
			"netIO": fiber.Map{
				"up":   netUp,
				"down": netDown,
			},
			"xray": fiber.Map{
				"version": getXrayVersion(),
			},
			"panelVersion": config.GetVersion(),
			"uptime":       uptime,
		},
	})
}
