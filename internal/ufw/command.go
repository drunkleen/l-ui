package ufw

import (
	"context"
	"errors"
	"fmt"
	"os/exec"
	"runtime"
	"strconv"
	"strings"
	"time"
)

const (
	cmdTimeout     = 30 * time.Second
	maxPort        = 65535
	safeCommentLen = 128
)

// CmdRunner executes UFW commands. Package-level variable so tests can
// inject a mock runner without fighting func-literal indirection.
var CmdRunner Runner = osRunner{}

// Runner abstracts ufw command execution for testability.
type Runner interface {
	LookPath(file string) (string, error)
	Run(ctx context.Context, name string, args ...string) ([]byte, error)
}

type osRunner struct{}

func (osRunner) LookPath(file string) (string, error) { return exec.LookPath(file) }

func (osRunner) Run(ctx context.Context, name string, args ...string) ([]byte, error) {
	cmd := exec.CommandContext(ctx, name, args...)
	return cmd.CombinedOutput()
}

// SanitizePort validates that port is a number between 1–65535.
func SanitizePort(port interface{}) (int, error) {
	switch v := port.(type) {
	case int:
		if v < 1 || v > maxPort {
			return 0, fmt.Errorf("port must be 1-%d, got %d", maxPort, v)
		}
		return v, nil
	case string:
		n, err := strconv.Atoi(strings.TrimSpace(v))
		if err != nil {
			return 0, fmt.Errorf("invalid port %q: %w", v, err)
		}
		if n < 1 || n > maxPort {
			return 0, fmt.Errorf("port must be 1-%d, got %d", maxPort, n)
		}
		return n, nil
	default:
		return 0, errors.New("port must be a number or numeric string")
	}
}

// SanitizeProtocol validates protocol string.
func SanitizeProtocol(protocol string) (string, error) {
	p := strings.ToLower(strings.TrimSpace(protocol))
	if p == "" {
		return "", nil
	}
	if p != "tcp" && p != "udp" {
		return "", fmt.Errorf("protocol must be tcp or udp, got %q", protocol)
	}
	return p, nil
}

// SanitizeComment validates a comment string for safe shell use.
func SanitizeComment(comment string) string {
	// Strip to safe length and reject shell metacharacters
	if len(comment) > safeCommentLen {
		comment = comment[:safeCommentLen]
	}
	if strings.ContainsAny(comment, ";&|`$!\\'\"<>(){}[]#~") {
		return ""
	}
	return strings.TrimSpace(comment)
}

// IsInstalled checks whether ufw is available on the system.
func IsInstalled() bool {
	_, err := CmdRunner.LookPath("ufw")
	return err == nil
}

// IsInstalledWithRunner checks whether ufw is available, using the provided runner.
func IsInstalledWithRunner(runner Runner) bool {
	_, err := runner.LookPath("ufw")
	return err == nil
}

func isLinux() bool {
	return runtime.GOOS == "linux"
}

// IsActive checks whether ufw is currently active.
func IsActive() (bool, error) {
	ctx, cancel := context.WithTimeout(context.Background(), cmdTimeout)
	defer cancel()
	out, err := CmdRunner.Run(ctx, "ufw", "status")
	if err != nil {
		return false, fmt.Errorf("ufw status check failed: %w", err)
	}
	return strings.Contains(strings.ToLower(string(out)), "active"), nil
}

// IsActiveWithRunner checks whether ufw is active, using the provided runner.
func IsActiveWithRunner(runner Runner) (bool, error) {
	ctx, cancel := context.WithTimeout(context.Background(), cmdTimeout)
	defer cancel()
	out, err := runner.Run(ctx, "ufw", "status")
	if err != nil {
		return false, fmt.Errorf("ufw status check failed: %w", err)
	}
	return strings.Contains(strings.ToLower(string(out)), "active"), nil
}

func runWithTimeout(args ...string) ([]byte, error) {
	ctx, cancel := context.WithTimeout(context.Background(), cmdTimeout)
	defer cancel()
	return CmdRunner.Run(ctx, "ufw", args...)
}

// RunAllow executes `ufw allow <port>/<protocol>` with optional comment.
func RunAllow(port int, protocol, comment string) error {
	args := buildRuleArgs("allow", port, protocol, comment)
	_, err := runWithTimeout(args...)
	return err
}

// RunDeny executes `ufw deny <port>/<protocol>` with optional comment.
func RunDeny(port int, protocol, comment string) error {
	args := buildRuleArgs("deny", port, protocol, comment)
	_, err := runWithTimeout(args...)
	return err
}

func buildRuleArgs(action string, port int, protocol, comment string) []string {
	target := strconv.Itoa(port)
	if protocol != "" {
		target += "/" + protocol
	}
	args := []string{action, target}
	if c := SanitizeComment(comment); c != "" {
		args = append(args, "comment", c)
	}
	return args
}

// RunDelete executes `ufw --force delete <ruleNum>`.
func RunDelete(ruleNum string) error {
	_, err := runWithTimeout("--force", "delete", ruleNum)
	return err
}

// RunEnable executes `ufw --force enable`.
func RunEnable() error {
	_, err := runWithTimeout("--force", "enable")
	return err
}

// RunDisable executes `ufw --force disable`.
func RunDisable() error {
	_, err := runWithTimeout("--force", "disable")
	return err
}

// GetRulesOutput tries `ufw status numbered` first (works when active),
// then falls back to `ufw show added` (works regardless of active state).
// Raw output is returned for the caller to parse.
func GetRulesOutput() (string, bool, error) {
	// Try numbered output first
	out, err := runWithTimeout("status", "numbered")
	if err == nil {
		return string(out), true, nil
	}

	// Fall back to show-added which works even when inactive
	out, err = runWithTimeout("show", "added")
	if err != nil {
		return "", false, fmt.Errorf("ufw rules query failed: %w", err)
	}
	return string(out), false, nil
}
