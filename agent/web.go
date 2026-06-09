package main

import (
	"context"
	"crypto/tls"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"strconv"
	"time"

	"github.com/drunkleen/l-ui/agent/controller"
	"github.com/drunkleen/l-ui/internal/config"
	"github.com/drunkleen/l-ui/internal/logger"

	"github.com/gofiber/fiber/v3"
	"github.com/robfig/cron/v3"
)

type AgentServer struct {
	app        *fiber.App
	listener   net.Listener
	cron       *cron.Cron
	port       int
	authSecret string
	certDir    string
	ctx        context.Context
	cancel     context.CancelFunc
}

func NewAgentServer(authSecret, certDir string) *AgentServer {
	port := 2054
	if webPort := config.GetEnvInt("LUI_WEB_PORT"); webPort > 0 {
		port = webPort
	}
	ctx, cancel := context.WithCancel(context.Background())
	return &AgentServer{
		port:       port,
		authSecret: authSecret,
		certDir:    certDir,
		ctx:        ctx,
		cancel:     cancel,
	}
}

func (s *AgentServer) initRouter() *fiber.App {
	app := fiber.New()

	api := app.Group("/api/v1")
	api.Use(controller.AuthMiddleware(s.authSecret))

	status := &controller.StatusController{}
	metrics := &controller.MetricsController{}
	sysinfo := &controller.SysInfoController{}
	cfg := &controller.ConfigController{}
	fw := &controller.FirewallController{}
	logs := &controller.LogsController{}
	restart := &controller.RestartController{}
	xrayCtrl := controller.NewXrayController()
	cert := controller.NewCertController(s.certDir)

	api.Get("/status", status.GetStatus)
	api.Get("/metrics", metrics.GetMetrics)
	api.Get("/sysinfo", sysinfo.GetSysInfo)
	api.Get("/config", cfg.GetConfig)
	api.Post("/config/push", cfg.PushConfig)
	api.Post("/config/apply", cfg.ApplyConfig)
	api.Get("/firewall/status", fw.GetStatus)
	api.Get("/firewall/rules", fw.GetRules)
	api.Post("/firewall/rules", fw.AddRule)
	api.Delete("/firewall/rules", fw.DeleteRule)
	api.Post("/firewall/enable", fw.Enable)
	api.Post("/firewall/disable", fw.Disable)
	api.Get("/logs", logs.TailLog)
	api.Post("/restart", restart.RestartAgent)
	api.Post("/xray/restart", restart.RestartXray)
	api.Get("/xray/version", xrayCtrl.GetVersion)
	api.Get("/xray/status", xrayCtrl.GetStatus)
	api.Post("/xray/install", xrayCtrl.Install)
	api.Post("/config/apply", xrayCtrl.ApplyConfig)

	api.Post("/certs", cert.Push)
	api.Get("/certs/status", cert.Status)

	app.Get("/healthz", controller.Healthz)
	app.Get("/readyz", controller.Readyz)

	app.Use(func(c fiber.Ctx) error {
		return c.Status(fiber.StatusNotFound).SendString("Not Found")
	})

	return app
}

func (s *AgentServer) startCron() {
	loc, err := time.LoadLocation("UTC")
	if err != nil {
		loc = time.UTC
	}
	s.cron = cron.New(cron.WithLocation(loc), cron.WithSeconds())
	s.cron.Start()
}

func (s *AgentServer) Start() error {
	s.app = s.initRouter()
	s.startCron()

	listenAddr := net.JoinHostPort("", strconv.Itoa(s.port))
	listener, err := net.Listen("tcp", listenAddr)
	if err != nil {
		return err
	}

	certPath := filepath.Join(s.certDir, "agent_cert.pem")
	keyPath := filepath.Join(s.certDir, "agent_key.pem")

	logger.Infof("Agent server listening on %s", listener.Addr())

	if _, err := os.Stat(certPath); err == nil {
		if _, err := os.Stat(keyPath); err == nil {
			tlsCert, err := tls.LoadX509KeyPair(certPath, keyPath)
			if err != nil {
				return fmt.Errorf("load TLS cert: %w", err)
			}
			listener = tls.NewListener(listener, &tls.Config{
				Certificates: []tls.Certificate{tlsCert},
				MinVersion:   tls.VersionTLS12,
			})
			logger.Infof("TLS enabled for agent")
		}
	}

	s.listener = listener
	go func() {
		if err := s.app.Listener(listener, fiber.ListenConfig{DisableStartupMessage: true}); err != nil {
			logger.Errorf("Agent server error: %v", err)
		}
	}()
	return nil
}

func (s *AgentServer) Stop() error {
	s.cancel()
	if s.cron != nil {
		s.cron.Stop()
	}
	if s.app != nil {
		return s.app.ShutdownWithTimeout(15 * time.Second)
	}
	return nil
}

func (s *AgentServer) Port() int {
	return s.port
}

func (s *AgentServer) AuthSecret() string {
	return s.authSecret
}
