package cmd

import (
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/drunkleen/l-ui/internal/config"
)

// ── Catppuccin Mocha colors ─────────────────────────────────────────

var (
	stBox      = lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).BorderForeground(lipgloss.Color("#585b70")).Padding(1, 2)
	stTitle    = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#89b4fa"))
	stSubtitle = lipgloss.NewStyle().Foreground(lipgloss.Color("#a6adc8"))
	stItem     = lipgloss.NewStyle().Foreground(lipgloss.Color("#cdd6f4")).Padding(0, 1)
	stSel      = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#89b4fa")).Padding(0, 1)
	stCursor   = lipgloss.NewStyle().Foreground(lipgloss.Color("#89b4fa"))
	stBack     = lipgloss.NewStyle().Foreground(lipgloss.Color("#f9e2af"))
	stInfo     = lipgloss.NewStyle().Foreground(lipgloss.Color("#a6adc8"))
	stErr      = lipgloss.NewStyle().Foreground(lipgloss.Color("#f38ba8"))
	stOK       = lipgloss.NewStyle().Foreground(lipgloss.Color("#a6e3a1"))
	stBread    = lipgloss.NewStyle().Foreground(lipgloss.Color("#6c7086"))
	stFooter   = lipgloss.NewStyle().Foreground(lipgloss.Color("#585b70"))
	stPrompt   = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#f9e2af"))
)

// ── Menu model ──────────────────────────────────────────────────────

type menuMode int

const (
	modeBrowse menuMode = iota
	modeInput
)

type menuItem struct {
	label string
	run   func() string // returns result message to display
}

type menuModel struct {
	title   string
	items   []menuItem
	cursor  int
	msg     string
	mode    menuMode
	prompt  string
	input   string
	onInput func(val string) string
}

type tuiModel struct {
	stack []menuModel
}

func newMenu(title string, items []menuItem) menuModel {
	return menuModel{title: title, items: items}
}

// ── Builders ────────────────────────────────────────────────────────

func buildMainMenu() []menuItem {
	return []menuItem{
		{label: "Service", run: nil},
		{label: "Settings", run: nil},
		{label: "Security", run: nil},
		{label: "System", run: nil},
		{label: "Install / Reinstall", run: showInstallGuide},
		{label: "Exit", run: func() string { os.Exit(0); return "" }},
	}
}

func buildServiceMenu() []menuItem {
	return []menuItem{
		{label: "← Back", run: nil},
		{label: "Start", run: actionSystemCtl("start")},
		{label: "Stop", run: actionSystemCtl("stop")},
		{label: "Restart", run: actionSystemCtl("restart")},
		{label: "Status", run: actionSystemCtl("status")},
		{label: "Enable (autostart)", run: actionSystemCtl("enable")},
		{label: "Disable (autostart)", run: actionSystemCtl("disable")},
		{label: "View journal logs (last 20 lines)", run: actionJournal},
	}
}

func buildSettingsMenu() []menuItem {
	return []menuItem{
		{label: "← Back", run: nil},
		{label: "View panel settings", run: actionShowSettings},
		{label: "View service status", run: actionSystemCtl("status")},
		{label: "View service logs", run: actionJournal},
		{label: "Change panel port", run: nil},
		{label: "Reset admin password", run: nil},
	}
}

func buildSecurityMenu() []menuItem {
	return []menuItem{
		{label: "← Back", run: nil},
		{label: "SSL certificate", run: nil},
		{label: "IP limit (fail2ban)", run: nil},
		{label: "Firewall (UFW)", run: nil},
	}
}

func buildSSLMenu() []menuItem {
	return []menuItem{
		{label: "← Back", run: nil},
		{label: "Show current certificate", run: actionCertInfo},
		{label: "Issue Let's Encrypt — domain", run: nil},
		{label: "Issue Let's Encrypt — IP", run: nil},
	}
}

func buildIPLimitMenu() []menuItem {
	return []menuItem{
		{label: "← Back", run: nil},
		{label: "Show fail2ban status", run: actionFail2ban},
		{label: "Install / configure", run: actionPrint("Run: l-ui iplimit install")},
		{label: "Remove", run: actionPrint("Run: l-ui iplimit remove")},
	}
}

func buildFirewallMenu() []menuItem {
	return []menuItem{
		{label: "← Back", run: nil},
		{label: "Show UFW status", run: actionUfwStatus},
		{label: "Allow port", run: nil},
		{label: "Deny port", run: nil},
		{label: "Delete rule", run: nil},
	}
}

func buildSystemMenu() []menuItem {
	return []menuItem{
		{label: "← Back", run: nil},
		{label: "Update panel", run: actionExec("l-ui", "update")},
		{label: "Uninstall panel", run: actionExec("l-ui", "uninstall")},
		{label: "Update geo files", run: actionExec("l-ui", "geo", "update")},
		{label: "BBR management", run: nil},
		{label: "PostgreSQL status", run: actionExec("l-ui", "postgres", "status")},
	}
}

func buildBBRMenu() []menuItem {
	return []menuItem{
		{label: "← Back", run: nil},
		{label: "Show BBR status", run: actionBBR},
		{label: "Enable BBR", run: actionExec("l-ui", "bbr", "enable")},
		{label: "Disable BBR", run: actionExec("l-ui", "bbr", "disable")},
	}
}

// ── Sub-menu routing ────────────────────────────────────────────────

var subMenus = map[string]func() []menuItem{
	"Service":             buildServiceMenu,
	"Settings":            buildSettingsMenu,
	"Security":            buildSecurityMenu,
	"SSL certificate":     buildSSLMenu,
	"IP limit (fail2ban)": buildIPLimitMenu,
	"Firewall (UFW)":      buildFirewallMenu,
	"System":              buildSystemMenu,
	"BBR management":      buildBBRMenu,
}

// Items that need interactive input — handled specially in Update
var inputItems = map[string]struct{}{
	"Change panel port":            {},
	"Reset admin password":         {},
	"Issue Let's Encrypt — domain": {},
	"Issue Let's Encrypt — IP":     {},
	"Allow port":                   {},
	"Deny port":                    {},
	"Delete rule":                  {},
}

// ── Action helpers ──────────────────────────────────────────────────

func actionPrint(msg string) func() string {
	return func() string { return msg }
}

func actionExec(name string, args ...string) func() string {
	return func() string {
		out, err := exec.Command(name, args...).CombinedOutput()
		if err != nil {
			return fmt.Sprintf("%s\n%s", strings.TrimSpace(string(out)), err.Error())
		}
		return strings.TrimSpace(string(out))
	}
}

func actionSystemCtl(action string) func() string {
	return func() string {
		return actionExec("systemctl", action, "l-ui")()
	}
}

func actionShowSettings() string {
	bin := config.GetBinFolderPath() + "/l-ui"
	if _, err := os.Stat(bin); os.IsNotExist(err) {
		return "Binary not found at " + bin
	}
	return actionExec(bin, "setting", "--show")()
}

func actionJournal() string {
	out, err := exec.Command("journalctl", "-u", "l-ui", "--no-pager", "-n", "20").CombinedOutput()
	if err != nil {
		return fmt.Sprintf("journalctl: %s", err.Error())
	}
	return strings.TrimSpace(string(out))
}

func actionCertInfo() string {
	certDir := config.GetDBFolderPath() + "/cert"
	if _, err := os.Stat(certDir + "/fullchain.pem"); os.IsNotExist(err) {
		return "No certificate installed yet."
	}
	out, _ := exec.Command("openssl", "x509", "-in", certDir+"/fullchain.pem", "-noout", "-subject", "-dates", "-issuer").CombinedOutput()
	if len(out) == 0 {
		return "Certificate file exists."
	}
	return strings.TrimSpace(string(out))
}

func actionFail2ban() string {
	out, _ := exec.Command("fail2ban-client", "status").CombinedOutput()
	if len(out) == 0 {
		return "fail2ban not installed"
	}
	return strings.TrimSpace(string(out))
}

func actionUfwStatus() string {
	out, _ := exec.Command("ufw", "status").CombinedOutput()
	if len(out) == 0 {
		return "ufw not installed"
	}
	return strings.TrimSpace(string(out))
}

func actionBBR() string {
	out, _ := exec.Command("sysctl", "net.ipv4.tcp_congestion_control").Output()
	val := strings.TrimSpace(string(out))
	if strings.Contains(val, "bbr") {
		return "✓ BBR is active\n" + val
	}
	return "✗ BBR is not active\n" + val
}

func showInstallGuide() string {
	return `To install or reinstall L-UI Hub:

  bash <(curl -Ls https://raw.githubusercontent.com/drunkleen/l-ui/master/install.sh)

Or if the binary is already downloaded:

  l-ui install`
}

func isDocker() bool {
	if os.Getenv("LUI_IN_DOCKER") == "true" {
		return true
	}
	if _, err := os.Stat("/.dockerenv"); err == nil {
		return true
	}
	return false
}

// ── Interactive input handlers ──────────────────────────────────────

func handleInputChangePort(val string) string {
	port, err := strconv.Atoi(strings.TrimSpace(val))
	if err != nil || port < 1 || port > 65535 {
		return stErr.Render("Invalid port. Must be 1-65535.")
	}
	return actionExec("l-ui", "setting", "--port", strconv.Itoa(port))()
}

func handleInputResetPassword(val string) string {
	pw := strings.TrimSpace(val)
	if len(pw) < 6 {
		return stErr.Render("Password must be at least 6 characters.")
	}
	return actionExec("l-ui", "setting", "--password", pw)()
}

func handleInputIssueDomain(val string) string {
	domain := strings.TrimSpace(val)
	if domain == "" {
		return stErr.Render("Domain cannot be empty.")
	}
	out, err := exec.Command("l-ui", "ssl", "issue", "--domain", domain).CombinedOutput()
	if err != nil {
		return fmt.Sprintf("SSL issue failed: %s\n%s", err.Error(), strings.TrimSpace(string(out)))
	}
	return strings.TrimSpace(string(out))
}

func handleInputIssueIP(val string) string {
	ip := strings.TrimSpace(val)
	if ip == "" {
		return stErr.Render("IP cannot be empty.")
	}
	out, err := exec.Command("l-ui", "ssl", "issue-ip", "--ip", ip).CombinedOutput()
	if err != nil {
		return fmt.Sprintf("SSL issue-ip failed: %s\n%s", err.Error(), strings.TrimSpace(string(out)))
	}
	return strings.TrimSpace(string(out))
}

func handleInputUfwAllow(val string) string {
	port := strings.TrimSpace(val)
	if port == "" {
		return stErr.Render("Port cannot be empty.")
	}
	out, err := exec.Command("ufw", "allow", port).CombinedOutput()
	if err != nil {
		return fmt.Sprintf("ufw allow failed: %s\n%s", err.Error(), strings.TrimSpace(string(out)))
	}
	return strings.TrimSpace(string(out))
}

func handleInputUfwDeny(val string) string {
	port := strings.TrimSpace(val)
	if port == "" {
		return stErr.Render("Port cannot be empty.")
	}
	out, err := exec.Command("ufw", "deny", port).CombinedOutput()
	if err != nil {
		return fmt.Sprintf("ufw deny failed: %s\n%s", err.Error(), strings.TrimSpace(string(out)))
	}
	return strings.TrimSpace(string(out))
}

func handleInputUfwDelete(val string) string {
	rule := strings.TrimSpace(val)
	if rule == "" {
		return stErr.Render("Rule number cannot be empty.")
	}
	out, err := exec.Command("ufw", "--force", "delete", rule).CombinedOutput()
	if err != nil {
		return fmt.Sprintf("ufw delete failed: %s\n%s", err.Error(), strings.TrimSpace(string(out)))
	}
	return strings.TrimSpace(string(out))
}

// ── Bubble Tea model ────────────────────────────────────────────────

func (m tuiModel) current() *menuModel {
	if len(m.stack) == 0 {
		return nil
	}
	return &m.stack[len(m.stack)-1]
}

func (m tuiModel) Init() tea.Cmd { return nil }

func (m tuiModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	cur := m.current()
	if cur == nil {
		return m, tea.Quit
	}

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		return m, nil

	case tea.KeyMsg:
		// ── Input mode ──────────────────────────────────
		if cur.mode == modeInput {
			switch msg.String() {
			case "enter":
				result := cur.onInput(cur.input)
				cur.msg = result
				cur.mode = modeBrowse
				cur.input = ""
				return m, nil
			case "esc":
				cur.mode = modeBrowse
				cur.input = ""
				return m, nil
			case "backspace":
				if len(cur.input) > 0 {
					cur.input = cur.input[:len(cur.input)-1]
				}
				return m, nil
			default:
				if msg.Type == tea.KeyRunes {
					cur.input += string(msg.Runes)
				}
				return m, nil
			}
		}

		// ── Browse mode ─────────────────────────────────
		switch msg.String() {
		case "ctrl+c", "q":
			return m, tea.Quit

		case "up", "k":
			cur.cursor--
			if cur.cursor < 0 {
				cur.cursor = len(cur.items) - 1
			}
			cur.msg = ""
			return m, nil

		case "down", "j":
			cur.cursor++
			if cur.cursor >= len(cur.items) {
				cur.cursor = 0
			}
			cur.msg = ""
			return m, nil

		case "enter", " ":
			item := cur.items[cur.cursor]
			if item.label == "← Back" || (item.label == "Exit" && len(m.stack) <= 1) {
				if len(m.stack) <= 1 {
					return m, tea.Quit
				}
				m.stack = m.stack[:len(m.stack)-1]
				return m, nil
			}
			// Interactive input items
			if _, needsInput := inputItems[item.label]; needsInput {
				cur.mode = modeInput
				cur.input = ""
				switch item.label {
				case "Change panel port":
					cur.prompt = "Enter new panel port:"
					cur.onInput = handleInputChangePort
				case "Reset admin password":
					cur.prompt = "Enter new admin password:"
					cur.onInput = handleInputResetPassword
				case "Issue Let's Encrypt — domain":
					cur.prompt = "Enter domain name:"
					cur.onInput = handleInputIssueDomain
				case "Issue Let's Encrypt — IP":
					cur.prompt = "Enter IP address:"
					cur.onInput = handleInputIssueIP
				case "Allow port":
					cur.prompt = "Enter port to allow:"
					cur.onInput = handleInputUfwAllow
				case "Deny port":
					cur.prompt = "Enter port to deny:"
					cur.onInput = handleInputUfwDeny
				case "Delete rule":
					cur.prompt = "Enter rule number to delete:"
					cur.onInput = handleInputUfwDelete
				}
				return m, nil
			}
			if item.run != nil {
				cur.msg = item.run()
				return m, nil
			}
			// Sub-menu
			if builder, ok := subMenus[item.label]; ok {
				items := builder()
				m.stack = append(m.stack, newMenu(item.label, items))
			}
			return m, nil

		case "esc", "backspace":
			if len(m.stack) > 1 {
				m.stack = m.stack[:len(m.stack)-1]
			}
			return m, nil
		}
	}
	return m, nil
}

// ── View ────────────────────────────────────────────────────────────

func (m tuiModel) View() string {
	cur := m.current()
	if cur == nil {
		return ""
	}

	var b strings.Builder

	// ── Header ──────────────────────────────────────────────
	version := config.GetVersion()
	name := config.GetName()
	b.WriteString(stTitle.Render("⇒ "+name+" Hub ─── "+version+" ───") + "\n\n")

	// Breadcrumb
	if len(m.stack) > 1 {
		var crumbs []string
		for i, s := range m.stack {
			if i == 0 {
				crumbs = append(crumbs, stBread.Render("Menu"))
			} else if i == len(m.stack)-1 {
				crumbs = append(crumbs, stTitle.Render(s.title))
			} else {
				crumbs = append(crumbs, stBread.Render(s.title))
			}
		}
		b.WriteString("  " + strings.Join(crumbs, stBread.Render(" › ")) + "\n\n")
	}

	// ── Input mode ──────────────────────────────────────────
	if cur.mode == modeInput {
		b.WriteString(stPrompt.Render("▸ "+cur.prompt) + "\n\n")
		b.WriteString(stBox.Render("  "+cur.input+"█") + "\n\n")
		b.WriteString(stFooter.Render("  Enter=confirm · Esc=cancel"))
		return b.String()
	}

	// ── Menu items ─────────────────────────────────────────
	var items strings.Builder
	for i, item := range cur.items {
		cursor := "  "
		style := stItem
		if i == cur.cursor {
			cursor = stCursor.Render("▸ ")
			style = stSel
		}
		label := item.label
		if strings.HasPrefix(label, "←") {
			style = stBack
			if i != cur.cursor {
				cursor = "  "
			}
		}
		if item.label != "← Back" && len(m.stack) > 1 {
			label = "   " + label
		}
		items.WriteString(fmt.Sprintf("%s%s\n", cursor, style.Render(label)))
	}
	b.WriteString(stBox.Render(strings.TrimRight(items.String(), "\n")) + "\n")

	// ── Action result ───────────────────────────────────────
	if cur.msg != "" {
		b.WriteString("\n" + stBox.Render(cur.msg) + "\n")
	}

	// ── Footer ──────────────────────────────────────────────
	b.WriteString(stFooter.Render("\n  ↑↓ navigate · Enter select · Esc back · q/Ctrl+C quit"))

	return b.String()
}

// ── Entry point ─────────────────────────────────────────────────────

func runMenu() int {
	main := newMenu("Main Menu", buildMainMenu())
	m := tuiModel{stack: []menuModel{main}}
	p := tea.NewProgram(m)
	if _, err := p.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
	}
	return 0
}
