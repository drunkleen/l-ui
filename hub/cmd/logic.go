package cmd

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/drunkleen/l-ui/hub/sub"
	"github.com/drunkleen/l-ui/hub/web"
	"github.com/drunkleen/l-ui/hub/web/global"
	"github.com/drunkleen/l-ui/hub/web/service"
	"github.com/drunkleen/l-ui/internal/config"
	"github.com/drunkleen/l-ui/internal/database"
	"github.com/drunkleen/l-ui/internal/logger"
	"github.com/drunkleen/l-ui/internal/util/crypto"
	"github.com/drunkleen/l-ui/internal/util/sys"

	"github.com/charmbracelet/lipgloss"
	"github.com/joho/godotenv"
)

func runWebServer() {
	log.Printf("Starting %v %v", config.GetName(), config.GetVersion())

	switch config.GetLogLevel() {
	case config.Debug:
		logger.InitLogger("debug")
	case config.Info:
		logger.InitLogger("info")
	case config.Notice:
		logger.InitLogger("notice")
	case config.Warning:
		logger.InitLogger("warning")
	case config.Error:
		logger.InitLogger("error")
	default:
		log.Fatalf("Unknown log level: %v", config.GetLogLevel())
	}

	godotenv.Load()

	err := database.InitDB(config.GetDBPath())
	if err != nil {
		log.Fatalf("Error initializing database: %v", err)
	}

	var server *web.Server
	server = web.NewServer()
	global.SetWebServer(server)
	err = server.Start()
	if err != nil {
		log.Fatalf("Error starting web server: %v", err)
		return
	}

	var subServer *sub.Server
	sub.SetDistFS(web.EmbeddedDist())
	service.RegisterSubLinkProvider(sub.NewLinkProvider())
	subServer = sub.NewServer()
	global.SetSubServer(subServer)
	err = subServer.Start()
	if err != nil {
		log.Fatalf("Error starting sub server: %v", err)
		return
	}

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGHUP, syscall.SIGTERM, sys.SIGUSR1, os.Interrupt)
	global.SetRestartHook(func() {
		select {
		case sigCh <- syscall.SIGHUP:
		default:
		}
	})
	for {
		sig := <-sigCh

		switch sig {
		case syscall.SIGHUP:
			logger.Info("Received SIGHUP signal. Restarting servers...")

			err := server.StopPanelOnly()
			if err != nil {
				logger.Debug("Error stopping web server:", err)
			}
			err = subServer.Stop()
			if err != nil {
				logger.Debug("Error stopping sub server:", err)
			}

			server = web.NewServer()
			global.SetWebServer(server)
			err = server.StartPanelOnly()
			if err != nil {
				log.Fatalf("Error restarting web server: %v", err)
				return
			}
			log.Println("Web server restarted successfully.")

			sub.SetDistFS(web.EmbeddedDist())
			subServer = sub.NewServer()
			global.SetSubServer(subServer)
			err = subServer.Start()
			if err != nil {
				log.Fatalf("Error restarting sub server: %v", err)
				return
			}
			log.Println("Sub server restarted successfully.")
		default:
			service.StopBot()
			server.Stop()
			subServer.Stop()
			_ = database.CloseDB()
			log.Println("Shutting down servers.")
			return
		}
	}
}

func resetSetting() error {
	err := database.InitDB(config.GetDBPath())
	if err != nil {
		fmt.Println("Failed to initialize database:", err)
		return err
	}

	settingService := service.SettingService{}
	err = settingService.ResetSettings()
	if err != nil {
		fmt.Println("Failed to reset settings:", err)
		return err
	} else {
		fmt.Println("Settings successfully reset.")
	}
	return nil
}

var (
	settingGreenStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("#00FF7F"))
	settingCyanStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("#00CED1"))
	settingRedStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("#FF6B6B"))
	settingYellowStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("#FFD700"))
)

func showSetting(show bool) {
	if show {
		settingService := service.SettingService{}
		port, err := settingService.GetPort()
		if err != nil {
			fmt.Println(settingRedStyle.Render("✖ get current port failed:"), err)
		}

		webBasePath, err := settingService.GetBasePath()
		if err != nil {
			fmt.Println(settingRedStyle.Render("✖ get webBasePath failed:"), err)
		}

		certFile, err := settingService.GetCertFile()
		if err != nil {
			fmt.Println(settingRedStyle.Render("✖ get cert file failed:"), err)
		}
		keyFile, err := settingService.GetKeyFile()
		if err != nil {
			fmt.Println(settingRedStyle.Render("✖ get key file failed:"), err)
		}

		userService := service.UserService{}
		userModel, err := userService.GetFirstUser()
		if err != nil {
			fmt.Println(settingRedStyle.Render("✖ get current user info failed:"), err)
		}

		if userModel.Username == "" || userModel.Password == "" {
			fmt.Println(settingYellowStyle.Render("⚠ current username or password is empty"))
		}

		fmt.Println()
		fmt.Println(settingCyanStyle.Render("╭─ Current Panel Settings ─"))
		fmt.Println("│")

		if certFile == "" || keyFile == "" {
			fmt.Println(settingYellowStyle.Render("│ ⚠ Panel is not secure with SSL"))
		} else {
			fmt.Println(settingGreenStyle.Render("│ ✓ Panel is secure with SSL"))
		}

		hasDefaultCredential := func() bool {
			return userModel.Username == "admin" && crypto.CheckPasswordHash(userModel.Password, "admin")
		}()

		if hasDefaultCredential {
			fmt.Println(settingYellowStyle.Render("│ ⚠ hasDefaultCredential: true"))
		} else {
			fmt.Println(settingGreenStyle.Render("│ ✓ hasDefaultCredential: false"))
		}

		fmt.Printf("│ %s %s\n", settingCyanStyle.Render("port:"), settingGreenStyle.Render(fmt.Sprintf("%d", port)))
		fmt.Printf("│ %s %s\n", settingCyanStyle.Render("webBasePath:"), settingGreenStyle.Render(webBasePath))

		externalIP := detectExternalIP()
		if externalIP != "" {
			fmt.Printf("│ %s %s\n", settingCyanStyle.Render("externalIP:"), settingGreenStyle.Render(externalIP))
		}

		fmt.Println("│")
		fmt.Println(settingCyanStyle.Render("╰─────────────────────────────────"))
		fmt.Println()
	}
}

func updateTgbotEnableSts(status bool) {
	settingService := service.SettingService{}
	currentTgSts, err := settingService.GetTgbotEnabled()
	if err != nil {
		fmt.Println(err)
		return
	}
	logger.Infof("current enabletgbot status[%v],need update to status[%v]", currentTgSts, status)
	if currentTgSts != status {
		err := settingService.SetTgbotEnabled(status)
		if err != nil {
			fmt.Println(err)
			return
		} else {
			logger.Infof("SetTgbotEnabled[%v] success", status)
		}
	}
}

func updateTgbotSetting(tgBotToken string, tgBotChatid string, tgBotRuntime string) {
	err := database.InitDB(config.GetDBPath())
	if err != nil {
		fmt.Println("Error initializing database:", err)
		return
	}

	settingService := service.SettingService{}

	if tgBotToken != "" {
		err := settingService.SetTgBotToken(tgBotToken)
		if err != nil {
			fmt.Printf("Error setting Telegram bot token: %v\n", err)
			return
		}
		logger.Info("Successfully updated Telegram bot token.")
	}

	if tgBotRuntime != "" {
		err := settingService.SetTgbotRuntime(tgBotRuntime)
		if err != nil {
			fmt.Printf("Error setting Telegram bot runtime: %v\n", err)
			return
		}
		logger.Infof("Successfully updated Telegram bot runtime to [%s].", tgBotRuntime)
	}

	if tgBotChatid != "" {
		err := settingService.SetTgBotChatId(tgBotChatid)
		if err != nil {
			fmt.Printf("Error setting Telegram bot chat ID: %v\n", err)
			return
		}
		logger.Info("Successfully updated Telegram bot chat ID.")
	}
}

func updateSetting(port int, username string, password string, webBasePath string, listenIP string, resetTwoFactor bool) error {
	err := database.InitDB(config.GetDBPath())
	if err != nil {
		fmt.Println("Database initialization failed:", err)
		return err
	}

	settingService := service.SettingService{}
	userService := service.UserService{}

	if port > 0 {
		portCheck := validatePortAvailability(port)
		if portCheck.Status == "FAIL" {
			fmt.Println(settingRedStyle.Render("✖ Failed to set port:"), portCheck.Message)
		} else {
			err := settingService.SetPort(port)
			if err != nil {
				fmt.Println(settingRedStyle.Render("✖ Failed to set port:"), err)
			} else {
				fmt.Println(settingGreenStyle.Render("✓ Port set successfully:"), port)
			}
		}
	}

	if username != "" || password != "" {
		err := userService.UpdateFirstUser(username, password)
		if err != nil {
			fmt.Println(settingRedStyle.Render("✖ Failed to update username and password:"), err)
		} else {
			fmt.Println(settingGreenStyle.Render("✓ Username and password updated successfully"))
		}
	}

	if webBasePath != "" {
		err := settingService.SetBasePath(webBasePath)
		if err != nil {
			fmt.Println(settingRedStyle.Render("✖ Failed to set base URI path:"), err)
		} else {
			fmt.Println(settingGreenStyle.Render("✓ Base URI path set successfully"))
		}
	}

	if resetTwoFactor {
		err := settingService.SetTwoFactorEnable(false)

		if err != nil {
			fmt.Println(settingRedStyle.Render("✖ Failed to reset two-factor authentication:"), err)
		} else {
			settingService.SetTwoFactorToken("")
			fmt.Println(settingGreenStyle.Render("✓ Two-factor authentication reset successfully"))
		}
	}

	if listenIP != "" {
		ipCheck := validateListenIP(listenIP)
		if ipCheck.Status == "FAIL" {
			fmt.Println(settingRedStyle.Render("✖ Failed to set listen IP:"), ipCheck.Message)
		} else {
			err := settingService.SetListen(listenIP)
			if err != nil {
				fmt.Println(settingRedStyle.Render("✖ Failed to set listen IP:"), err)
			} else {
				fmt.Println(settingGreenStyle.Render("✓ listen"), listenIP, settingGreenStyle.Render("set successfully"))
			}
		}
	}

	return nil
}

func updateCert(publicKey string, privateKey string) {
	err := database.InitDB(config.GetDBPath())
	if err != nil {
		fmt.Println(err)
		return
	}

	if (privateKey != "" && publicKey != "") || (privateKey == "" && publicKey == "") {
		settingService := service.SettingService{}
		err = settingService.SetCertFile(publicKey)
		if err != nil {
			fmt.Println("set certificate public key failed:", err)
		} else {
			fmt.Println("set certificate public key success")
		}

		err = settingService.SetKeyFile(privateKey)
		if err != nil {
			fmt.Println("set certificate private key failed:", err)
		} else {
			fmt.Println("set certificate private key success")
		}

		err = settingService.SetSubCertFile(publicKey)
		if err != nil {
			fmt.Println("set certificate for subscription public key failed:", err)
		} else {
			fmt.Println("set certificate for subscription public key success")
		}

		err = settingService.SetSubKeyFile(privateKey)
		if err != nil {
			fmt.Println("set certificate for subscription private key failed:", err)
		} else {
			fmt.Println("set certificate for subscription private key success")
		}
	} else {
		fmt.Println("both public and private key should be entered.")
	}
}

func GetCertificate(getCert bool) {
	if getCert {
		settingService := service.SettingService{}
		certFile, err := settingService.GetCertFile()
		if err != nil {
			fmt.Println("get cert file failed, error info:", err)
		}
		keyFile, err := settingService.GetKeyFile()
		if err != nil {
			fmt.Println("get key file failed, error info:", err)
		}

		fmt.Println("cert:", certFile)
		fmt.Println("key:", keyFile)
	}
}

func GetListenIP(getListen bool) {
	if getListen {
		settingService := service.SettingService{}
		ListenIP, err := settingService.GetListen()
		if err != nil {
			log.Printf("Failed to retrieve listen IP: %v", err)
			return
		}

		fmt.Println()
		fmt.Println(settingCyanStyle.Render("╭─ Listen IP Configuration ─"))
		fmt.Println("│")
		fmt.Printf("│ %s %s\n", settingCyanStyle.Render("listenIP:"), settingGreenStyle.Render(ListenIP))

		externalIP := detectExternalIP()
		if externalIP != "" {
			fmt.Printf("│ %s %s\n", settingCyanStyle.Render("externalIP:"), settingGreenStyle.Render(externalIP))
		}

		fmt.Println("│")
		fmt.Println(settingCyanStyle.Render("╰─────────────────────────────────"))
		fmt.Println()
	}
}

func GetApiToken(getApiToken bool) {
	if !getApiToken {
		return
	}
	apiTokenService := service.ApiTokenService{}
	tokens, err := apiTokenService.List()
	if err != nil {
		fmt.Println("get apiToken failed, error info:", err)
		return
	}
	if len(tokens) > 0 {
		fmt.Println("apiToken:", tokens[0].Token)
		return
	}
	created, err := apiTokenService.Create("install")
	if err != nil {
		fmt.Println("create apiToken failed, error info:", err)
		return
	}
	fmt.Println("apiToken:", created.Token)
}

func migrateDb() {
	inboundService := service.InboundService{}

	err := database.InitDB(config.GetDBPath())
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println("Start migrating database...")
	inboundService.MigrateDB()
	fmt.Println("Migration done!")
}

func loadServiceEnvFile() {
	for _, path := range config.GetEnvFilePaths() {
		if _, err := os.Stat(path); err != nil {
			continue
		}
		if err := godotenv.Load(path); err != nil {
			log.Printf("warning: failed to load env file %s: %v", path, err)
		}
		return
	}
}

type doctorCheck struct {
	Name    string `json:"name"`
	Status  string `json:"status"`
	Message string `json:"message,omitempty"`
}

type doctorResult struct {
	Pass   bool          `json:"pass"`
	Checks []doctorCheck `json:"checks"`
}

const (
	colorGreen  = "\033[32m"
	colorRed    = "\033[31m"
	colorYellow = "\033[33m"
	colorReset  = "\033[0m"
)

func runDoctor(jsonOutput bool) bool {
	checks := []doctorCheck{}
	allPass := true

	dbKind := config.GetDBKind()
	var dbInitialized bool

	if dbKind == "sqlite" {
		dbPath := config.GetDBPath()
		if dbPath == "" {
			checks = append(checks, doctorCheck{Name: "Database", Status: "FAIL", Message: "DB path is empty"})
			allPass = false
		} else if _, err := os.Stat(dbPath); os.IsNotExist(err) {
			dir := filepath.Dir(dbPath)
			if err := os.MkdirAll(dir, 0750); err != nil {
				checks = append(checks, doctorCheck{Name: "Database", Status: "FAIL", Message: fmt.Sprintf("Cannot create DB directory %s: %v", dir, err)})
				allPass = false
			} else if f, err := os.Create(dbPath); err != nil {
				checks = append(checks, doctorCheck{Name: "Database", Status: "FAIL", Message: fmt.Sprintf("Cannot create DB file %s: %v", dbPath, err)})
				allPass = false
			} else {
				f.Close()
				os.Remove(dbPath)
				checks = append(checks, doctorCheck{Name: "Database", Status: "PASS", Message: "SQLite DB path is writable (will be created on first run)"})
			}
		} else if _, err := os.OpenFile(dbPath, os.O_RDWR, 0644); err != nil {
			checks = append(checks, doctorCheck{Name: "Database", Status: "FAIL", Message: fmt.Sprintf("DB file %s is not writable: %v", dbPath, err)})
			allPass = false
		} else {
			checks = append(checks, doctorCheck{Name: "Database", Status: "PASS", Message: fmt.Sprintf("SQLite DB at %s is accessible", dbPath)})
		}
	} else {
		dsn := config.GetDBDSN()
		if dsn == "" {
			checks = append(checks, doctorCheck{Name: "Database", Status: "FAIL", Message: fmt.Sprintf("LUI_DB_DSN is not set for %s", dbKind)})
			allPass = false
		} else if err := database.InitDB(dsn); err != nil {
			checks = append(checks, doctorCheck{Name: "Database", Status: "FAIL", Message: fmt.Sprintf("Cannot connect to %s: %v", dbKind, err)})
			allPass = false
		} else {
			defer database.CloseDB()
			checks = append(checks, doctorCheck{Name: "Database", Status: "PASS", Message: fmt.Sprintf("%s connection is valid", dbKind)})
			dbInitialized = true
		}
	}

	if dbInitialized {
		if portCheck := checkPort(); portCheck.Status == "FAIL" {
			allPass = false
			checks = append(checks, portCheck)
		} else {
			checks = append(checks, portCheck)
		}

		if certCheck := checkCertPaths(); certCheck.Status == "FAIL" {
			allPass = false
			checks = append(checks, certCheck)
		} else {
			checks = append(checks, certCheck)
		}

		if basePathCheck := checkWebBasePath(); basePathCheck.Status == "FAIL" {
			allPass = false
			checks = append(checks, basePathCheck)
		} else {
			checks = append(checks, basePathCheck)
		}
	} else {
		checks = append(checks, doctorCheck{Name: "Port", Status: "WARN", Message: "Skipped (DB not initialized)"})
		checks = append(checks, doctorCheck{Name: "Certificates", Status: "WARN", Message: "Skipped (DB not initialized)"})
		checks = append(checks, doctorCheck{Name: "WebBasePath", Status: "WARN", Message: "Skipped (DB not initialized)"})
	}

	if configCheck := checkConfigConsistency(); configCheck.Status == "FAIL" {
		allPass = false
		checks = append(checks, configCheck)
	} else {
		checks = append(checks, configCheck)
	}

	if jsonOutput {
		result := doctorResult{Pass: allPass, Checks: checks}
		data, err := json.MarshalIndent(result, "", "  ")
		if err != nil {
			fmt.Fprintf(os.Stderr, "failed to marshal JSON: %v\n", err)
			return false
		}
		fmt.Println(string(data))
	} else {
		fmt.Println("=== L-UI Configuration Doctor ===")
		fmt.Println()
		for _, check := range checks {
			status := colorGreen + "PASS" + colorReset
			if check.Status == "FAIL" {
				status = colorRed + "FAIL" + colorReset
			} else if check.Status == "WARN" {
				status = colorYellow + "WARN" + colorReset
			}
			fmt.Printf("[%s] %s\n", status, check.Name)
			if check.Message != "" {
				fmt.Printf("       %s\n", check.Message)
			}
		}
		fmt.Println()
		if allPass {
			fmt.Println(colorGreen + "All checks passed!" + colorReset)
		} else {
			fmt.Println(colorRed + "Some checks failed. Please review the issues above." + colorReset)
		}
	}

	return allPass
}

func checkDatabase() doctorCheck {
	dbKind := config.GetDBKind()

	if dbKind == "sqlite" {
		dbPath := config.GetDBPath()
		if dbPath == "" {
			return doctorCheck{Name: "Database", Status: "FAIL", Message: "DB path is empty"}
		}

		if _, err := os.Stat(dbPath); os.IsNotExist(err) {
			dir := filepath.Dir(dbPath)
			if err := os.MkdirAll(dir, 0750); err != nil {
				return doctorCheck{Name: "Database", Status: "FAIL", Message: fmt.Sprintf("Cannot create DB directory %s: %v", dir, err)}
			}
			if f, err := os.Create(dbPath); err != nil {
				return doctorCheck{Name: "Database", Status: "FAIL", Message: fmt.Sprintf("Cannot create DB file %s: %v", dbPath, err)}
			} else {
				f.Close()
				os.Remove(dbPath)
				return doctorCheck{Name: "Database", Status: "PASS", Message: "SQLite DB path is writable (will be created on first run)"}
			}
		}

		if _, err := os.OpenFile(dbPath, os.O_RDWR, 0644); err != nil {
			return doctorCheck{Name: "Database", Status: "FAIL", Message: fmt.Sprintf("DB file %s is not writable: %v", dbPath, err)}
		}

		return doctorCheck{Name: "Database", Status: "PASS", Message: fmt.Sprintf("SQLite DB at %s is accessible", dbPath)}
	}

	if dbKind == "postgres" || dbKind == "mysql" {
		dsn := config.GetDBDSN()
		if dsn == "" {
			return doctorCheck{Name: "Database", Status: "FAIL", Message: fmt.Sprintf("LUI_DB_DSN is not set for %s", dbKind)}
		}

		if err := database.InitDB(dsn); err != nil {
			return doctorCheck{Name: "Database", Status: "FAIL", Message: fmt.Sprintf("Cannot connect to %s: %v", dbKind, err)}
		}
		defer database.CloseDB()

		return doctorCheck{Name: "Database", Status: "PASS", Message: fmt.Sprintf("%s connection is valid", dbKind)}
	}

	return doctorCheck{Name: "Database", Status: "FAIL", Message: fmt.Sprintf("Unknown DB kind: %s", dbKind)}
}

func checkPort() doctorCheck {
	settingService := service.SettingService{}
	port, err := settingService.GetPort()
	if err != nil {
		return doctorCheck{Name: "Port", Status: "WARN", Message: fmt.Sprintf("Cannot get port setting: %v", err)}
	}

	if port <= 0 || port > 65535 {
		return doctorCheck{Name: "Port", Status: "FAIL", Message: fmt.Sprintf("Port %d is out of valid range (1-65535)", port)}
	}

	addr := fmt.Sprintf(":%d", port)
	ln, err := net.Listen("tcp", addr)
	if err != nil {
		return doctorCheck{Name: "Port", Status: "FAIL", Message: fmt.Sprintf("Port %d is already in use", port)}
	}
	ln.Close()

	return doctorCheck{Name: "Port", Status: "PASS", Message: fmt.Sprintf("Port %d is available", port)}
}

func checkCertPaths() doctorCheck {
	settingService := service.SettingService{}
	certFile, err := settingService.GetCertFile()
	if err != nil {
		return doctorCheck{Name: "Certificates", Status: "WARN", Message: fmt.Sprintf("Cannot get cert file setting: %v", err)}
	}

	keyFile, err := settingService.GetKeyFile()
	if err != nil {
		return doctorCheck{Name: "Certificates", Status: "WARN", Message: fmt.Sprintf("Cannot get key file setting: %v", err)}
	}

	if certFile == "" || keyFile == "" {
		return doctorCheck{Name: "Certificates", Status: "WARN", Message: "No TLS certificates configured (HTTP only)"}
	}

	if _, err := os.Stat(certFile); os.IsNotExist(err) {
		return doctorCheck{Name: "Certificates", Status: "FAIL", Message: fmt.Sprintf("Cert file not found: %s", certFile)}
	}

	if _, err := os.Stat(keyFile); os.IsNotExist(err) {
		return doctorCheck{Name: "Certificates", Status: "FAIL", Message: fmt.Sprintf("Key file not found: %s", keyFile)}
	}

	return doctorCheck{Name: "Certificates", Status: "PASS", Message: fmt.Sprintf("Cert and key files exist (%s, %s)", certFile, keyFile)}
}

func checkWebBasePath() doctorCheck {
	settingService := service.SettingService{}
	basePath, err := settingService.GetBasePath()
	if err != nil {
		return doctorCheck{Name: "WebBasePath", Status: "WARN", Message: fmt.Sprintf("Cannot get base path setting: %v", err)}
	}

	if basePath == "" {
		return doctorCheck{Name: "WebBasePath", Status: "FAIL", Message: "Base path is empty"}
	}

	if !strings.HasPrefix(basePath, "/") {
		return doctorCheck{Name: "WebBasePath", Status: "FAIL", Message: fmt.Sprintf("Base path must start with /, got: %s", basePath)}
	}

	if len(basePath) < 4 && basePath != "/" {
		return doctorCheck{Name: "WebBasePath", Status: "FAIL", Message: fmt.Sprintf("Base path must be at least 4 characters (or /), got: %s", basePath)}
	}

	if strings.Contains(basePath, " ") {
		return doctorCheck{Name: "WebBasePath", Status: "FAIL", Message: fmt.Sprintf("Base path cannot contain spaces: %s", basePath)}
	}

	return doctorCheck{Name: "WebBasePath", Status: "PASS", Message: fmt.Sprintf("Base path is valid: %s", basePath)}
}

func checkConfigConsistency() doctorCheck {
	dbKind := config.GetDBKind()
	dsn := config.GetDBDSN()

	if (dbKind == "postgres" || dbKind == "mysql") && dsn == "" {
		return doctorCheck{Name: "Config Consistency", Status: "FAIL", Message: "LUI_DB_TYPE is set to " + dbKind + " but LUI_DB_DSN is not set"}
	}

	if dbKind == "sqlite" && dsn != "" {
		return doctorCheck{Name: "Config Consistency", Status: "WARN", Message: "LUI_DB_DSN is set but LUI_DB_TYPE is sqlite (DSN will be ignored)"}
	}

	return doctorCheck{Name: "Config Consistency", Status: "PASS", Message: "Configuration is consistent"}
}

func detectExternalIP() string {
	ipServices := []string{
		"https://ipinfo.io/ip",
		"https://ifconfig.me/ip",
		"https://icanhazip.com",
	}

	client := &http.Client{
		Timeout: 5 * time.Second,
	}

	for _, serviceURL := range ipServices {
		resp, err := client.Get(serviceURL)
		if err != nil {
			continue
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			continue
		}

		body, err := io.ReadAll(resp.Body)
		if err != nil {
			continue
		}

		ip := strings.TrimSpace(string(body))
		parsedIP := net.ParseIP(ip)
		if parsedIP != nil {
			return ip
		}
	}

	return getLocalIPs()
}

func getLocalIPs() string {
	addrs, err := net.InterfaceAddrs()
	if err != nil {
		return ""
	}

	var ips []string
	for _, addr := range addrs {
		if ipNet, ok := addr.(*net.IPNet); ok && !ipNet.IP.IsLoopback() {
			ips = append(ips, ipNet.IP.String())
		}
	}

	if len(ips) > 0 {
		return strings.Join(ips, ", ")
	}
	return ""
}

func runSettingValidation() {
	fmt.Println()
	fmt.Println(settingCyanStyle.Render("╭─ Settings Validation ─"))
	fmt.Println("│")

	allPass := true

	portCheck := validatePortAvailability(settingPort)
	if settingPort > 0 {
		fmt.Printf("│ %s %s\n", settingCyanStyle.Render("port:"), formatValidationStatus(portCheck))
		if portCheck.Status != "PASS" {
			allPass = false
		}
	}

	listenIPCheck := validateListenIP(settingListenIP)
	if settingListenIP != "" {
		fmt.Printf("│ %s %s\n", settingCyanStyle.Render("listenIP:"), formatValidationStatus(listenIPCheck))
		if listenIPCheck.Status != "PASS" {
			allPass = false
		}
	}

	webBasePathCheck := validateWebBasePath(settingWebBasePath)
	if settingWebBasePath != "" {
		fmt.Printf("│ %s %s\n", settingCyanStyle.Render("webBasePath:"), formatValidationStatus(webBasePathCheck))
		if webBasePathCheck.Status != "PASS" {
			allPass = false
		}
	}

	usernameCheck := validateUsername(settingUsername)
	if settingUsername != "" {
		fmt.Printf("│ %s %s\n", settingCyanStyle.Render("username:"), formatValidationStatus(usernameCheck))
		if usernameCheck.Status != "PASS" {
			allPass = false
		}
	}

	passwordCheck := validatePassword(settingPassword)
	if settingPassword != "" {
		fmt.Printf("│ %s %s\n", settingCyanStyle.Render("password:"), formatValidationStatus(passwordCheck))
		if passwordCheck.Status != "PASS" {
			allPass = false
		}
	}

	fmt.Println("│")
	if allPass {
		fmt.Println(settingGreenStyle.Render("│ ✓ All validations passed"))
	} else {
		fmt.Println(settingRedStyle.Render("│ ✖ Some validations failed"))
	}
	fmt.Println("│")
	fmt.Println(settingCyanStyle.Render("╰─────────────────────────────────"))
	fmt.Println()
}

type settingValidation struct {
	Status  string
	Message string
}

func formatValidationStatus(v settingValidation) string {
	switch v.Status {
	case "PASS":
		return settingGreenStyle.Render("✓ " + v.Message)
	case "FAIL":
		return settingRedStyle.Render("✖ " + v.Message)
	case "WARN":
		return settingYellowStyle.Render("⚠ " + v.Message)
	default:
		return settingCyanStyle.Render(v.Message)
	}
}

func validatePortAvailability(port int) settingValidation {
	if port <= 0 || port > 65535 {
		return settingValidation{Status: "FAIL", Message: fmt.Sprintf("Port %d is out of valid range (1-65535)", port)}
	}

	addr := fmt.Sprintf(":%d", port)
	ln, err := net.Listen("tcp", addr)
	if err != nil {
		return settingValidation{Status: "FAIL", Message: fmt.Sprintf("Port %d is already in use", port)}
	}
	ln.Close()

	return settingValidation{Status: "PASS", Message: fmt.Sprintf("Port %d is available", port)}
}

func validateListenIP(listenIP string) settingValidation {
	if listenIP == "" {
		return settingValidation{Status: "WARN", Message: "Listen IP is empty"}
	}

	ip := net.ParseIP(listenIP)
	if ip == nil {
		return settingValidation{Status: "FAIL", Message: fmt.Sprintf("Invalid IP address: %s", listenIP)}
	}

	return settingValidation{Status: "PASS", Message: fmt.Sprintf("IP address %s is valid", listenIP)}
}

func validateWebBasePath(webBasePath string) settingValidation {
	if webBasePath == "" {
		return settingValidation{Status: "WARN", Message: "Web base path is empty"}
	}

	if !strings.HasPrefix(webBasePath, "/") {
		return settingValidation{Status: "FAIL", Message: "Base path must start with /"}
	}

	if len(webBasePath) < 4 && webBasePath != "/" {
		return settingValidation{Status: "FAIL", Message: "Base path must be at least 4 characters (or /)"}
	}

	if strings.Contains(webBasePath, " ") {
		return settingValidation{Status: "FAIL", Message: "Base path cannot contain spaces"}
	}

	return settingValidation{Status: "PASS", Message: fmt.Sprintf("Base path %s is valid", webBasePath)}
}

func validateUsername(username string) settingValidation {
	if username == "" {
		return settingValidation{Status: "WARN", Message: "Username is empty"}
	}

	if len(username) < 3 {
		return settingValidation{Status: "FAIL", Message: "Username must be at least 3 characters"}
	}

	if strings.Contains(username, " ") {
		return settingValidation{Status: "FAIL", Message: "Username cannot contain spaces"}
	}

	return settingValidation{Status: "PASS", Message: "Username is valid"}
}

func validatePassword(password string) settingValidation {
	if password == "" {
		return settingValidation{Status: "WARN", Message: "Password is empty"}
	}

	if len(password) < 6 {
		return settingValidation{Status: "FAIL", Message: "Password must be at least 6 characters"}
	}

	return settingValidation{Status: "PASS", Message: "Password is valid"}
}
