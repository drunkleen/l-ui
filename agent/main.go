package main

import (
	"log"
	"os"
	"os/signal"
	"strings"
	"syscall"

	agentdb "github.com/drunkleen/l-ui/agent/database"
	"github.com/drunkleen/l-ui/internal/config"
	"github.com/drunkleen/l-ui/internal/logger"
	"github.com/drunkleen/l-ui/internal/util/sys"

	"github.com/joho/godotenv"
)

func main() {
	log.Printf("Starting %v %v (agent mode)", config.GetName(), config.GetVersion())

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

	agentDBPath := config.GetDBFolderPath() + "/l-ui-agent.db"
	if err := agentdb.InitDB(agentDBPath); err != nil {
		log.Fatalf("Error initializing agent database: %v", err)
	}

	authSecret := resolveAuthSecret()

	certDir := os.Getenv("LUI_CERT_DIR")
	if certDir == "" {
		certDir = config.GetDBFolderPath() + "/certs"
	}

	server := NewAgentServer(authSecret, certDir)
	if err := server.Start(); err != nil {
		log.Fatalf("Error starting agent server: %v", err)
	}
	log.Printf("Agent server started on port %d", server.Port())

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGHUP, syscall.SIGTERM, sys.SIGUSR1, os.Interrupt)

	for {
		sig := <-sigCh
		switch sig {
		case syscall.SIGHUP:
			logger.Info("Received SIGHUP signal. Restarting agent server...")
			if err := server.Stop(); err != nil {
				logger.Debug("Error stopping agent server:", err)
			}
			server = NewAgentServer(authSecret, certDir)
			if err := server.Start(); err != nil {
				log.Fatalf("Error restarting agent server: %v", err)
			}
			log.Println("Agent server restarted successfully.")
		case sys.SIGUSR1:
			logger.Info("Received USR1 signal, xray restart not implemented in standalone agent mode.")
		default:
			if err := server.Stop(); err != nil {
				logger.Debug("Error stopping agent server:", err)
			}
			log.Println("Shutting down agent server.")
			return
		}
	}
}

func resolveAuthSecret() string {
	if secret := loadStoredSecret(); secret != "" {
		logger.Info("Using stored node secret from database")
		return secret
	}

	regToken := strings.TrimSpace(os.Getenv("LUI_REGISTRATION_TOKEN"))
	hubEndpoint := strings.TrimSpace(os.Getenv("LUI_HUB_ENDPOINT"))
	if regToken != "" && hubEndpoint != "" {
		logger.Info("Attempting self-registration with hub...")
		secret, nodeID, err := tryRegisterWithHub(hubEndpoint, regToken)
		if err != nil {
			log.Fatalf("Self-registration with hub failed: %v", err)
		}
		storeNodeSecret(secret, nodeID, hubEndpoint)
		logger.Info("Self-registration successful, node secret stored")
		return secret
	}

	bootstrapToken := strings.TrimSpace(os.Getenv("LUI_BOOTSTRAP_API_TOKEN"))
	if bootstrapToken != "" {
		logger.Info("Using bootstrap API token from environment")
		return bootstrapToken
	}

	log.Fatal("No authentication secret found. Set LUI_BOOTSTRAP_API_TOKEN, or LUI_REGISTRATION_TOKEN + LUI_HUB_ENDPOINT, or ensure a node secret exists in the database")
	return ""
}
