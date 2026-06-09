package cmd

import (
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"
)

var geoTargetDir = "/usr/local/l-ui-hub"

// --- Geo ---

var geoCmd = &cobra.Command{
	Use:   "geo",
	Short: "Geo file management",
}

var geoUpdateCmd = &cobra.Command{
	Use:   "update",
	Short: "Update geoip.dat and geosite.dat from GitHub",
	Run:   runGeoUpdate,
}

func runGeoUpdate(cmd *cobra.Command, args []string) {
	fmt.Println(cyanStyle.Render("ℹ  Updating geo files..."))

	// Ensure target directory exists
	if err := os.MkdirAll(geoTargetDir, 0755); err != nil {
		fmt.Println(redStyle.Render("✖  Failed to create directory:"))
		fmt.Println(err.Error())
		return
	}

	files := map[string]string{
		"geoip.dat":   "https://github.com/Loyalsoldier/v2ray-rules-dat/releases/latest/download/geoip.dat",
		"geosite.dat": "https://github.com/Loyalsoldier/v2ray-rules-dat/releases/latest/download/geosite.dat",
	}

	for filename, url := range files {
		filepath := geoTargetDir + "/" + filename
		fmt.Printf(yellowStyle.Render("↓  Downloading %s...\n"), filename)

		out, err := execCommand("curl", "-L", "-f", "--progress-bar", url, "-o", filepath).CombinedOutput()
		if err != nil {
			fmt.Printf(redStyle.Render("✖  Failed to download %s:\n"), filename)
			fmt.Println(string(out))
			// Try to clean up partial file
			os.Remove(filepath)
			continue
		}

		// Verify file was created and has content
		info, err := os.Stat(filepath)
		if err != nil || info.Size() == 0 {
			fmt.Printf(redStyle.Render("✖  %s is empty or invalid\n"), filename)
			os.Remove(filepath)
			continue
		}

		fmt.Printf(greenStyle.Render("✓  %s updated (%d bytes)\n"), filename, info.Size())
	}

	fmt.Println(greenStyle.Render("✓  Geo update complete"))
}

// --- BBR ---

var bbrCmd = &cobra.Command{
	Use:   "bbr",
	Short: "BBR TCP congestion control",
}

var bbrEnableCmd = &cobra.Command{
	Use:   "enable",
	Short: "Enable BBR congestion control",
	Run:   runBBREnable,
}

var bbrDisableCmd = &cobra.Command{
	Use:   "disable",
	Short: "Disable BBR (revert to CUBIC)",
	Run:   runBBRDisable,
}

func runBBREnable(cmd *cobra.Command, args []string) {
	// Check current status
	current, err := checkBBRStatus()
	if err != nil {
		fmt.Println(yellowStyle.Render("⚠  Could not determine current BBR status"))
	}

	if current == "bbr" {
		fmt.Println(greenStyle.Render("✓  BBR is already enabled"))
		return
	}

	// Enable fq qdisc
	fmt.Println(cyanStyle.Render("ℹ  Enabling net.core.default_qdisc=fq..."))
	out, err := execCommand("sysctl", "-w", "net.core.default_qdisc=fq").CombinedOutput()
	if err != nil {
		fmt.Println(redStyle.Render("✖  Failed to set qdisc:"))
		fmt.Println(string(out))
		return
	}

	// Enable bbr
	fmt.Println(cyanStyle.Render("ℹ  Enabling net.ipv4.tcp_congestion_control=bbr..."))
	out, err = execCommand("sysctl", "-w", "net.ipv4.tcp_congestion_control=bbr").CombinedOutput()
	if err != nil {
		fmt.Println(redStyle.Render("✖  Failed to enable BBR:"))
		fmt.Println(string(out))
		return
	}

	fmt.Println(greenStyle.Render("✓  BBR enabled successfully"))
	fmt.Println(cyanStyle.Render("ℹ  Changes take effect immediately. Add to /etc/sysctl.conf for persistence."))
}

func runBBRDisable(cmd *cobra.Command, args []string) {
	// Check current status
	current, err := checkBBRStatus()
	if err != nil {
		fmt.Println(yellowStyle.Render("⚠  Could not determine current BBR status"))
	}

	if current == "cubic" {
		fmt.Println(greenStyle.Render("✓  BBR is already disabled (CUBIC is active)"))
		return
	}

	// Disable BBR by switching to CUBIC
	fmt.Println(cyanStyle.Render("ℹ  Switching to CUBIC congestion control..."))
	out, err := execCommand("sysctl", "-w", "net.ipv4.tcp_congestion_control=cubic").CombinedOutput()
	if err != nil {
		fmt.Println(redStyle.Render("✖  Failed to disable BBR:"))
		fmt.Println(string(out))
		return
	}

	fmt.Println(greenStyle.Render("✓  BBR disabled, CUBIC is now active"))
}

func checkBBRStatus() (string, error) {
	out, err := execCommand("sysctl", "net.ipv4.tcp_congestion_control").Output()
	if err != nil {
		return "", err
	}
	output := strings.TrimSpace(string(out))
	if strings.Contains(output, "bbr") {
		return "bbr", nil
	}
	return "cubic", nil
}

// --- Firewall ---

var firewallCmd = &cobra.Command{
	Use:   "firewall",
	Short: "UFW firewall management",
}

var firewallStatusCmd = &cobra.Command{
	Use:   "status",
	Short: "Show UFW firewall status",
	Run:   runFirewallStatus,
}

var firewallEnableCmd = &cobra.Command{
	Use:   "enable",
	Short: "Enable UFW firewall",
	Run:   runFirewallEnable,
}

var firewallDisableCmd = &cobra.Command{
	Use:   "disable",
	Short: "Disable UFW firewall",
	Run:   runFirewallDisable,
}

var firewallAllowCmd = &cobra.Command{
	Use:   "allow",
	Short: "Allow port through firewall",
	Args:  cobra.ExactArgs(1),
	Run:   runFirewallAllow,
}

var firewallDenyCmd = &cobra.Command{
	Use:   "deny",
	Short: "Deny port through firewall",
	Args:  cobra.ExactArgs(1),
	Run:   runFirewallDeny,
}

func checkUFWInstalled() bool {
	_, err := execCommand("which", "ufw").Output()
	return err == nil
}

func runFirewallStatus(cmd *cobra.Command, args []string) {
	if !checkUFWInstalled() {
		fmt.Println(yellowStyle.Render("⚠  UFW is not installed. Install with: apt install ufw"))
		return
	}
	out, err := execCommand("ufw", "status").CombinedOutput()
	if err != nil {
		fmt.Println(redStyle.Render("✖  Failed to get firewall status:"))
		fmt.Println(string(out))
		return
	}
	fmt.Println(string(out))
}

func runFirewallEnable(cmd *cobra.Command, args []string) {
	if !checkUFWInstalled() {
		fmt.Println(yellowStyle.Render("⚠  UFW is not installed. Install with: apt install ufw"))
		return
	}
	fmt.Println(yellowStyle.Render("⚠  Enabling firewall may lock you out if SSH is not allowed!"))
	out, err := execCommand("ufw", "enable").CombinedOutput()
	if err != nil {
		fmt.Println(redStyle.Render("✖  Failed to enable firewall:"))
		fmt.Println(string(out))
		return
	}
	fmt.Println(greenStyle.Render("✓  Firewall enabled"))
	fmt.Println(string(out))
}

func runFirewallDisable(cmd *cobra.Command, args []string) {
	if !checkUFWInstalled() {
		fmt.Println(yellowStyle.Render("⚠  UFW is not installed. Install with: apt install ufw"))
		return
	}
	out, err := execCommand("ufw", "disable").CombinedOutput()
	if err != nil {
		fmt.Println(redStyle.Render("✖  Failed to disable firewall:"))
		fmt.Println(string(out))
		return
	}
	fmt.Println(greenStyle.Render("✓  Firewall disabled"))
	fmt.Println(string(out))
}

func runFirewallAllow(cmd *cobra.Command, args []string) {
	port := args[0]
	if !checkUFWInstalled() {
		fmt.Println(yellowStyle.Render("⚠  UFW is not installed. Install with: apt install ufw"))
		return
	}
	out, err := execCommand("ufw", "allow", port).CombinedOutput()
	if err != nil {
		fmt.Println(redStyle.Render("✖  Failed to allow port:"))
		fmt.Println(string(out))
		return
	}
	fmt.Printf(greenStyle.Render("✓  Port %s allowed\n"), port)
	fmt.Println(string(out))
}

func runFirewallDeny(cmd *cobra.Command, args []string) {
	port := args[0]
	if !checkUFWInstalled() {
		fmt.Println(yellowStyle.Render("⚠  UFW is not installed. Install with: apt install ufw"))
		return
	}
	out, err := execCommand("ufw", "deny", port).CombinedOutput()
	if err != nil {
		fmt.Println(redStyle.Render("✖  Failed to deny port:"))
		fmt.Println(string(out))
		return
	}
	fmt.Printf(greenStyle.Render("✓  Port %s denied\n"), port)
	fmt.Println(string(out))
}

// --- IP Limit (fail2ban) ---

var iplimitCmd = &cobra.Command{
	Use:   "iplimit",
	Short: "fail2ban IP ban management",
}

var iplimitStatusCmd = &cobra.Command{
	Use:   "status",
	Short: "Show fail2ban status",
	Run:   runIPLimitStatus,
}

var iplimitEnableCmd = &cobra.Command{
	Use:   "enable",
	Short: "Enable/start fail2ban",
	Run:   runIPLimitEnable,
}

var iplimitDisableCmd = &cobra.Command{
	Use:   "disable",
	Short: "Disable/stop fail2ban",
	Run:   runIPLimitDisable,
}

var iplimitBanCmd = &cobra.Command{
	Use:   "ban",
	Short: "Ban an IP address",
	Args:  cobra.ExactArgs(1),
	Run:   runIPLimitBan,
}

var iplimitUnbanCmd = &cobra.Command{
	Use:   "unban",
	Short: "Unban an IP address",
	Args:  cobra.ExactArgs(1),
	Run:   runIPLimitUnban,
}

func checkFail2banInstalled() bool {
	_, err := execCommand("which", "fail2ban-client").Output()
	return err == nil
}

func runIPLimitStatus(cmd *cobra.Command, args []string) {
	if !checkFail2banInstalled() {
		fmt.Println(yellowStyle.Render("⚠  fail2ban is not installed. Install with: apt install fail2ban"))
		return
	}
	out, err := execCommand("fail2ban-client", "status").CombinedOutput()
	if err != nil {
		fmt.Println(redStyle.Render("✖  Failed to get fail2ban status:"))
		fmt.Println(string(out))
		return
	}
	fmt.Println(string(out))
}

func runIPLimitEnable(cmd *cobra.Command, args []string) {
	if !checkFail2banInstalled() {
		fmt.Println(yellowStyle.Render("⚠  fail2ban is not installed. Install with: apt install fail2ban"))
		return
	}
	out, err := execCommand("fail2ban-client", "start").CombinedOutput()
	if err != nil {
		fmt.Println(redStyle.Render("✖  Failed to start fail2ban:"))
		fmt.Println(string(out))
		return
	}
	fmt.Println(greenStyle.Render("✓  fail2ban started"))
	fmt.Println(string(out))
}

func runIPLimitDisable(cmd *cobra.Command, args []string) {
	if !checkFail2banInstalled() {
		fmt.Println(yellowStyle.Render("⚠  fail2ban is not installed. Install with: apt install fail2ban"))
		return
	}
	out, err := execCommand("fail2ban-client", "stop").CombinedOutput()
	if err != nil {
		fmt.Println(redStyle.Render("✖  Failed to stop fail2ban:"))
		fmt.Println(string(out))
		return
	}
	fmt.Println(greenStyle.Render("✓  fail2ban stopped"))
	fmt.Println(string(out))
}

func runIPLimitBan(cmd *cobra.Command, args []string) {
	ip := args[0]
	if !checkFail2banInstalled() {
		fmt.Println(yellowStyle.Render("⚠  fail2ban is not installed. Install with: apt install fail2ban"))
		return
	}
	out, err := execCommand("fail2ban-client", "set", "jailname", "banip", ip).CombinedOutput()
	if err != nil {
		fmt.Println(redStyle.Render("✖  Failed to ban IP:"))
		fmt.Println(string(out))
		return
	}
	fmt.Printf(greenStyle.Render("✓  IP %s banned\n"), ip)
	fmt.Println(string(out))
}

func runIPLimitUnban(cmd *cobra.Command, args []string) {
	ip := args[0]
	if !checkFail2banInstalled() {
		fmt.Println(yellowStyle.Render("⚠  fail2ban is not installed. Install with: apt install fail2ban"))
		return
	}
	out, err := execCommand("fail2ban-client", "set", "jailname", "unbanip", ip).CombinedOutput()
	if err != nil {
		fmt.Println(redStyle.Render("✖  Failed to unban IP:"))
		fmt.Println(string(out))
		return
	}
	fmt.Printf(greenStyle.Render("✓  IP %s unbanned\n"), ip)
	fmt.Println(string(out))
}

// --- PostgreSQL ---

var postgresCmd = &cobra.Command{
	Use:   "postgres",
	Short: "PostgreSQL management",
}

var postgresInstallCmd = &cobra.Command{
	Use:   "install",
	Short: "Install PostgreSQL",
	Run:   runPostgresInstall,
}

var postgresStatusCmd = &cobra.Command{
	Use:   "status",
	Short: "Check PostgreSQL status",
	Run:   runPostgresStatus,
}

var postgresMigrateCmd = &cobra.Command{
	Use:   "migrate",
	Short: "Run database migration",
	Run:   runPostgresMigrate,
}

var postgresEnvCmd = &cobra.Command{
	Use:   "env",
	Short: "Generate PostgreSQL env file template",
	Run:   runPostgresEnv,
}

func detectPackageManager() string {
	// Try apt
	_, err := execCommand("apt", "--version").Output()
	if err == nil {
		return "apt"
	}
	// Try yum
	_, err = execCommand("yum", "--version").Output()
	if err == nil {
		return "yum"
	}
	// Try dnf
	_, err = execCommand("dnf", "--version").Output()
	if err == nil {
		return "dnf"
	}
	// Try apk
	_, err = execCommand("apk", "--version").Output()
	if err == nil {
		return "apk"
	}
	return ""
}

func runPostgresInstall(cmd *cobra.Command, args []string) {
	pm := detectPackageManager()
	if pm == "" {
		fmt.Println(redStyle.Render("✖  Could not detect package manager (apt/yum/dnf/apk)"))
		return
	}

	fmt.Printf(cyanStyle.Render("ℹ  Detected package manager: %s\n"), pm)

	var installCmd []string
	switch pm {
	case "apt":
		installCmd = []string{"apt", "update", "&&", "apt", "install", "-y", "postgresql", "postgresql-contrib"}
	case "yum":
		installCmd = []string{"yum", "install", "-y", "postgresql-server", "postgresql-contrib"}
	case "dnf":
		installCmd = []string{"dnf", "install", "-y", "postgresql-server", "postgresql-contrib"}
	case "apk":
		installCmd = []string{"apk", "add", "postgresql", "postgresql-client"}
	}

	fmt.Println(yellowStyle.Render("⚠  Installing PostgreSQL may require root/sudo access"))

	// Run installation based on package manager
	switch pm {
	case "apt":
		out, err := execCommand("sh", "-c", strings.Join(installCmd, " ")).CombinedOutput()
		if err != nil {
			fmt.Println(redStyle.Render("✖  Failed to install PostgreSQL:"))
			fmt.Println(string(out))
			return
		}
	case "yum", "dnf":
		out, err := execCommand(installCmd[0], installCmd[1:]...).CombinedOutput()
		if err != nil {
			fmt.Println(redStyle.Render("✖  Failed to install PostgreSQL:"))
			fmt.Println(string(out))
			return
		}
	case "apk":
		out, err := execCommand(installCmd[0], installCmd[1:]...).CombinedOutput()
		if err != nil {
			fmt.Println(redStyle.Render("✖  Failed to install PostgreSQL:"))
			fmt.Println(string(out))
			return
		}
	}

	fmt.Println(greenStyle.Render("✓  PostgreSQL installed"))
	fmt.Println(cyanStyle.Render("ℹ  Initialize with: postgresql-setup initdb (EL systems)"))
	fmt.Println(cyanStyle.Render("ℹ  Start with: systemctl start postgresql (EL) or systemctl start postgresql (Debian/Ubuntu)"))
}

func runPostgresStatus(cmd *cobra.Command, args []string) {
	// Try to connect to PostgreSQL
	out, err := execCommand("pg_isready").Output()
	if err != nil {
		fmt.Println(redStyle.Render("✖  PostgreSQL is not running"))
		fmt.Println(cyanStyle.Render("ℹ  Try: systemctl start postgresql"))
		return
	}
	fmt.Println(greenStyle.Render("✓  PostgreSQL is running"))
	fmt.Println(string(out))
}

func runPostgresMigrate(cmd *cobra.Command, args []string) {
	fmt.Println(cyanStyle.Render("ℹ  Running database migration..."))
	fmt.Println(yellowStyle.Render("⚠  Ensure LUI_DB_TYPE=postgres and LUI_DB_DSN are set"))

	// Run the migrate command if the environment is configured
	out, err := execCommand("l-ui", "migrate-db").CombinedOutput()
	if err != nil {
		fmt.Println(redStyle.Render("✖  Migration failed:"))
		fmt.Println(string(out))
		return
	}
	fmt.Println(greenStyle.Render("✓  Migration complete"))
	fmt.Println(string(out))
}

func runPostgresEnv(cmd *cobra.Command, args []string) {
	envTemplate := `# PostgreSQL Configuration for L-UI
# Copy to /etc/l-ui.env or ~/.l-ui.env and customize

# Database type (must be 'postgres')
LUI_DB_TYPE=postgres

# PostgreSQL connection string
# Format: host=localhost user=postgres password=your_password dbname=l-ui port=5432
LUI_DB_DSN=host=localhost user=postgres password=your_password dbname=l-ui port=5432 sslmode=disable

# Optional: Connection pool settings
# LUI_DB_MAX_OPEN_CONNS=25
# LUI_DB_MAX_IDLE_CONNS=5
# LUI_DB_CONN_MAX_LIFETIME=5m
`
	fmt.Println("PostgreSQL environment template:")
	fmt.Println("---")
	fmt.Print(envTemplate)
	fmt.Println("---")
	fmt.Println(greenStyle.Render("✓  Template generated"))
	fmt.Println(cyanStyle.Render("ℹ  After configuring, restart l-ui service: systemctl restart l-ui"))
}

func init() {
	// Geo commands
	geoCmd.AddCommand(geoUpdateCmd)
	rootCmd.AddCommand(geoCmd)

	// BBR commands
	bbrCmd.AddCommand(bbrEnableCmd)
	bbrCmd.AddCommand(bbrDisableCmd)
	rootCmd.AddCommand(bbrCmd)

	// Firewall commands
	firewallCmd.AddCommand(firewallStatusCmd)
	firewallCmd.AddCommand(firewallEnableCmd)
	firewallCmd.AddCommand(firewallDisableCmd)
	firewallCmd.AddCommand(firewallAllowCmd)
	firewallCmd.AddCommand(firewallDenyCmd)
	rootCmd.AddCommand(firewallCmd)

	// IP limit commands
	iplimitCmd.AddCommand(iplimitStatusCmd)
	iplimitCmd.AddCommand(iplimitEnableCmd)
	iplimitCmd.AddCommand(iplimitDisableCmd)
	iplimitCmd.AddCommand(iplimitBanCmd)
	iplimitCmd.AddCommand(iplimitUnbanCmd)
	rootCmd.AddCommand(iplimitCmd)

	// PostgreSQL commands
	postgresCmd.AddCommand(postgresInstallCmd)
	postgresCmd.AddCommand(postgresStatusCmd)
	postgresCmd.AddCommand(postgresMigrateCmd)
	postgresCmd.AddCommand(postgresEnvCmd)
	rootCmd.AddCommand(postgresCmd)
}
