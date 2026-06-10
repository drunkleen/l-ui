package cmd

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/charmbracelet/lipgloss"
	"github.com/spf13/cobra"
)

var (
	// Colors
	sslGreenStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("#00FF7F"))
	sslRedStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("#FF6B6B"))
	sslYellowStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("#FFD700"))
	sslCyanStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("#00CED1"))

	// acmeShPath is the path to the acme.sh binary
	acmeShPath = filepath.Join(os.Getenv("HOME"), ".acme.sh", "acme.sh")

	// acmeShInstalledFunc allows overriding in tests
	acmeShInstalledFunc = acmeShInstalledImpl
)

func acmeShInstalledImpl() bool {
	if _, err := os.Stat(acmeShPath); err == nil {
		return true
	}
	return false
}

// --- SSL Command (parent) ---

var sslCmd = &cobra.Command{
	Use:   "ssl",
	Short: "SSL certificate management via acme.sh",
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println(sslCyanStyle.Render("ℹ  SSL Certificate Management"))
		fmt.Println()
		fmt.Println("Available subcommands:")
		fmt.Println("  " + sslGreenStyle.Render("ssl issue --domain example.com") + "     Issue domain certificate (Let's Encrypt)")
		fmt.Println("  " + sslGreenStyle.Render("ssl issue-ip --ip 1.2.3.4") + "       Issue IP certificate")
		fmt.Println("  " + sslGreenStyle.Render("ssl issue-cf --domain example.com") + "  Issue certificate via Cloudflare DNS")
		fmt.Println("  " + sslGreenStyle.Render("ssl show") + "                        Show current certificate paths")
		fmt.Println("  " + sslGreenStyle.Render("ssl renew") + "                        Force renew certificate")
		fmt.Println()
		fmt.Println("Install acme.sh first if not already installed:")
		fmt.Println("  curl https://get.acme.sh | sh")
	},
}

// --- SSL Issue (domain) ---

var sslIssueCmd = &cobra.Command{
	Use:   "issue --domain <domain>",
	Short: "Issue SSL certificate for domain via Let's Encrypt",
	Run:   runSslIssue,
}

var sslDomain string

func init() {
	sslIssueCmd.Flags().StringVar(&sslDomain, "domain", "", "Domain name for certificate")
	sslCmd.AddCommand(sslIssueCmd)
}

func runSslIssue(cmd *cobra.Command, args []string) {
	if sslDomain == "" {
		fmt.Println(sslRedStyle.Render("✖  Error: --domain flag is required"))
		fmt.Println("Usage: l-ui ssl issue --domain example.com")
		return
	}

	if err := installAcmeShIfMissing(); err != nil {
		fmt.Println(sslRedStyle.Render("✖  " + err.Error()))
		return
	}

	fmt.Println(sslCyanStyle.Render("ℹ  Issuing certificate for domain: " + sslDomain))

	stopPanelService()

	// Use standalone mode (HTTP-01 challenge on port 80) with Let's Encrypt RSA.
	acmeCmd := []string{"--issue", "--standalone", "-d", sslDomain, "--server", "letsencrypt", "--keylength", "2048"}
	out, err := execCommand(acmeShPath, acmeCmd...).CombinedOutput()
	if err != nil {
		fmt.Println(sslRedStyle.Render("✖  Failed to issue certificate:"))
		fmt.Println(string(out))
		startPanelService()
		return
	}

	fmt.Println(sslGreenStyle.Render("✓  Certificate issued successfully"))

	if err := copyCertToPanelDir(sslDomain); err != nil {
		fmt.Println(sslRedStyle.Render("✖  " + err.Error()))
	}

	startPanelService()
}

// --- SSL Issue IP ---

var sslIssueIPCmd = &cobra.Command{
	Use:   "issue-ip --ip <IP>",
	Short: "Issue SSL certificate for IP via acme.sh shortlived profile",
	Run:   runSslIssueIP,
}

var sslIP string

func init() {
	sslIssueIPCmd.Flags().StringVar(&sslIP, "ip", "", "IP address for certificate")
	sslCmd.AddCommand(sslIssueIPCmd)
}

func runSslIssueIP(cmd *cobra.Command, args []string) {
	if sslIP == "" {
		fmt.Println(sslRedStyle.Render("✖  Error: --ip flag is required"))
		fmt.Println("Usage: l-ui ssl issue-ip --ip 1.2.3.4")
		return
	}

	if err := installAcmeShIfMissing(); err != nil {
		fmt.Println(sslRedStyle.Render("✖  " + err.Error()))
		return
	}

	fmt.Println(sslCyanStyle.Render("ℹ  Issuing IP certificate for: " + sslIP))

	stopPanelService()

	// acme.sh supports IP certificates via the --ip flag
	out, err := execCommand(acmeShPath, "--issue", "--standalone", "-d", sslIP, "--ip", sslIP, "--server", "letsencrypt", "--keylength", "2048").CombinedOutput()
	if err != nil {
		fmt.Println(sslRedStyle.Render("✖  Failed to issue IP certificate:"))
		fmt.Println(string(out))
		startPanelService()
		return
	}

	fmt.Println(sslGreenStyle.Render("✓  IP certificate issued successfully"))

	if err := copyCertToPanelDir(sslIP); err != nil {
		fmt.Println(sslRedStyle.Render("✖  " + err.Error()))
	}

	startPanelService()
}

// --- SSL Issue Cloudflare ---

var sslIssueCFCmd = &cobra.Command{
	Use:   "issue-cf --domain <domain>",
	Short: "Issue SSL certificate via Cloudflare DNS",
	Run:   runSslIssueCF,
}

func init() {
	sslIssueCFCmd.Flags().StringVar(&sslDomain, "domain", "", "Domain name for certificate")
	sslCmd.AddCommand(sslIssueCFCmd)
}

func runSslIssueCF(cmd *cobra.Command, args []string) {
	if sslDomain == "" {
		fmt.Println(sslRedStyle.Render("✖  Error: --domain flag is required"))
		fmt.Println("Usage: l-ui ssl issue-cf --domain example.com")
		return
	}

	if err := installAcmeShIfMissing(); err != nil {
		fmt.Println(sslRedStyle.Render("✖  " + err.Error()))
		return
	}

	fmt.Println(sslCyanStyle.Render("ℹ  Issuing certificate via Cloudflare DNS for: " + sslDomain))

	// acme.sh Cloudflare DNS API mode
	out, err := execCommand(acmeShPath, "--issue", "--dns", "dns_cf", "-d", sslDomain, "--server", "letsencrypt", "--keylength", "2048").CombinedOutput()
	if err != nil {
		fmt.Println(sslRedStyle.Render("✖  Failed to issue Cloudflare certificate:"))
		fmt.Println(string(out))
		return
	}

	fmt.Println(sslGreenStyle.Render("✓  Cloudflare certificate issued successfully"))

	if err := copyCertToPanelDir(sslDomain); err != nil {
		fmt.Println(sslRedStyle.Render("✖  " + err.Error()))
	}
}

// --- SSL Show ---

var sslShowCmd = &cobra.Command{
	Use:   "show",
	Short: "Show current certificate paths from settings",
	Run:   runSslShow,
}

func init() {
	sslCmd.AddCommand(sslShowCmd)
}

func runSslShow(cmd *cobra.Command, args []string) {
	GetCertificate(true)
}

// --- SSL Renew ---

var sslRenewCmd = &cobra.Command{
	Use:   "renew",
	Short: "Force renew certificate via acme.sh",
	Run:   runSslRenew,
}

func init() {
	sslCmd.AddCommand(sslRenewCmd)
}

func runSslRenew(cmd *cobra.Command, args []string) {
	if !acmeShInstalledFunc() {
		fmt.Println(sslRedStyle.Render("✖  acme.sh is not installed"))
		fmt.Println()
		fmt.Println("Please install acme.sh first:")
		fmt.Println("  " + sslYellowStyle.Render("curl https://get.acme.sh | sh"))
		fmt.Println()
		fmt.Println("Or manually install to: ~/.acme.sh/acme.sh")
		return
	}

	fmt.Println(sslCyanStyle.Render("ℹ  Forcing certificate renewal..."))

	out, err := execCommand(acmeShPath, "--renew", "-d", sslDomain).CombinedOutput()
	if err != nil {
		fmt.Println(sslRedStyle.Render("✖  Failed to renew certificate:"))
		fmt.Println(string(out))
		return
	}

	fmt.Println(sslGreenStyle.Render("✓  Certificate renewal completed"))
	fmt.Println(string(out))
}

// --- Cert Command (original) ---

var certCmd = &cobra.Command{
	Use:   "cert",
	Short: "Configure panel certificate paths",
	Run: func(cmd *cobra.Command, args []string) {
		if settingReset {
			updateCert("", "")
		} else {
			updateCert(settingWebCert, settingWebCertKey)
		}
	},
}

// --- Helpers for TUI cert issuance ---

// installAcmeShIfMissing installs acme.sh if not already present.
func installAcmeShIfMissing() error {
	if acmeShInstalledFunc() {
		return nil
	}
	fmt.Println(sslCyanStyle.Render("ℹ  Installing acme.sh..."))
	cmd := exec.Command("curl", "-fsSL", "https://get.acme.sh", "-o", "/tmp/acme.sh")
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("download acme.sh failed: %w\n%s", err, out)
	}
	install := exec.Command("sh", "/tmp/acme.sh")
	if out, err := install.CombinedOutput(); err != nil {
		return fmt.Errorf("install acme.sh failed: %w\n%s", err, out)
	}
	exec.Command(acmeShPath, "--set-default-ca", "--server", "letsencrypt").Run()
	return nil
}

// stopPanelService stops the l-ui systemd service.
func stopPanelService() {
	fmt.Println(sslCyanStyle.Render("ℹ  Stopping l-ui service to free port 80..."))
	if out, err := exec.Command("systemctl", "stop", "l-ui").CombinedOutput(); err != nil {
		fmt.Println(sslYellowStyle.Render("⚠  Could not stop l-ui service:", err.Error(), string(out)))
	}
}

// startPanelService starts the l-ui systemd service.
func startPanelService() {
	fmt.Println(sslCyanStyle.Render("ℹ  Starting l-ui service..."))
	if out, err := exec.Command("systemctl", "start", "l-ui").CombinedOutput(); err != nil {
		fmt.Println(sslYellowStyle.Render("⚠  Could not start l-ui service:", err.Error(), string(out)))
	}
}

// copyCertToPanelDir copies the issued cert from ~/.acme.sh/<name>/ to
// /etc/l-ui/cert/ and configures the panel to use it.
func copyCertToPanelDir(name string) error {
	home, err := os.UserHomeDir()
	if err != nil {
		home = "/root"
	}
	srcDir := filepath.Join(home, ".acme.sh", name)

	certDir := "/etc/l-ui/cert"
	if err := os.MkdirAll(certDir, 0700); err != nil {
		return fmt.Errorf("create %s: %w", certDir, err)
	}

	// acme.sh stores the cert as fullchain.cer (or fullchain.pem) and <name>.key.
	srcFullchain := filepath.Join(srcDir, "fullchain.cer")
	if _, err := os.Stat(srcFullchain); os.IsNotExist(err) {
		srcFullchain = filepath.Join(srcDir, "fullchain.pem")
	}
	srcKey := filepath.Join(srcDir, name+".key")

	dstFullchain := filepath.Join(certDir, "fullchain.pem")
	dstKey := filepath.Join(certDir, "privkey.pem")

	if err := copyFile(srcFullchain, dstFullchain); err != nil {
		return fmt.Errorf("copy cert: %w", err)
	}
	fmt.Println(sslGreenStyle.Render("✓  Certificate copied to", dstFullchain))

	if err := copyFile(srcKey, dstKey); err != nil {
		return fmt.Errorf("copy key: %w", err)
	}
	fmt.Println(sslGreenStyle.Render("✓  Private key copied to", dstKey))

	// Configure panel to use the new cert.
	updateCert(dstFullchain, dstKey)
	return nil
}

// copyFile copies a file from src to dst, creating directories as needed.
func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	if err := os.MkdirAll(filepath.Dir(dst), 0700); err != nil {
		return err
	}
	out, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer out.Close()
	_, err = io.Copy(out, in)
	return err
}

func init() {
	addSharedSettingFlags(certCmd)
	rootCmd.AddCommand(certCmd)
	rootCmd.AddCommand(sslCmd)
}
