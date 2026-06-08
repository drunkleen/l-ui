package install

import (
	"crypto/rand"
	"math/big"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// ── Catppuccin Mocha colors ─────────────────────────────────────────

var (
	stBox      = lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).BorderForeground(lipgloss.Color("#585b70")).Padding(0, 2)
	stTitle    = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#89b4fa"))
	stStepNow  = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#89b4fa"))
	stStepDone = lipgloss.NewStyle().Foreground(lipgloss.Color("#a6e3a1"))
	stStepWait = lipgloss.NewStyle().Foreground(lipgloss.Color("#7f849c"))
	stLabel    = lipgloss.NewStyle().Foreground(lipgloss.Color("#cdd6f4"))
	stDefault  = lipgloss.NewStyle().Foreground(lipgloss.Color("#a6adc8"))
	stInput    = lipgloss.NewStyle().Foreground(lipgloss.Color("#f9e2af"))
	stHint     = lipgloss.NewStyle().Foreground(lipgloss.Color("#a6adc8"))
	stErr      = lipgloss.NewStyle().Foreground(lipgloss.Color("#f38ba8"))
	stOK       = lipgloss.NewStyle().Foreground(lipgloss.Color("#a6e3a1"))
	stFooter   = lipgloss.NewStyle().Foreground(lipgloss.Color("#bac2de"))
	stAccent   = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#89b4fa"))
	stSelOpt   = lipgloss.NewStyle().Foreground(lipgloss.Color("#89b4fa")).Bold(true)
	stSelDesc  = lipgloss.NewStyle().Foreground(lipgloss.Color("#a6adc8"))
)

// ── Steps ───────────────────────────────────────────────────────────

type wizardStep int

const (
	stepPort wizardStep = iota
	stepBasePath
	stepSSLSelect
	stepSSLValue
	stepCredentials
	stepInstall
	stepSummary
)

var stepNames = map[wizardStep]string{
	stepPort:        "Port",
	stepBasePath:    "Base path",
	stepSSLSelect:   "SSL",
	stepSSLValue:    "SSL detail",
	stepCredentials: "Login",
	stepInstall:     "Install",
	stepSummary:     "Done",
}

// ── SSL types ───────────────────────────────────────────────────────

type sslType int

const (
	sslNone   sslType = 0
	sslDomain sslType = 1
	sslIP     sslType = 2
	sslCustom sslType = 3
)

func sslLabel(t sslType) string {
	switch t {
	case sslNone:
		return "None"
	case sslDomain:
		return "Domain"
	case sslIP:
		return "IP"
	case sslCustom:
		return "Custom"
	}
	return ""
}

var sslOptions = []struct {
	t    sslType
	desc string
}{
	{sslNone, "No SSL — HTTP only"},
	{sslDomain, "Let's Encrypt — domain"},
	{sslIP, "Let's Encrypt — IP"},
	{sslCustom, "Custom certificate"},
}

// ── Form data ───────────────────────────────────────────────────────

type installForm struct {
	port     string
	basePath string
	ssl      sslType
	sslValue string
	username string
	password string
}

// ── Async messages ─────────────────────────────────────────────────

type progressMsg struct {
	step string
}

type installDoneMsg struct {
	result *InstallResult
	err    error
}

type quitMsg struct{}

// installProg is set in tuiInstall() so the install goroutine can send progress messages.
var installProg *tea.Program

func startInstall(cfg InstallConfig) tea.Cmd {
	return func() tea.Msg {
		eng := NewEngine(cfg, func(step string) {
			if installProg != nil {
				installProg.Send(progressMsg{step: step})
			}
		})
		result, err := eng.Run()
		return installDoneMsg{result: result, err: err}
	}
}

// ── Model ───────────────────────────────────────────────────────────

type tuiModel struct {
	step        wizardStep
	form        installForm
	input       string
	sslCursor   int
	credField   int
	sys         systemInfo
	errMsg      string
	result      *InstallResult
	currentStep string
}

func newTUIModel() tuiModel {
	info := detectSystem()
	localIP := ""
	if len(info.LocalIPs) > 0 {
		localIP = info.LocalIPs[0]
	}
	return tuiModel{
		step: stepPort,
		form: installForm{
			port:     defaultPort,
			basePath: "/",
			ssl:      sslNone,
			sslValue: localIP,
			username: randomString(8),
			password: randomString(24),
		},
		sys: info,
	}
}

func randomString(n int) string {
	const chars = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
	b := make([]byte, n)
	for i := range b {
		idx, _ := rand.Int(rand.Reader, big.NewInt(int64(len(chars))))
		b[i] = chars[idx.Int64()]
	}
	return string(b)
}

func (m tuiModel) Init() tea.Cmd { return nil }

// ── Helpers ─────────────────────────────────────────────────────────

func (m tuiModel) currentHint() string {
	switch m.step {
	case stepPort:
		return "1-65535"
	case stepBasePath:
		return "e.g. / or /panel"
	case stepSSLValue:
		switch m.form.ssl {
		case sslDomain:
			return "e.g. panel.example.com"
		case sslIP:
			return "Leave empty for auto-detect"
		case sslCustom:
			return "Directory containing fullchain.pem + privkey.pem"
		}
	}
	return ""
}

// ── Validation ──────────────────────────────────────────────────────

func (m tuiModel) validateCurrent() string {
	val := strings.TrimSpace(m.input)
	switch m.step {
	case stepPort:
		if val == "" {
			return "" // empty = use default
		}
		for _, c := range val {
			if c < '0' || c > '9' {
				return "Port must be a number (digits only)"
			}
		}
		p := 0
		for _, c := range val {
			p = p*10 + int(c-'0')
		}
		if p < 1 || p > 65535 {
			return "Port must be between 1 and 65535"
		}
	case stepSSLValue:
		if m.form.ssl == sslDomain && val == "" {
			return "Domain is required when SSL type is Domain"
		}
	}
	return ""
}

// ── Commit input ───────────────────────────────────────────────────

func (m *tuiModel) commitCredField() {
	val := strings.TrimSpace(m.input)
	if val == "" {
		return
	}
	if m.credField == 0 {
		m.form.username = val
	} else {
		m.form.password = val
	}
	m.input = ""
}

func (m *tuiModel) commitInput() {
	val := strings.TrimSpace(m.input)
	if val == "" {
		switch m.step {
		case stepPort:
			val = m.form.port
		case stepBasePath:
			val = m.form.basePath
		case stepSSLValue:
			val = m.form.sslValue
		}
	}
	switch m.step {
	case stepPort:
		if val != "" {
			m.form.port = val
		}
	case stepBasePath:
		if val != "" {
			if !strings.HasPrefix(val, "/") {
				val = "/" + val
			}
			if !strings.HasSuffix(val, "/") {
				val += "/"
			}
			m.form.basePath = val
		}
	case stepSSLValue:
		if val != "" {
			m.form.sslValue = val
		}
	}
	m.input = ""
}

// ── Step navigation ────────────────────────────────────────────────

func (m tuiModel) nextStep() wizardStep {
	switch m.step {
	case stepPort:
		return stepBasePath
	case stepBasePath:
		return stepSSLSelect
	case stepSSLSelect:
		if m.form.ssl != sslNone {
			return stepSSLValue
		}
		return stepCredentials
	case stepSSLValue:
		return stepCredentials
	case stepCredentials:
		return stepInstall
	}
	return stepSummary
}

func (m tuiModel) prevStep() wizardStep {
	switch m.step {
	case stepPort:
		return stepPort
	case stepBasePath:
		return stepPort
	case stepSSLSelect:
		return stepBasePath
	case stepSSLValue:
		return stepSSLSelect
	case stepCredentials:
		if m.form.ssl != sslNone {
			return stepSSLValue
		}
		return stepSSLSelect
	}
	return stepPort
}

// ── Update ──────────────────────────────────────────────────────────

func (m tuiModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case progressMsg:
		m.currentStep = msg.step
		return m, nil

	case installDoneMsg:
		if msg.err != nil {
			m.errMsg = msg.err.Error()
			m.step = stepSummary
			return m, nil
		}
		m.result = msg.result
		m.step = stepSummary
		return m, tea.Tick(2*time.Second, func(t time.Time) tea.Msg {
			return quitMsg{}
		})

	case quitMsg:
		return m, tea.Quit

	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c":
			return m, tea.Quit

		// ── Navigation ──────────────────────────────────────
		case "up":
			if m.step == stepSSLSelect {
				m.sslCursor--
				if m.sslCursor < 0 {
					m.sslCursor = len(sslOptions) - 1
				}
				return m, nil
			}
			if m.step == stepCredentials {
				m.commitCredField()
				m.credField = 0
				return m, nil
			}
			return m, nil

		case "down":
			if m.step == stepSSLSelect {
				m.sslCursor++
				if m.sslCursor >= len(sslOptions) {
					m.sslCursor = 0
				}
				return m, nil
			}
			if m.step == stepCredentials {
				m.commitCredField()
				m.credField = 1
				return m, nil
			}
			return m, nil

		case "tab":
			if m.step == stepCredentials {
				m.commitCredField()
				if m.credField == 0 {
					m.credField = 1
				} else {
					m.credField = 0
				}
				return m, nil
			}
			return m, nil

		case "shift+tab":
			if m.step == stepCredentials {
				m.commitCredField()
				if m.credField == 1 {
					m.credField = 0
				}
				return m, nil
			}
			return m, nil

		// ── Enter ───────────────────────────────────────────
		case "enter":
			if m.step == stepInstall {
				return m, nil
			}
			if m.step == stepSummary {
				return m, tea.Quit
			}

			if m.step == stepSSLSelect {
				m.form.ssl = sslOptions[m.sslCursor].t
				m.errMsg = ""
				next := m.nextStep()
				if next == stepInstall {
					m.step = stepInstall
					return m, startInstall(configFromForm(m.form, m.sys))
				}
				m.step = next
				return m, nil
			}

			if m.step == stepCredentials {
				if m.credField == 0 {
					val := strings.TrimSpace(m.input)
					if val != "" {
						if len(val) < 4 {
							m.errMsg = "Username must be at least 4 characters"
							return m, nil
						}
						m.form.username = val
					}
					m.input = ""
					m.credField = 1
					return m, nil
				}
				val := strings.TrimSpace(m.input)
				if val != "" {
					if len(val) < 8 {
						m.errMsg = "Password must be at least 8 characters"
						return m, nil
					}
					m.form.password = val
				}
				m.input = ""
				m.credField = 0
				m.step = stepInstall
				return m, startInstall(configFromForm(m.form, m.sys))
			}

			if errMsg := m.validateCurrent(); errMsg != "" {
				m.errMsg = errMsg
				return m, nil
			}
			m.errMsg = ""
			m.commitInput()
			next := m.nextStep()
			if next == stepInstall {
				m.step = stepInstall
				return m, startInstall(configFromForm(m.form, m.sys))
			}
			m.step = next
			return m, nil

		// ── Space selects in SSL list, inserts space elsewhere ─
		case " ":
			if m.step == stepSSLSelect {
				m.form.ssl = sslOptions[m.sslCursor].t
				m.errMsg = ""
				next := m.nextStep()
				if next == stepInstall {
					m.step = stepInstall
					return m, startInstall(configFromForm(m.form, m.sys))
				}
				m.step = next
				return m, nil
			}
			m.input += " "
			m.errMsg = ""
			return m, nil

		// ── Esc: go back one step ───────────────────────────
		case "esc":
			if m.step == stepPort {
				return m, nil
			}
			if m.step == stepInstall {
				return m, nil
			}
			if m.step == stepCredentials {
				m.commitCredField()
				if m.credField == 1 {
					m.credField = 0
					return m, nil
				}
			}
			m.step = m.prevStep()
			m.errMsg = ""
			// Restore previous value into input so it looks editable
			switch m.step {
			case stepPort:
				m.input = m.form.port
			case stepBasePath:
				m.input = m.form.basePath
			case stepSSLSelect:
				for i, opt := range sslOptions {
					if opt.t == m.form.ssl {
						m.sslCursor = i
						break
					}
				}
			case stepSSLValue:
				m.input = m.form.sslValue
			case stepCredentials:
				m.input = ""
			}
			return m, nil

		// ── Backspace: delete char, never navigate ──────────
		case "backspace":
			if len(m.input) > 0 {
				m.input = m.input[:len(m.input)-1]
				m.errMsg = ""
			}
			return m, nil

		// ── Typing ──────────────────────────────────────────
		default:
			if msg.Type != tea.KeyRunes || len(msg.Runes) == 0 {
				return m, nil
			}
			for _, ch := range msg.Runes {
				switch {
				case m.step == stepPort && (ch < '0' || ch > '9'):
					continue
				case m.step == stepCredentials && m.credField == 0:
					if !((ch >= 'a' && ch <= 'z') || (ch >= 'A' && ch <= 'Z') || (ch >= '0' && ch <= '9')) {
						continue
					}
					if len(m.input) >= 32 {
						continue
					}
				case m.step == stepCredentials && m.credField == 1:
					if len(m.input) >= 64 {
						continue
					}
				}
				m.input += string(ch)
				m.errMsg = ""
			}
			return m, nil
		}
	}
	return m, nil
}

func configFromForm(f installForm, sys systemInfo) InstallConfig {
	cfg := InstallConfig{
		Port:     f.port,
		BasePath: f.basePath,
		Username: f.username,
		Password: f.password,
		Tarball:  flagTarball,
	}
	switch f.ssl {
	case sslDomain:
		cfg.SSLType = sslDomain
		cfg.Domain = f.sslValue
	case sslIP:
		cfg.SSLType = sslIP
		cfg.IP = f.sslValue
		if cfg.IP == "" {
			cfg.IP = sys.PublicIP
		}
		if cfg.IP == "" && len(sys.LocalIPs) > 0 {
			cfg.IP = sys.LocalIPs[0]
		}
	case sslCustom:
		cfg.SSLType = sslCustom
		cfg.CertPath = f.sslValue
	}
	return cfg
}

// ── View ────────────────────────────────────────────────────────────

func (m tuiModel) View() string {
	var b strings.Builder

	b.WriteString(stTitle.Render("⇒ L-UI Installation Wizard") + "\n\n")
	b.WriteString(m.renderProgress())
	b.WriteString("\n")

	card := m.renderCard()
	if m.errMsg != "" {
		card += "\n" + stErr.Render("⚠ "+m.errMsg)
	}
	b.WriteString(stBox.Render(card) + "\n")
	b.WriteString(stFooter.Render("Enter=next  ↑↓=navigate  Tab=switch  Esc=back  Ctrl+C=quit"))

	return b.String()
}

func (m tuiModel) renderProgress() string {
	var b strings.Builder
	steps := []wizardStep{stepPort, stepBasePath, stepSSLSelect, stepCredentials, stepInstall, stepSummary}
	if m.form.ssl != sslNone {
		steps = []wizardStep{stepPort, stepBasePath, stepSSLSelect, stepSSLValue, stepCredentials, stepInstall, stepSummary}
	}
	for _, s := range steps {
		name := stepNames[s]
		if s == m.step {
			b.WriteString(stStepNow.Render("●"))
		} else if s < m.step {
			b.WriteString(stStepDone.Render("✓"))
		} else {
			b.WriteString(stStepWait.Render("○"))
		}
		b.WriteString(" ")
		if s == m.step {
			b.WriteString(stStepNow.Render(name))
		} else if s < m.step {
			b.WriteString(stStepDone.Render(name))
		} else {
			b.WriteString(stStepWait.Render(name))
		}
		if s != stepSummary {
			b.WriteString(stStepWait.Render("  "))
		}
	}
	return b.String()
}

func (m tuiModel) renderCard() string {
	switch m.step {
	case stepSSLSelect:
		return m.renderSSLSelect()
	case stepCredentials:
		return m.renderCredFields()
	case stepSummary:
		return m.renderSummary()
	case stepInstall:
		return m.renderInstalling()
	default:
		return m.renderTextField()
	}
}

func (m tuiModel) renderTextField() string {
	var b strings.Builder

	prompt := ""
	def := ""
	hint := m.currentHint()
	switch m.step {
	case stepPort:
		prompt = "Panel port"
		def = m.form.port
	case stepBasePath:
		prompt = "Web base path"
		def = m.form.basePath
	case stepSSLValue:
		switch m.form.ssl {
		case sslDomain:
			prompt = "Domain name"
			def = m.form.sslValue
		case sslIP:
			prompt = "IP address"
			def = m.form.sslValue
		case sslCustom:
			prompt = "Certificate path"
			def = m.form.sslValue
		}
	}

	b.WriteString(stLabel.Render(prompt) + "\n")

	if m.input == "" {
		if hint != "" {
			b.WriteString(stDefault.Render(def + "  " + stHint.Render("("+hint+")")))
		} else {
			b.WriteString(stDefault.Render(def))
		}
		b.WriteString(stAccent.Render("█"))
	} else {
		b.WriteString(stInput.Render(m.input))
	}

	return b.String()
}

func (m tuiModel) renderSSLSelect() string {
	var b strings.Builder
	b.WriteString(stLabel.Render("SSL type") + "\n\n")

	for i, opt := range sslOptions {
		cursor := "  "
		style := stSelDesc
		if i == m.sslCursor {
			cursor = stSelOpt.Render("➤ ")
			style = stSelOpt
		}
		b.WriteString(cursor + style.Render(opt.desc) + "\n")
	}

	b.WriteString("\n" + stHint.Render("↑↓ to navigate · Enter/Space to select"))
	return b.String()
}

func (m tuiModel) renderCredFields() string {
	var b strings.Builder

	unameLabel := "Admin username"
	unameVal := m.form.username
	if m.credField == 0 {
		b.WriteString(stSelOpt.Render("➤ ") + stLabel.Render(unameLabel) + "\n")
		if m.input == "" {
			b.WriteString("   " + stDefault.Render(unameVal) + stAccent.Render("█") + "\n")
		} else {
			b.WriteString("   " + stInput.Render(m.input) + "\n")
		}
	} else {
		b.WriteString("  " + stLabel.Render(unameLabel) + "\n")
		b.WriteString("   " + stInput.Render(unameVal) + "\n")
	}

	b.WriteString("\n")

	pwLabel := "Admin password"
	pwVal := m.form.password
	if m.credField == 1 {
		b.WriteString(stSelOpt.Render("➤ ") + stLabel.Render(pwLabel) + "\n")
		if m.input == "" {
			b.WriteString("   " + stDefault.Render(pwVal) + stAccent.Render("█") + "\n")
		} else {
			b.WriteString("   " + stInput.Render(m.input) + "\n")
		}
	} else {
		b.WriteString("  " + stLabel.Render(pwLabel) + "\n")
		b.WriteString("   " + stInput.Render(pwVal) + "\n")
	}

	b.WriteString("\n" + stHint.Render("Tab/↑↓ to switch fields · Enter password field to continue"))
	return b.String()
}

func (m tuiModel) renderInstalling() string {
	var b strings.Builder
	b.WriteString(stHint.Render("Installing...") + "\n")
	if m.currentStep != "" {
		b.WriteString("\n" + stDefault.Render("→ "+m.currentStep))
	}
	return b.String()
}

func (m tuiModel) renderSummary() string {
	var b strings.Builder
	r := m.result
	if r == nil {
		return ""
	}
	b.WriteString(stOK.Render("✓ L-UI is installed and running") + "\n\n")
	b.WriteString(stLabel.Render("Access: ") + stInput.Render(r.AccessURL) + "\n")
	b.WriteString(stLabel.Render("User:   ") + stInput.Render(r.Username) + "\n")
	b.WriteString(stLabel.Render("Pass:   ") + stInput.Render(r.Password) + "\n")
	return b.String()
}
