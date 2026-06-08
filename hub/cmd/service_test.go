package cmd

import (
	"fmt"
	"os"
	"os/exec"
	"testing"

	"github.com/spf13/cobra"
)

func TestServiceCommandsExist(t *testing.T) {
	cmds := []struct {
		cmd  *cobra.Command
		name string
	}{
		{startCmd, "start"},
		{stopCmd, "stop"},
		{restartCmd, "restart"},
		{restartXrayCmd, "restart-xray"},
		{statusCmd, "status"},
		{enableCmd, "enable"},
		{disableCmd, "disable"},
		{logCmd, "log"},
	}

	for _, tc := range cmds {
		t.Run(tc.name, func(t *testing.T) {
			if tc.cmd == nil {
				t.Fatalf("%sCmd is nil", tc.name)
			}
			if tc.cmd.Use != tc.name {
				t.Errorf("Use = %q, want %q", tc.cmd.Use, tc.name)
			}
		})
	}
}

func TestDetectInitSystem(t *testing.T) {
	orig := detectInitSystemFunc
	defer func() { detectInitSystemFunc = orig }()

	t.Run("systemd detected", func(t *testing.T) {
		detectInitSystemFunc = func() InitSystem { return InitSystemSystemd }
		if got := DetectInitSystem(); got != InitSystemSystemd {
			t.Errorf("DetectInitSystem() = %v, want %v", got, InitSystemSystemd)
		}
	})

	t.Run("openrc detected", func(t *testing.T) {
		detectInitSystemFunc = func() InitSystem { return InitSystemOpenRC }
		if got := DetectInitSystem(); got != InitSystemOpenRC {
			t.Errorf("DetectInitSystem() = %v, want %v", got, InitSystemOpenRC)
		}
	})

	t.Run("none detected", func(t *testing.T) {
		detectInitSystemFunc = func() InitSystem { return InitSystemUnknown }
		if got := DetectInitSystem(); got != InitSystemUnknown {
			t.Errorf("DetectInitSystem() = %v, want %v", got, InitSystemUnknown)
		}
	})
}

func TestDockerMessage(t *testing.T) {
	origEnv := os.Getenv("LUI_IN_DOCKER")
	defer os.Setenv("LUI_IN_DOCKER", origEnv)

	t.Run("in docker returns true", func(t *testing.T) {
		os.Setenv("LUI_IN_DOCKER", "true")
		if !DockerMessage() {
			t.Error("DockerMessage() = false, want true")
		}
	})

	t.Run("not in docker returns false", func(t *testing.T) {
		os.Setenv("LUI_IN_DOCKER", "")
		if DockerMessage() {
			t.Error("DockerMessage() = true, want false")
		}
	})
}

func TestStartServiceSystemd(t *testing.T) {
	origExec := execCommand
	origInitSys := detectInitSystemFunc
	defer func() {
		execCommand = origExec
		detectInitSystemFunc = origInitSys
	}()

	var capturedArgs [][]string
	execCommand = func(name string, args ...string) *exec.Cmd {
		capturedArgs = append(capturedArgs, append([]string{name}, args...))
		return exec.Command("echo")
	}
	detectInitSystemFunc = func() InitSystem { return InitSystemSystemd }

	cmd := &cobra.Command{}
	startService(cmd, nil)

	if len(capturedArgs) != 1 {
		t.Fatalf("expected 1 exec call, got %d", len(capturedArgs))
	}
	if capturedArgs[0][0] != "systemctl" || capturedArgs[0][1] != "start" || capturedArgs[0][2] != "l-ui" {
		t.Errorf("unexpected args: %v", capturedArgs[0])
	}
}

func TestStartServiceOpenRC(t *testing.T) {
	origExec := execCommand
	origInitSys := detectInitSystemFunc
	defer func() {
		execCommand = origExec
		detectInitSystemFunc = origInitSys
	}()

	var capturedArgs [][]string
	execCommand = func(name string, args ...string) *exec.Cmd {
		capturedArgs = append(capturedArgs, append([]string{name}, args...))
		return exec.Command("echo")
	}
	detectInitSystemFunc = func() InitSystem { return InitSystemOpenRC }

	cmd := &cobra.Command{}
	startService(cmd, nil)

	if len(capturedArgs) != 1 {
		t.Fatalf("expected 1 exec call, got %d", len(capturedArgs))
	}
	if capturedArgs[0][0] != "rc-service" || capturedArgs[0][1] != "l-ui" || capturedArgs[0][2] != "start" {
		t.Errorf("unexpected args: %v", capturedArgs[0])
	}
}

func TestStopServiceSystemd(t *testing.T) {
	origExec := execCommand
	origInitSys := detectInitSystemFunc
	defer func() {
		execCommand = origExec
		detectInitSystemFunc = origInitSys
	}()

	var capturedArgs [][]string
	execCommand = func(name string, args ...string) *exec.Cmd {
		capturedArgs = append(capturedArgs, append([]string{name}, args...))
		return exec.Command("echo")
	}
	detectInitSystemFunc = func() InitSystem { return InitSystemSystemd }

	cmd := &cobra.Command{}
	stopService(cmd, nil)

	if len(capturedArgs) != 1 {
		t.Fatalf("expected 1 exec call, got %d", len(capturedArgs))
	}
	if capturedArgs[0][0] != "systemctl" || capturedArgs[0][1] != "stop" || capturedArgs[0][2] != "l-ui" {
		t.Errorf("unexpected args: %v", capturedArgs[0])
	}
}

func TestRestartServiceSystemd(t *testing.T) {
	origExec := execCommand
	origInitSys := detectInitSystemFunc
	defer func() {
		execCommand = origExec
		detectInitSystemFunc = origInitSys
	}()

	var capturedArgs [][]string
	execCommand = func(name string, args ...string) *exec.Cmd {
		capturedArgs = append(capturedArgs, append([]string{name}, args...))
		return exec.Command("echo")
	}
	detectInitSystemFunc = func() InitSystem { return InitSystemSystemd }

	cmd := &cobra.Command{}
	restartService(cmd, nil)

	if len(capturedArgs) != 1 {
		t.Fatalf("expected 1 exec call, got %d", len(capturedArgs))
	}
	if capturedArgs[0][0] != "systemctl" || capturedArgs[0][1] != "restart" || capturedArgs[0][2] != "l-ui" {
		t.Errorf("unexpected args: %v", capturedArgs[0])
	}
}

func TestRestartXrayServiceSystemd(t *testing.T) {
	origExec := execCommand
	origInitSys := detectInitSystemFunc
	defer func() {
		execCommand = origExec
		detectInitSystemFunc = origInitSys
	}()

	var capturedArgs [][]string
	execCommand = func(name string, args ...string) *exec.Cmd {
		capturedArgs = append(capturedArgs, append([]string{name}, args...))
		return exec.Command("echo")
	}
	detectInitSystemFunc = func() InitSystem { return InitSystemSystemd }

	cmd := &cobra.Command{}
	restartXrayService(cmd, nil)

	if len(capturedArgs) != 1 {
		t.Fatalf("expected 1 exec call, got %d", len(capturedArgs))
	}
	if capturedArgs[0][0] != "systemctl" || capturedArgs[0][1] != "reload" || capturedArgs[0][2] != "l-ui" {
		t.Errorf("unexpected args: %v", capturedArgs[0])
	}
}

func TestEnableServiceSystemd(t *testing.T) {
	origExec := execCommand
	origInitSys := detectInitSystemFunc
	defer func() {
		execCommand = origExec
		detectInitSystemFunc = origInitSys
	}()

	var capturedArgs [][]string
	execCommand = func(name string, args ...string) *exec.Cmd {
		capturedArgs = append(capturedArgs, append([]string{name}, args...))
		return exec.Command("echo")
	}
	detectInitSystemFunc = func() InitSystem { return InitSystemSystemd }

	cmd := &cobra.Command{}
	enableService(cmd, nil)

	if len(capturedArgs) != 1 {
		t.Fatalf("expected 1 exec call, got %d", len(capturedArgs))
	}
	if capturedArgs[0][0] != "systemctl" || capturedArgs[0][1] != "enable" || capturedArgs[0][2] != "l-ui" {
		t.Errorf("unexpected args: %v", capturedArgs[0])
	}
}

func TestDisableServiceSystemd(t *testing.T) {
	origExec := execCommand
	origInitSys := detectInitSystemFunc
	defer func() {
		execCommand = origExec
		detectInitSystemFunc = origInitSys
	}()

	var capturedArgs [][]string
	execCommand = func(name string, args ...string) *exec.Cmd {
		capturedArgs = append(capturedArgs, append([]string{name}, args...))
		return exec.Command("echo")
	}
	detectInitSystemFunc = func() InitSystem { return InitSystemSystemd }

	cmd := &cobra.Command{}
	disableService(cmd, nil)

	if len(capturedArgs) != 1 {
		t.Fatalf("expected 1 exec call, got %d", len(capturedArgs))
	}
	if capturedArgs[0][0] != "systemctl" || capturedArgs[0][1] != "disable" || capturedArgs[0][2] != "l-ui" {
		t.Errorf("unexpected args: %v", capturedArgs[0])
	}
}

func TestEnableServiceOpenRC(t *testing.T) {
	origExec := execCommand
	origInitSys := detectInitSystemFunc
	defer func() {
		execCommand = origExec
		detectInitSystemFunc = origInitSys
	}()

	var capturedArgs [][]string
	execCommand = func(name string, args ...string) *exec.Cmd {
		capturedArgs = append(capturedArgs, append([]string{name}, args...))
		return exec.Command("echo")
	}
	detectInitSystemFunc = func() InitSystem { return InitSystemOpenRC }

	cmd := &cobra.Command{}
	enableService(cmd, nil)

	if len(capturedArgs) != 1 {
		t.Fatalf("expected 1 exec call, got %d", len(capturedArgs))
	}
	if capturedArgs[0][0] != "rc-update" || capturedArgs[0][1] != "add" || capturedArgs[0][2] != "l-ui" {
		t.Errorf("unexpected args: %v", capturedArgs[0])
	}
}

func TestDisableServiceOpenRC(t *testing.T) {
	origExec := execCommand
	origInitSys := detectInitSystemFunc
	defer func() {
		execCommand = origExec
		detectInitSystemFunc = origInitSys
	}()

	var capturedArgs [][]string
	execCommand = func(name string, args ...string) *exec.Cmd {
		capturedArgs = append(capturedArgs, append([]string{name}, args...))
		return exec.Command("echo")
	}
	detectInitSystemFunc = func() InitSystem { return InitSystemOpenRC }

	cmd := &cobra.Command{}
	disableService(cmd, nil)

	if len(capturedArgs) != 1 {
		t.Fatalf("expected 1 exec call, got %d", len(capturedArgs))
	}
	if capturedArgs[0][0] != "rc-update" || capturedArgs[0][1] != "del" || capturedArgs[0][2] != "l-ui" {
		t.Errorf("unexpected args: %v", capturedArgs[0])
	}
}

func TestRestartServiceDocker(t *testing.T) {
	origEnv := os.Getenv("LUI_IN_DOCKER")
	origFindPID := findPIDFunc
	defer func() {
		os.Setenv("LUI_IN_DOCKER", origEnv)
		findPIDFunc = origFindPID
	}()

	os.Setenv("LUI_IN_DOCKER", "true")
	findPIDFunc = func(name string) (int, error) {
		if name == "l-ui" {
			return 12345, nil
		}
		return 0, fmt.Errorf("not found")
	}

	cmd := &cobra.Command{}
	restartService(cmd, nil)
	// In Docker with a found PID, it should send SIGHUP. We can't easily verify
	// the signal was sent without mocking os.FindProcess, but we verify no panic.
}

func TestInitSystemConstants(t *testing.T) {
	if InitSystemUnknown != 0 {
		t.Errorf("InitSystemUnknown = %d, want 0", InitSystemUnknown)
	}
	if InitSystemSystemd != 1 {
		t.Errorf("InitSystemSystemd = %d, want 1", InitSystemSystemd)
	}
	if InitSystemOpenRC != 2 {
		t.Errorf("InitSystemOpenRC = %d, want 2", InitSystemOpenRC)
	}
}
