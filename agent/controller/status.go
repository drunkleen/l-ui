package controller

import (
	"net/http"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/drunkleen/l-ui/internal/config"
	"github.com/gin-gonic/gin"
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

func (s *StatusController) GetStatus(c *gin.Context) {
	metrics, err := sysSvc.GetMetrics()
	if err != nil {
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"msg":     err.Error(),
		})
		return
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

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"obj": gin.H{
			"cpu": cpuPct,
			"mem": gin.H{
				"current": memCurrent,
				"total":   memTotal,
			},
			"disk": gin.H{
				"current": diskCurrent,
				"total":   diskTotal,
			},
			"netIO": gin.H{
				"up":   netUp,
				"down": netDown,
			},
			"xray": gin.H{
				"version": getXrayVersion(),
			},
			"panelVersion": config.GetVersion(),
			"uptime":       uptime,
		},
	})
}
