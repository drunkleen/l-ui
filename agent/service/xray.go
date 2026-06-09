package service

import (
	"archive/zip"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/drunkleen/l-ui/internal/config"
	"github.com/drunkleen/l-ui/internal/logger"
)

const xrayDownloadTimeout = 120 * time.Second

type XrayService struct{}

type XrayInstallRequest struct {
	Version string `json:"version"`
}

type ApplyConfigRequest struct {
	XrayConfig json.RawMessage `json:"xray_config"`
}

func (s *XrayService) GetXrayVersion() string {
	candidates := []string{"xray"}
	if binDir := config.GetBinFolderPath(); binDir != "" {
		fi, err := os.Stat(filepath.Join(binDir, "xray"))
		if err == nil && !fi.IsDir() {
			candidates = append(candidates, filepath.Join(binDir, "xray"))
		}
		// Try the os-arch prefixed name too
		prefixed := filepath.Join(binDir, fmt.Sprintf("xray-%s-%s", runtime.GOOS, runtime.GOARCH))
		fi2, err2 := os.Stat(prefixed)
		if err2 == nil && !fi2.IsDir() {
			candidates = append(candidates, prefixed)
		}
	}
	candidates = append(candidates,
		"/usr/local/bin/xray",
		"/usr/local/l-ui/bin/xray",
		"/usr/local/l-ui-agent/bin/xray",
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

func (s *XrayService) IsXrayRunning() bool {
	if err := exec.Command("systemctl", "is-active", "--quiet", "xray").Run(); err == nil {
		return true
	}
	// Fallback: check if the binary is running
	binPath := s.xrayBinaryPath()
	if binPath == "" {
		return false
	}
	out, err := exec.Command("pgrep", "-f", binPath).Output()
	return err == nil && len(strings.TrimSpace(string(out))) > 0
}

func (s *XrayService) xrayBinaryPath() string {
	binDir := config.GetBinFolderPath()
	prefixed := filepath.Join(binDir, fmt.Sprintf("xray-%s-%s", runtime.GOOS, runtime.GOARCH))
	if fi, err := os.Stat(prefixed); err == nil && !fi.IsDir() {
		return prefixed
	}
	if binDir != "" {
		plain := filepath.Join(binDir, "xray")
		if fi, err := os.Stat(plain); err == nil && !fi.IsDir() {
			return plain
		}
	}
	return ""
}

func (s *XrayService) InstallXray(version string) error {
	version = strings.TrimPrefix(version, "v")
	downloadURL := fmt.Sprintf(
		"https://github.com/XTLS/Xray-core/releases/download/v%s/Xray-%s-%s.zip",
		version, runtime.GOOS, runtime.GOARCH,
	)

	logger.Infof("Downloading Xray %s from %s", version, downloadURL)

	client := &http.Client{Timeout: xrayDownloadTimeout}
	resp, err := client.Get(downloadURL)
	if err != nil {
		return fmt.Errorf("download xray: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("download xray: HTTP %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("read xray zip: %w", err)
	}

	zipReader, err := zip.NewReader(bytes.NewReader(body), int64(len(body)))
	if err != nil {
		return fmt.Errorf("open xray zip: %w", err)
	}

	binDir := config.GetBinFolderPath()
	if err := os.MkdirAll(binDir, 0755); err != nil {
		return fmt.Errorf("mkdir bin dir: %w", err)
	}

	targetName := fmt.Sprintf("xray-%s-%s", runtime.GOOS, runtime.GOARCH)
	targetPath := filepath.Join(binDir, targetName)

	for _, f := range zipReader.File {
		if f.Name == "xray" || f.Name == "xray.exe" {
			src, err := f.Open()
			if err != nil {
				return fmt.Errorf("open xray entry: %w", err)
			}
			defer src.Close()

			dst, err := os.OpenFile(targetPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0755)
			if err != nil {
				return fmt.Errorf("create xray binary: %w", err)
			}
			defer dst.Close()

			if _, err := io.Copy(dst, src); err != nil {
				return fmt.Errorf("write xray binary: %w", err)
			}
			logger.Infof("Xray %s installed to %s", version, targetPath)
			return nil
		}
	}

	return fmt.Errorf("xray binary not found in release zip")
}

var RestartXrayFn = func() error {
	return exec.Command("systemctl", "restart", "xray").Run()
}

func (s *XrayService) ApplyConfig(configJSON json.RawMessage) error {
	configPath := config.GetBinFolderPath() + "/config.json"
	if err := os.MkdirAll(filepath.Dir(configPath), 0755); err != nil {
		return fmt.Errorf("mkdir config dir: %w", err)
	}

	pretty, err := json.MarshalIndent(configJSON, "", "  ")
	if err != nil {
		return fmt.Errorf("format config: %w", err)
	}

	if err := os.WriteFile(configPath, pretty, 0644); err != nil {
		return fmt.Errorf("write config: %w", err)
	}

	// Backward compat: also write to the legacy path used by pre-existing xray.service
	legacyPath := "/usr/local/l-ui/bin/config.json"
	if legacyPath != configPath {
		if err := os.MkdirAll(filepath.Dir(legacyPath), 0755); err == nil {
			_ = os.WriteFile(legacyPath, pretty, 0644)
		}
	}

	logger.Infof("Xray config written to %s and legacy path, restarting xray", configPath)

	if err := RestartXrayFn(); err != nil {
		return fmt.Errorf("restart xray: %w", err)
	}

	return nil
}

func (s *XrayService) RestartXray() error {
	if err := RestartXrayFn(); err != nil {
		return fmt.Errorf("restart xray: %w", err)
	}
	return nil
}
