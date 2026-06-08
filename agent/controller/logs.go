package controller

import (
	"net/http"
	"strconv"
	"strings"

	"github.com/drunkleen/l-ui/agent/service"
	"github.com/gin-gonic/gin"
)

var logSvc = service.NewLogService()

func (s *LogsController) TailLog(c *gin.Context) {
	logPath := c.Query("path")
	if logPath == "" {
		abortJSONError(c, http.StatusBadRequest, "path query parameter is required")
		return
	}

	linesStr := c.DefaultQuery("lines", "50")
	tailLines, err := strconv.Atoi(linesStr)
	if err != nil || tailLines <= 0 {
		tailLines = 50
	}
	if tailLines > 500 {
		tailLines = 500
	}

	lines, err := logSvc.TailLog(logPath, tailLines)
	if err != nil {
		abortJSONError(c, http.StatusInternalServerError, "failed to read log: "+err.Error())
		return
	}
	if lines == nil {
		lines = []string{}
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"obj":     strings.Join(lines, "\n"),
	})
}
