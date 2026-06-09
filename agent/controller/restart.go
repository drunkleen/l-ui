package controller

import (
	"os"
	"os/exec"
	"syscall"

	"github.com/gofiber/fiber/v3"
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

func (s *RestartController) RestartAgent(c fiber.Ctx) error {
	_ = c.Status(fiber.StatusOK).JSON(fiber.Map{"success": true, "msg": "restarting"})
	go restartAgentFn()
	return nil
}

type restartXrayResponse struct {
	Success bool   `json:"success"`
	Status  string `json:"status"`
	Error   string `json:"error,omitempty"`
}

func (s *RestartController) RestartXray(c fiber.Ctx) error {
	if err := restartXrayFn(); err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(restartXrayResponse{
			Success: false,
			Status:  "error",
			Error:   err.Error(),
		})
	}
	return c.Status(fiber.StatusOK).JSON(restartXrayResponse{Success: true, Status: "ok"})
}
