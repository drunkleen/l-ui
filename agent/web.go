package main

import (
	"context"
	"crypto/tls"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"time"

	"github.com/drunkleen/l-ui/agent/controller"
	"github.com/drunkleen/l-ui/internal/config"
	"github.com/drunkleen/l-ui/internal/logger"

	"github.com/gin-gonic/gin"
	"github.com/robfig/cron/v3"
)

type AgentServer struct {
	httpServer *http.Server
	listener   net.Listener
	engine     *gin.Engine
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

func (s *AgentServer) initRouter() *gin.Engine {
	if config.IsDebug() {
		gin.SetMode(gin.DebugMode)
	} else {
		gin.DefaultWriter = io.Discard
		gin.DefaultErrorWriter = io.Discard
		gin.SetMode(gin.ReleaseMode)
	}

	engine := gin.Default()

	api := engine.Group("/api/v1")
	api.Use(controller.AuthMiddleware(s.authSecret))

	status := &controller.StatusController{}
	metrics := &controller.MetricsController{}
	sysinfo := &controller.SysInfoController{}
	cfg := &controller.ConfigController{}
	fw := &controller.FirewallController{}
	logs := &controller.LogsController{}
	restart := &controller.RestartController{}
	cert := controller.NewCertController(s.certDir)

	api.GET("/status", status.GetStatus)
	api.GET("/metrics", metrics.GetMetrics)
	api.GET("/sysinfo", sysinfo.GetSysInfo)
	api.GET("/config", cfg.GetConfig)
	api.POST("/config/push", cfg.PushConfig)
	api.POST("/config/apply", cfg.ApplyConfig)
	api.GET("/firewall/status", fw.GetStatus)
	api.GET("/firewall/rules", fw.GetRules)
	api.POST("/firewall/rules", fw.AddRule)
	api.DELETE("/firewall/rules", fw.DeleteRule)
	api.POST("/firewall/enable", fw.Enable)
	api.POST("/firewall/disable", fw.Disable)
	api.GET("/logs", logs.TailLog)
	api.POST("/restart", restart.RestartAgent)
	api.POST("/xray/restart", restart.RestartXray)

	api.POST("/certs", cert.Push)
	api.GET("/certs/status", cert.Status)

	engine.GET("/healthz", controller.Healthz)
	engine.GET("/readyz", controller.Readyz)

	engine.NoRoute(func(c *gin.Context) {
		c.AbortWithStatus(http.StatusNotFound)
	})

	return engine
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
	s.engine = s.initRouter()
	s.startCron()

	listenAddr := net.JoinHostPort("", strconv.Itoa(s.port))
	listener, err := net.Listen("tcp", listenAddr)
	if err != nil {
		return err
	}
	s.listener = listener

	s.httpServer = &http.Server{
		Handler:           s.engine,
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       30 * time.Second,
		WriteTimeout:      30 * time.Second,
		IdleTimeout:       120 * time.Second,
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
			s.httpServer.TLSConfig = &tls.Config{
				Certificates: []tls.Certificate{tlsCert},
				MinVersion:   tls.VersionTLS12,
			}
			listener = tls.NewListener(listener, s.httpServer.TLSConfig)
			logger.Infof("TLS enabled for agent")
		}
	}

	go func() {
		if err := s.httpServer.Serve(listener); err != nil && err != http.ErrServerClosed {
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
	if s.httpServer != nil {
		shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer shutdownCancel()
		return s.httpServer.Shutdown(shutdownCtx)
	}
	return nil
}

func (s *AgentServer) Port() int {
	return s.port
}

func (s *AgentServer) AuthSecret() string {
	return s.authSecret
}
