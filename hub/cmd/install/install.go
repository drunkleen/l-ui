// Package install provides the interactive l-ui installation wizard.
// It is embedded in the hub binary and launched via `l-ui install`.
// The thin install.sh shell script downloads the hub tarball, extracts
// it to a temp directory, then exec's this binary's install subcommand.
package install

import (
	"crypto/rand"
	"fmt"
	"math/big"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/drunkleen/l-ui/internal/config"
	"github.com/spf13/cobra"
)

var defaultPort string

func init() {
	n, _ := rand.Int(rand.Reader, big.NewInt(8000))
	defaultPort = fmt.Sprintf("%d", 2000+int(n.Int64()))
}

var (
	flagNonInteractive bool
	flagPort           string
	flagBasePath       string
	flagDomain         string
	flagIP             string
	flagCertPath       string
	flagUsername       string
	flagPassword       string
	flagVersion        string
	flagTarball        string // path to pre-downloaded tarball (set by install.sh)
)

// Cmd returns the install subcommand.
func Cmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "install",
		Short: "Install or reinstall l-ui hub",
		Long: `Interactive installation wizard for l-ui hub.

Uses a Bubble Tea TUI to configure port, SSL, credentials, and
more.  Use --non-interactive for automated provisioning.

When run from install.sh the tarball path is passed via --tarball
so the download step is skipped.`,
		RunE: runInstall,
	}
	cmd.Flags().BoolVarP(&flagNonInteractive, "non-interactive", "n", false, "non-interactive mode")
	cmd.Flags().StringVarP(&flagPort, "port", "p", "", "panel port")
	cmd.Flags().StringVar(&flagBasePath, "base-path", "", "web base path")
	cmd.Flags().StringVar(&flagDomain, "domain", "", "domain for Let's Encrypt")
	cmd.Flags().StringVar(&flagIP, "ip", "", "IP for Let's Encrypt")
	cmd.Flags().StringVar(&flagCertPath, "cert-path", "", "custom certificate directory")
	cmd.Flags().StringVarP(&flagUsername, "username", "u", "", "admin username")
	cmd.Flags().StringVarP(&flagPassword, "password", "P", "", "admin password")
	cmd.Flags().StringVarP(&flagVersion, "version", "v", "", "release version tag")
	cmd.Flags().StringVar(&flagTarball, "tarball", "", "path to pre-downloaded tarball")
	return cmd
}

func runInstall(cmd *cobra.Command, args []string) error {
	// In non-interactive mode with all required flags, do a silent install.
	if flagNonInteractive {
		return nonInteractiveInstall()
	}
	// Launch the Bubble Tea TUI wizard.
	return tuiInstall()
}

// ── Detect helpers ──────────────────────────────────────────────────

type systemInfo struct {
	OS         string
	Arch       string
	InitSystem string
	PublicIP   string
	LocalIPs   []string
}

func detectSystem() systemInfo {
	info := systemInfo{
		OS:   runtime.GOOS,
		Arch: runtime.GOARCH,
	}
	// Init system
	if _, err := os.Stat("/run/systemd/system"); err == nil {
		info.InitSystem = "systemd"
	} else if _, err := os.Stat("/sbin/openrc-run"); err == nil {
		info.InitSystem = "openrc"
	} else {
		info.InitSystem = "unknown"
	}
	// Public IP
	if ip := fetchPublicIP(); ip != "" {
		info.PublicIP = ip
	}
	// Local IPs
	if ips := fetchLocalIPs(); len(ips) > 0 {
		info.LocalIPs = ips
	}
	return info
}

func fetchPublicIP() string {
	out, err := exec.Command("curl", "-4", "-s", "--max-time", "3", "https://ipinfo.io/ip").Output()
	if err == nil && len(out) > 0 {
		return strings.TrimSpace(string(out))
	}
	return ""
}

func fetchLocalIPs() []string {
	out, err := exec.Command("hostname", "-I").Output()
	if err != nil {
		return nil
	}
	parts := strings.Fields(string(out))
	var ips []string
	for _, p := range parts {
		if strings.Contains(p, ".") && !strings.HasPrefix(p, "127.") {
			ips = append(ips, p)
		}
	}
	return ips
}

// ── Non-interactive install ─────────────────────────────────────────

func nonInteractiveInstall() error {
	cfg := InstallConfig{
		Port:     valueOrEnv(flagPort, "LUI_PORT", defaultPort),
		BasePath: valueOrEnv(flagBasePath, "LUI_BASE_PATH", "/"),
		Username: valueOrEnv(flagUsername, "LUI_USERNAME", "admin"),
		Password: valueOrEnv(flagPassword, "LUI_PASSWORD", ""),
		Domain:   valueOrEnv(flagDomain, "LUI_DOMAIN", ""),
		IP:       valueOrEnv(flagIP, "LUI_IP", ""),
		CertPath: valueOrEnv(flagCertPath, "LUI_CERT_PATH", ""),
		Tarball:  flagTarball,
	}
	// Determine SSL type from what's provided.
	if cfg.Domain != "" {
		cfg.SSLType = sslDomain
	} else if cfg.IP != "" {
		cfg.SSLType = sslIP
	} else if cfg.CertPath != "" {
		cfg.SSLType = sslCustom
	} else {
		cfg.SSLType = sslNone
	}
	// Auto-generate password if not set.
	if cfg.Password == "" {
		cfg.Password = randomString(24)
	}

	eng := NewEngine(cfg)
	fmt.Println("Installing l-ui...")
	result, err := eng.Run()
	if err != nil {
		return fmt.Errorf("installation failed: %w", err)
	}
	fmt.Printf("\n  ✓  L-UI is installed and running\n")
	fmt.Printf("  Access: %s\n", result.AccessURL)
	fmt.Printf("  User:   %s\n", result.Username)
	fmt.Printf("  Pass:   %s\n", result.Password)
	return nil
}

func valueOrEnv(cli, env, def string) string {
	if cli != "" {
		return cli
	}
	if v := os.Getenv(env); v != "" {
		return v
	}
	return def
}

// ── TUI install ─────────────────────────────────────────────────────

func tuiInstall() error {
	p := tea.NewProgram(newTUIModel())
	installProg = p
	m, err := p.Run()
	if err != nil {
		return fmt.Errorf("install wizard: %w", err)
	}
	model, ok := m.(tuiModel)
	if !ok || model.errMsg != "" {
		if model.errMsg != "" {
			return fmt.Errorf("installation failed: %s", model.errMsg)
		}
		return nil
	}
	return nil
}

// ── Thin shell helper: called by install.sh ─────────────────────────

// EnsureInstallFromTarball extracts a pre-downloaded tarball and runs
// the install wizard.  This is the entry point called by install.sh.
func EnsureInstallFromTarball(tarballPath string) error {
	destDir := config.GetBinFolderPath()
	if destDir == "" {
		destDir = "/usr/local/l-ui-hub"
	}
	// Create destination if needed
	if err := os.MkdirAll(destDir, 0755); err != nil {
		return fmt.Errorf("create %s: %w", destDir, err)
	}
	// Extract tarball
	cmd := exec.Command("tar", "-xzf", tarballPath, "-C", filepath.Dir(destDir))
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("extract tarball: %w\n%s", err, out)
	}
	// Run the install wizard
	return tuiInstall()
}
