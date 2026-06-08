package cmd

import (
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/drunkleen/l-ui/internal/config"
	"github.com/spf13/cobra"
)

var updateVersion string

var updateCmd = &cobra.Command{
	Use:   "update",
	Short: "Update l-ui to the latest version",
	Long: `Downloads and installs the latest l-ui release.

Backs up the current installation, extracts the new version,
and restarts the service. Use --version to pin a specific release.`,
	RunE: runUpdate,
}

func init() {
	updateCmd.Flags().StringVarP(&updateVersion, "version", "v", "", "specific version tag (e.g. v0.0.2)")
	rootCmd.AddCommand(updateCmd)
}

func runUpdate(cmd *cobra.Command, args []string) error {
	arch := detectArch()
	version := updateVersion
	if version == "" {
		v, err := fetchLatestTag()
		if err != nil {
			return fmt.Errorf("fetch latest version: %w", err)
		}
		version = v
	}
	fmt.Printf("Updating to %s (%s)...\n", version, arch)

	tarball := fmt.Sprintf("l-ui-hub-linux-%s.tar.gz", arch)
	url := fmt.Sprintf("https://github.com/drunkleen/l-ui/releases/download/%s/%s", version, tarball)

	// Download tarball
	tmpFile := "/tmp/" + tarball
	if err := downloadFile(url, tmpFile); err != nil {
		return fmt.Errorf("download %s: %w", url, err)
	}
	defer os.Remove(tmpFile)

	// Backup
	destDir := config.GetBinFolderPath()
	if destDir == "" {
		destDir = "/usr/local/l-ui"
	}
	backupDir := ""
	if _, err := os.Stat(destDir); err == nil {
		backupDir = destDir + ".bak"
		os.RemoveAll(backupDir)
		if err := os.Rename(destDir, backupDir); err != nil {
			return fmt.Errorf("backup: %w", err)
		}
		fmt.Println("Backed up existing installation")
	}

	// Extract
	if err := exec.Command("tar", "-xzf", tmpFile, "-C", "/usr/local").Run(); err != nil {
		restoreBackup(destDir, backupDir)
		return fmt.Errorf("extract: %w", err)
	}
	fmt.Println("Extracted new version")

	// Restart service
	exec.Command("systemctl", "daemon-reload").Run()
	exec.Command("systemctl", "restart", "l-ui").Run()

	// Health check
	if err := waitForHealth(destDir + "/l-ui"); err != nil {
		restoreBackup(destDir, backupDir)
		return fmt.Errorf("health check failed — rolled back: %w", err)
	}

	fmt.Println("Update complete!")
	cleanupBackup(backupDir)
	return nil
}

func detectArch() string {
	out, err := exec.Command("uname", "-m").Output()
	if err != nil {
		return "amd64"
	}
	arch := strings.TrimSpace(string(out))
	switch arch {
	case "x86_64", "amd64":
		return "amd64"
	case "aarch64", "arm64":
		return "arm64"
	case "armv7l":
		return "armv7"
	case "armv6l":
		return "armv6"
	default:
		return arch
	}
}

func fetchLatestTag() (string, error) {
	out, err := exec.Command("curl", "-sfL", "https://api.github.com/repos/drunkleen/l-ui/releases/latest").Output()
	if err == nil {
		// Parse JSON: find "tag_name": "vX.Y.Z"
		content := string(out)
		idx := strings.Index(content, `"tag_name":`)
		if idx >= 0 {
			start := idx + 12 // skip "tag_name": "
			end := strings.Index(content[start:], `"`)
			if end > 0 {
				return content[start : start+end], nil
			}
		}
	}
	// Fallback: scrape releases page
	out, err = exec.Command("curl", "-sfL", "https://github.com/drunkleen/l-ui/releases").Output()
	if err != nil {
		return "", fmt.Errorf("cannot fetch latest version")
	}
	content := string(out)
	marker := `/drunkleen/l-ui/releases/tag/`
	idx := strings.Index(content, marker)
	if idx < 0 {
		return "", fmt.Errorf("no releases found")
	}
	start := idx + len(marker)
	end := strings.IndexAny(content[start:], `"< `)
	if end < 0 {
		end = len(content) - start
	}
	return content[start : start+end], nil
}

func downloadFile(url, dest string) error {
	cmd := exec.Command("curl", "-fSL", "-o", dest, url)
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("%s\n%s", err.Error(), strings.TrimSpace(string(out)))
	}
	return nil
}

func waitForHealth(binPath string) error {
	// Try curling the health endpoint
	for i := 0; i < 30; i++ {
		if err := exec.Command("curl", "-fs", "http://127.0.0.1:2053/healthz").Run(); err == nil {
			return nil
		}
		exec.Command("sleep", "1").Run()
	}
	journal, _ := exec.Command("journalctl", "-u", "l-ui", "--no-pager", "-n", "20").CombinedOutput()
	return fmt.Errorf("service not ready — journal:\n%s", string(journal))
}

func restoreBackup(destDir, backupDir string) {
	if backupDir == "" {
		return
	}
	os.RemoveAll(destDir)
	os.Rename(backupDir, destDir)
	fmt.Println("Rolled back to previous version")
}

func cleanupBackup(backupDir string) {
	if backupDir == "" {
		return
	}
	os.RemoveAll(backupDir)
}
