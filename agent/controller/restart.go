package controller

import (
	"net/http"
	"os"
	"os/exec"
	"syscall"

	"github.com/gin-gonic/gin"
)

var restartAgentFn = restartAgent
var restartXrayFn = restartXray

func restartAgent() {
	exec.Command("systemctl", "restart", "l-ui-agent").Start()
	p, err := os.FindProcess(os.Getpid())
	if err == nil {
		p.Signal(syscall.SIGHUP)
	}
}

func restartXray() error {
	return exec.Command("systemctl", "restart", "xray").Run()
}

func (s *RestartController) RestartAgent(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"success": true, "msg": "restarting"})
	go restartAgentFn()
}

type restartXrayResponse struct {
	Success bool   `json:"success"`
	Status  string `json:"status"`
	Error   string `json:"error,omitempty"`
}

func (s *RestartController) RestartXray(c *gin.Context) {
	if err := restartXrayFn(); err != nil {
		c.JSON(http.StatusInternalServerError, restartXrayResponse{
			Success: false,
			Status:  "error",
			Error:   err.Error(),
		})
		return
	}
	c.JSON(http.StatusOK, restartXrayResponse{Success: true, Status: "ok"})
}
