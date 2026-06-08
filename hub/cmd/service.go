package cmd

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"

	"github.com/charmbracelet/lipgloss"
	"github.com/spf13/cobra"
)

var (
	serviceName = "l-ui"

	// Colors
	greenStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("#00FF7F"))
	redStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("#FF6B6B"))
	yellowStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("#FFD700"))
	cyanStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("#00CED1"))

	// execCommand allows overriding in tests
	execCommand = exec.Command

	// detectInitSystemFunc allows overriding in tests
	detectInitSystemFunc = detectInitSystemImpl

	// findPIDFunc allows overriding in tests
	findPIDFunc = findPIDImpl
)

// InitSystem represents the detected init system
type InitSystem int

const (
	InitSystemUnknown InitSystem = iota
	InitSystemSystemd
	InitSystemOpenRC
)

// detectInitSystemImpl is the real implementation
func detectInitSystemImpl() InitSystem {
	if _, err := execCommand("systemctl", "--version").Output(); err == nil {
		return InitSystemSystemd
	}
	if _, err := execCommand("rc-service", "--version").Output(); err == nil {
		return InitSystemOpenRC
	}
	return InitSystemUnknown
}

// DetectInitSystem returns the detected init system (can be overridden in tests)
func DetectInitSystem() InitSystem {
	return detectInitSystemFunc()
}

// findPIDImpl is the real implementation
func findPIDImpl(name string) (int, error) {
	cmd := execCommand("pgrep", "-x", name)
	output, err := cmd.Output()
	if err != nil {
		return 0, fmt.Errorf("process %q not found", name)
	}
	var pid int
	_, err = fmt.Sscanf(strings.TrimSpace(string(output)), "%d", &pid)
	if err != nil {
		return 0, fmt.Errorf("failed to parse PID: %w", err)
	}
	return pid, nil
}

// findPIDByName finds the PID of a process by name (uses findPIDFunc)
func findPIDByName(name string) (int, error) {
	return findPIDFunc(name)
}

// sendSignal sends a signal to a process by name
func sendSignal(name string, sig syscall.Signal) error {
	pid, err := findPIDByName(name)
	if err != nil {
		return err
	}
	proc, err := os.FindProcess(pid)
	if err != nil {
		return fmt.Errorf("failed to find process: %w", err)
	}
	return proc.Signal(sig)
}

// DockerMessage prints the standard Docker message and returns true if running in Docker
func DockerMessage() bool {
	if isDocker() {
		fmt.Println(cyanStyle.Render("ℹ  Running in Docker container"))
		fmt.Println(yellowStyle.Render("   Use 'docker compose restart' or 'docker restart <container>' instead."))
		return true
	}
	return false
}

// --- Start Service ---

var startCmd = &cobra.Command{
	Use:   "start",
	Short: "Start the l-ui service",
	Run:   startService,
}

func startService(cmd *cobra.Command, args []string) {
	if DockerMessage() {
		return
	}

	initSys := DetectInitSystem()
	switch initSys {
	case InitSystemSystemd:
		out, err := execCommand("systemctl", "start", serviceName).CombinedOutput()
		if err != nil {
			fmt.Println(redStyle.Render("✖  Failed to start service:"))
			fmt.Println(string(out))
			return
		}
		fmt.Println(greenStyle.Render("✓  Service started successfully"))
	case InitSystemOpenRC:
		out, err := execCommand("rc-service", serviceName, "start").CombinedOutput()
		if err != nil {
			fmt.Println(redStyle.Render("✖  Failed to start service:"))
			fmt.Println(string(out))
			return
		}
		fmt.Println(greenStyle.Render("✓  Service started successfully"))
	default:
		fmt.Println(yellowStyle.Render("⚠  Could not detect init system. Trying systemctl..."))
		out, err := execCommand("systemctl", "start", serviceName).CombinedOutput()
		if err != nil {
			fmt.Println(redStyle.Render("✖  Failed to start service:"))
			fmt.Println(string(out))
			return
		}
		fmt.Println(greenStyle.Render("✓  Service started successfully"))
	}
}

// --- Stop Service ---

var stopCmd = &cobra.Command{
	Use:   "stop",
	Short: "Stop the l-ui service",
	Run:   stopService,
}

func stopService(cmd *cobra.Command, args []string) {
	if DockerMessage() {
		return
	}

	initSys := DetectInitSystem()
	switch initSys {
	case InitSystemSystemd:
		out, err := execCommand("systemctl", "stop", serviceName).CombinedOutput()
		if err != nil {
			fmt.Println(redStyle.Render("✖  Failed to stop service:"))
			fmt.Println(string(out))
			return
		}
		fmt.Println(greenStyle.Render("✓  Service stopped successfully"))
	case InitSystemOpenRC:
		out, err := execCommand("rc-service", serviceName, "stop").CombinedOutput()
		if err != nil {
			fmt.Println(redStyle.Render("✖  Failed to stop service:"))
			fmt.Println(string(out))
			return
		}
		fmt.Println(greenStyle.Render("✓  Service stopped successfully"))
	default:
		fmt.Println(yellowStyle.Render("⚠  Could not detect init system. Trying systemctl..."))
		out, err := execCommand("systemctl", "stop", serviceName).CombinedOutput()
		if err != nil {
			fmt.Println(redStyle.Render("✖  Failed to stop service:"))
			fmt.Println(string(out))
			return
		}
		fmt.Println(greenStyle.Render("✓  Service stopped successfully"))
	}
}

// --- Restart Service ---

var restartCmd = &cobra.Command{
	Use:   "restart",
	Short: "Restart the l-ui service",
	Run:   restartService,
}

func restartService(cmd *cobra.Command, args []string) {
	if DockerMessage() {
		fmt.Println(cyanStyle.Render("ℹ  Sending HUP signal to l-ui process in Docker..."))
		if err := sendSignal("l-ui", syscall.SIGHUP); err != nil {
			fmt.Println(redStyle.Render("✖  Failed to restart service:"))
			fmt.Println(err.Error())
			return
		}
		fmt.Println(greenStyle.Render("✓  Service restarted successfully"))
		return
	}

	initSys := DetectInitSystem()
	switch initSys {
	case InitSystemSystemd:
		out, err := execCommand("systemctl", "restart", serviceName).CombinedOutput()
		if err != nil {
			fmt.Println(redStyle.Render("✖  Failed to restart service:"))
			fmt.Println(string(out))
			return
		}
		fmt.Println(greenStyle.Render("✓  Service restarted successfully"))
	case InitSystemOpenRC:
		out, err := execCommand("rc-service", serviceName, "restart").CombinedOutput()
		if err != nil {
			fmt.Println(redStyle.Render("✖  Failed to restart service:"))
			fmt.Println(string(out))
			return
		}
		fmt.Println(greenStyle.Render("✓  Service restarted successfully"))
	default:
		fmt.Println(yellowStyle.Render("⚠  Could not detect init system. Trying systemctl..."))
		out, err := execCommand("systemctl", "restart", serviceName).CombinedOutput()
		if err != nil {
			fmt.Println(redStyle.Render("✖  Failed to restart service:"))
			fmt.Println(string(out))
			return
		}
		fmt.Println(greenStyle.Render("✓  Service restarted successfully"))
	}
}

// --- Restart Xray ---

var restartXrayCmd = &cobra.Command{
	Use:   "restart-xray",
	Short: "Send Xray reload signal (USR1)",
	Run:   restartXrayService,
}

func restartXrayService(cmd *cobra.Command, args []string) {
	if DockerMessage() {
		fmt.Println(cyanStyle.Render("ℹ  Sending USR1 signal to xray process in Docker..."))
		if err := sendSignal("xray", syscall.SIGUSR1); err != nil {
			if err2 := sendSignal("l-ui", syscall.SIGUSR1); err2 != nil {
				fmt.Println(redStyle.Render("✖  Failed to reload Xray:"))
				fmt.Println(err.Error())
				return
			}
		}
		fmt.Println(greenStyle.Render("✓  Xray reloaded successfully"))
		return
	}

	initSys := DetectInitSystem()
	switch initSys {
	case InitSystemSystemd:
		out, err := execCommand("systemctl", "reload", serviceName).CombinedOutput()
		if err != nil {
			fmt.Println(redStyle.Render("✖  Failed to reload Xray:"))
			fmt.Println(string(out))
			return
		}
		fmt.Println(greenStyle.Render("✓  Xray reloaded successfully"))
	case InitSystemOpenRC:
		out, err := execCommand("rc-service", serviceName, "reload").CombinedOutput()
		if err != nil {
			fmt.Println(redStyle.Render("✖  Failed to reload Xray:"))
			fmt.Println(string(out))
			return
		}
		fmt.Println(greenStyle.Render("✓  Xray reloaded successfully"))
	default:
		fmt.Println(yellowStyle.Render("⚠  Could not detect init system. Trying systemctl reload..."))
		out, err := execCommand("systemctl", "reload", serviceName).CombinedOutput()
		if err != nil {
			fmt.Println(redStyle.Render("✖  Failed to reload Xray:"))
			fmt.Println(string(out))
			return
		}
		fmt.Println(greenStyle.Render("✓  Xray reloaded successfully"))
	}
}

// --- Status ---

var statusCmd = &cobra.Command{
	Use:   "status",
	Short: "Show l-ui service status",
	Run:   showStatus,
}

func showStatus(cmd *cobra.Command, args []string) {
	if isDocker() {
		_, err := findPIDByName("l-ui")
		if err != nil {
			fmt.Println(yellowStyle.Render("⚠  Status: Stopped"))
		} else {
			fmt.Println(greenStyle.Render("✓  Status: Running"))
		}
		fmt.Println(cyanStyle.Render("ℹ  Running in Docker container"))
		return
	}

	initSys := DetectInitSystem()
	var out []byte
	var err error

	switch initSys {
	case InitSystemSystemd:
		out, err = execCommand("systemctl", "is-active", serviceName).Output()
	case InitSystemOpenRC:
		out, err = execCommand("rc-service", serviceName, "status").Output()
	default:
		out, err = execCommand("systemctl", "is-active", serviceName).Output()
		if err != nil {
			_, pgrepErr := findPIDByName("l-ui")
			if pgrepErr != nil {
				fmt.Println(yellowStyle.Render("⚠  Status: Not installed"))
			} else {
				fmt.Println(greenStyle.Render("✓  Status: Running"))
			}
			return
		}
	}

	if err != nil {
		fmt.Println(yellowStyle.Render("⚠  Status: Not installed"))
		return
	}

	status := strings.TrimSpace(string(out))
	switch initSys {
	case InitSystemSystemd:
		if status == "active" {
			fmt.Println(greenStyle.Render("✓  Status: Running"))
		} else {
			fmt.Println(redStyle.Render("✖  Status: ") + status)
		}
	case InitSystemOpenRC:
		lowerStatus := strings.ToLower(status)
		if strings.Contains(lowerStatus, "start") || strings.Contains(lowerStatus, "run") {
			fmt.Println(greenStyle.Render("✓  Status: Running"))
		} else if strings.Contains(lowerStatus, "stop") {
			fmt.Println(yellowStyle.Render("⚠  Status: Stopped"))
		} else {
			fmt.Println(status)
		}
	}
}

// --- Enable Autostart ---

var enableCmd = &cobra.Command{
	Use:   "enable",
	Short: "Enable l-ui autostart",
	Run:   enableService,
}

func enableService(cmd *cobra.Command, args []string) {
	if DockerMessage() {
		return
	}

	initSys := DetectInitSystem()
	switch initSys {
	case InitSystemSystemd:
		out, err := execCommand("systemctl", "enable", serviceName).CombinedOutput()
		if err != nil {
			fmt.Println(redStyle.Render("✖  Failed to enable service:"))
			fmt.Println(string(out))
			return
		}
		fmt.Println(greenStyle.Render("✓  Autostart enabled successfully"))
	case InitSystemOpenRC:
		out, err := execCommand("rc-update", "add", serviceName).CombinedOutput()
		if err != nil {
			fmt.Println(redStyle.Render("✖  Failed to enable service:"))
			fmt.Println(string(out))
			return
		}
		fmt.Println(greenStyle.Render("✓  Autostart enabled successfully"))
	default:
		fmt.Println(yellowStyle.Render("⚠  Could not detect init system. Trying systemctl..."))
		out, err := execCommand("systemctl", "enable", serviceName).CombinedOutput()
		if err != nil {
			fmt.Println(redStyle.Render("✖  Failed to enable service:"))
			fmt.Println(string(out))
			return
		}
		fmt.Println(greenStyle.Render("✓  Autostart enabled successfully"))
	}
}

// --- Disable Autostart ---

var disableCmd = &cobra.Command{
	Use:   "disable",
	Short: "Disable l-ui autostart",
	Run:   disableService,
}

func disableService(cmd *cobra.Command, args []string) {
	if DockerMessage() {
		return
	}

	initSys := DetectInitSystem()
	switch initSys {
	case InitSystemSystemd:
		out, err := execCommand("systemctl", "disable", serviceName).CombinedOutput()
		if err != nil {
			fmt.Println(redStyle.Render("✖  Failed to disable service:"))
			fmt.Println(string(out))
			return
		}
		fmt.Println(greenStyle.Render("✓  Autostart disabled successfully"))
	case InitSystemOpenRC:
		out, err := execCommand("rc-update", "del", serviceName).CombinedOutput()
		if err != nil {
			fmt.Println(redStyle.Render("✖  Failed to disable service:"))
			fmt.Println(string(out))
			return
		}
		fmt.Println(greenStyle.Render("✓  Autostart disabled successfully"))
	default:
		fmt.Println(yellowStyle.Render("⚠  Could not detect init system. Trying systemctl..."))
		out, err := execCommand("systemctl", "disable", serviceName).CombinedOutput()
		if err != nil {
			fmt.Println(redStyle.Render("✖  Failed to disable service:"))
			fmt.Println(string(out))
			return
		}
		fmt.Println(greenStyle.Render("✓  Autostart disabled successfully"))
	}
}

// --- Show Logs ---

var logCmd = &cobra.Command{
	Use:   "log",
	Short: "Show l-ui service logs",
	Run:   showLogs,
}

func showLogs(cmd *cobra.Command, args []string) {
	if isDocker() {
		fmt.Println(cyanStyle.Render("ℹ  Docker container detected"))
		fmt.Println(yellowStyle.Render("   Use 'docker compose logs -f' or 'docker logs <container>' instead."))
		return
	}

	initSys := DetectInitSystem()
	switch initSys {
	case InitSystemSystemd:
		cmd := execCommand("journalctl", "-u", serviceName, "-f")
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		cmd.Stdin = os.Stdin
		if err := cmd.Run(); err != nil {
			fmt.Println(redStyle.Render("✖  Failed to show logs:"))
			fmt.Println(err.Error())
		}
	case InitSystemOpenRC:
		logPaths := []string{
			filepath.Join("/var/log", serviceName+".log"),
			filepath.Join("/var/log", serviceName),
		}
		for _, logPath := range logPaths {
			if _, err := os.Stat(logPath); err == nil {
				cmd := execCommand("cat", logPath)
				cmd.Stdout = os.Stdout
				cmd.Stderr = os.Stderr
				if err := cmd.Run(); err != nil {
					fmt.Println(redStyle.Render("✖  Failed to show logs:"))
					fmt.Println(err.Error())
				}
				return
			}
		}
		fmt.Println(yellowStyle.Render("⚠  No log file found. Showing recent system messages..."))
		cmd := execCommand("dmesg")
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		_ = cmd.Run()
	default:
		fmt.Println(yellowStyle.Render("⚠  Could not detect init system. Trying journalctl..."))
		cmd := execCommand("journalctl", "-u", serviceName)
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		if err := cmd.Run(); err != nil {
			fmt.Println(redStyle.Render("✖  Failed to show logs:"))
			fmt.Println(err.Error())
		}
	}
}

func init() {
	rootCmd.AddCommand(startCmd)
	rootCmd.AddCommand(stopCmd)
	rootCmd.AddCommand(restartCmd)
	rootCmd.AddCommand(restartXrayCmd)
	rootCmd.AddCommand(statusCmd)
	rootCmd.AddCommand(enableCmd)
	rootCmd.AddCommand(disableCmd)
	rootCmd.AddCommand(logCmd)
}
