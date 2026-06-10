package web

import (
	"context"
	"crypto/tls"
	"embed"
	"io/fs"
	"mime"
	"net"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/drunkleen/l-ui/hub/web/controller"
	"github.com/drunkleen/l-ui/hub/web/job"
	"github.com/drunkleen/l-ui/hub/web/locale"
	"github.com/drunkleen/l-ui/hub/web/middleware"
	"github.com/drunkleen/l-ui/hub/web/network"
	"github.com/drunkleen/l-ui/hub/web/runtime"
	"github.com/drunkleen/l-ui/hub/web/service"
	"github.com/drunkleen/l-ui/hub/web/session"
	"github.com/drunkleen/l-ui/hub/web/websocket"
	"github.com/drunkleen/l-ui/internal/config"
	"github.com/drunkleen/l-ui/internal/logger"
	"github.com/drunkleen/l-ui/internal/util/common"

	"github.com/gofiber/fiber/v3"
	"github.com/gofiber/fiber/v3/middleware/compress"
	"github.com/gofiber/fiber/v3/middleware/recover"
	"github.com/robfig/cron/v3"
)

//go:embed translation/*
var i18nFS embed.FS

//go:embed all:dist
var distFS embed.FS

var startTime = time.Now()

type wrapDistFS struct {
	embed.FS
}

func (f *wrapDistFS) Open(name string) (fs.File, error) {
	file, err := f.FS.Open("dist/assets/" + name)
	if err != nil {
		return nil, err
	}
	return &wrapAssetsFile{
		File: file,
	}, nil
}

type wrapAssetsFile struct {
	fs.File
}

func (f *wrapAssetsFile) Stat() (fs.FileInfo, error) {
	info, err := f.File.Stat()
	if err != nil {
		return nil, err
	}
	return &wrapAssetsFileInfo{
		FileInfo: info,
	}, nil
}

type wrapAssetsFileInfo struct {
	fs.FileInfo
}

func (f *wrapAssetsFileInfo) ModTime() time.Time {
	return startTime
}

func EmbeddedDist() embed.FS {
	return distFS
}

type Server struct {
	app      *fiber.App
	listener net.Listener

	index *controller.IndexController
	panel *controller.LUIController
	api   *controller.APIController
	ws    *controller.WebSocketController

	xrayService      service.XrayService
	settingService   service.SettingService
	tgbotService     service.Tgbot
	customGeoService *service.CustomGeoService

	wsHub *websocket.Hub

	cron *cron.Cron

	ctx    context.Context
	cancel context.CancelFunc
}

func (s *Server) ModeString() string { return "hub" }

func NewServer() *Server {
	ctx, cancel := context.WithCancel(context.Background())
	return &Server{
		ctx:    ctx,
		cancel: cancel,
	}
}

func (s *Server) isDirectHTTPSConfigured() bool {
	certFile, certErr := s.settingService.GetCertFile()
	keyFile, keyErr := s.settingService.GetKeyFile()
	if certErr != nil || keyErr != nil || certFile == "" || keyFile == "" {
		return false
	}
	_, err := tls.LoadX509KeyPair(certFile, keyFile)
	return err == nil
}

func (s *Server) initRouter() (*fiber.App, error) {
	app := fiber.New(fiber.Config{
		AppName: "l-ui",
		ErrorHandler: func(c fiber.Ctx, err error) error {
			c.Status(fiber.StatusNotFound)
			return nil
		},
	})

	if config.IsDebug() {
		app.Use(recover.New())
	} else {
		app.Server().LogAllErrors = false
	}

	directHTTPS := s.isDirectHTTPSConfigured()
	sendHSTS := directHTTPS && !config.IsSkipHSTS()
	app.Use(middleware.SecurityHeadersMiddleware(sendHSTS))

	webDomain, err := s.settingService.GetWebDomain()
	if err != nil {
		return nil, err
	}

	if webDomain != "" {
		app.Use(middleware.DomainValidatorMiddleware(webDomain))
	}

	secret, err := s.settingService.GetSecret()
	if err != nil {
		return nil, err
	}

	basePath, err := s.settingService.GetBasePath()
	if err != nil {
		return nil, err
	}

	app.Use(compress.New(compress.Config{
		Level: compress.LevelDefault,
	}))

	assetsBasePath := basePath + "assets/"

	// Session middleware
	directHTTPSBool := directHTTPS
	sessionMaxAge := 0
	if sma, err := s.settingService.GetSessionMaxAge(); err == nil && sma > 0 {
		sessionMaxAge = sma * 60 // minutes -> seconds
	}

	sessionHandler := session.SetupStore([]byte(secret), basePath, directHTTPSBool, sessionMaxAge)
	if sessionHandler != nil {
		app.Use(sessionHandler)
	}

	app.Use(func(c fiber.Ctx) error {
		c.Locals("base_path", basePath)
		return c.Next()
	})

	app.Use(func(c fiber.Ctx) error {
		uri := c.Path()
		if strings.HasPrefix(uri, assetsBasePath) {
			c.Set("Cache-Control", "max-age=31536000")
		}
		return c.Next()
	})

	err = locale.InitLocalizer(i18nFS, &s.settingService)
	if err != nil {
		return nil, err
	}

	app.Use(locale.LocalizerMiddleware())

	g := app.Group(basePath)

	// Serve Vite-built frontend assets
	if config.IsDebug() {
		g.Get("/assets/*", func(c fiber.Ctx) error {
			path := strings.TrimPrefix(c.Params("*"), "/")
			if path == "" {
				path = c.Params("*")
			}
			filePath := "web/dist/assets/" + path
			return c.SendFile(filePath)
		})
	} else {
		g.Get("/assets/*", func(c fiber.Ctx) error {
			path := strings.TrimPrefix(c.Params("*"), "/")
			if path == "" {
				path = c.Params("*")
			}
			data, err := distFS.ReadFile("dist/assets/" + path)
			if err != nil {
				return c.SendStatus(fiber.StatusNotFound)
			}
			if ct := mime.TypeByExtension(filepath.Ext(path)); ct != "" {
				c.Set("Content-Type", ct)
			}
			c.Set("Cache-Control", "max-age=31536000")
			return c.Send(data)
		})
	}

	app.Use(middleware.RedirectMiddleware(basePath))

	controller.SetDistFS(distFS)

	s.index = controller.NewIndexController(g)
	s.panel = controller.NewLUIController(g)
	g.Get("/panel/api/openapi.json", controller.ServeOpenAPISpec)
	s.api = controller.NewAPIController(g, s.customGeoService)

	s.wsHub = websocket.NewHub()
	go s.wsHub.Run()

	s.ws = controller.NewWebSocketController(service.NewWebSocketService(s.wsHub))
	g.Get("/ws", s.ws.HandleWebSocket)

	app.Get("/.well-known/appspecific/com.chrome.devtools.json", func(c fiber.Ctx) error {
		return c.Status(fiber.StatusOK).JSON(fiber.Map{})
	})

	app.Get("/healthz", controller.Healthz)
	app.Get("/readyz", controller.Readyz)

	return app, nil
}

func (s *Server) startTask(restartXray bool) {
	s.customGeoService.EnsureOnStartup()
	s.cron.AddJob("@every 5s", job.NewNodeHeartbeatJob())
	s.cron.AddJob("@every 5s", job.NewNodeTrafficSyncJob())

	s.cron.AddJob("@daily", job.NewClearLogsJob())

	s.cron.AddJob("@hourly", job.NewPeriodicTrafficResetJob("hourly"))
	s.cron.AddJob("@daily", job.NewPeriodicTrafficResetJob("daily"))
	s.cron.AddJob("@weekly", job.NewPeriodicTrafficResetJob("weekly"))
	s.cron.AddJob("@monthly", job.NewPeriodicTrafficResetJob("monthly"))

	if ldapEnabled, _ := s.settingService.GetLdapEnable(); ldapEnabled {
		runtime, err := s.settingService.GetLdapSyncCron()
		if err != nil || runtime == "" {
			runtime = "@every 1m"
		}
		j := job.NewLdapSyncJob()
		s.cron.AddJob(runtime, j)
	}

	var entry cron.EntryID
	isTgbotenabled, err := s.settingService.GetTgbotEnabled()
	if (err == nil) && (isTgbotenabled) {
		runtime, err := s.settingService.GetTgbotRuntime()
		if err != nil {
			logger.Warningf("Add NewStatsNotifyJob: failed to load runtime: %v; using default @daily", err)
			runtime = "@daily"
		} else if strings.TrimSpace(runtime) == "" {
			logger.Warning("Add NewStatsNotifyJob runtime is empty, using default @daily")
			runtime = "@daily"
		}
		logger.Infof("Tg notify enabled,run at %s", runtime)
		_, err = s.cron.AddJob(runtime, job.NewStatsNotifyJob())
		if err != nil {
			logger.Warningf("Add NewStatsNotifyJob: failed to schedule runtime %q: %v", runtime, err)
			return
		}

		s.cron.AddJob("@every 2m", job.NewCheckHashStorageJob())

		cpuThreshold, err := s.settingService.GetTgCpu()
		if (err == nil) && (cpuThreshold > 0) {
			s.cron.AddJob("@every 10s", job.NewCheckCpuJob())
		}

		s.cron.AddJob("@every 10s", job.NewNodeAlertJob())
	} else {
		s.cron.Remove(entry)
	}

	s.cron.AddJob("@daily", job.NewNodeCertRenewalJob())
}

func (s *Server) Start() (err error) {
	return s.start(true, true)
}

func (s *Server) StartPanelOnly() (err error) {
	return s.start(false, false)
}

func (s *Server) start(restartXray bool, startTgBot bool) (err error) {
	defer func() {
		if err != nil {
			s.Stop()
		}
	}()

	loc, err := s.settingService.GetTimeLocation()
	if err != nil {
		return err
	}
	service.StartTrafficWriter()

	s.cron = cron.New(cron.WithLocation(loc), cron.WithSeconds())
	s.cron.Start()

	runtime.SetManager(runtime.NewManager(runtime.LocalDeps{
		APIPort:        func() int { return s.xrayService.GetXrayAPIPort() },
		SetNeedRestart: func() { s.xrayService.SetToNeedRestart() },
	}))

	s.customGeoService = service.NewCustomGeoService()

	app, err := s.initRouter()
	if err != nil {
		return err
	}
	s.app = app

	certFile, err := s.settingService.GetCertFile()
	if err != nil {
		return err
	}
	keyFile, err := s.settingService.GetKeyFile()
	if err != nil {
		return err
	}
	listen, err := s.settingService.GetListen()
	if err != nil {
		return err
	}
	port, err := s.settingService.GetPort()
	if err != nil {
		return err
	}
	listenAddr := net.JoinHostPort(listen, strconv.Itoa(port))
	listener, err := net.Listen("tcp", listenAddr)
	if err != nil {
		return err
	}
	if certFile != "" || keyFile != "" {
		cert, err := tls.LoadX509KeyPair(certFile, keyFile)
		if err == nil {
			c := &tls.Config{
				Certificates: []tls.Certificate{cert},
			}
			listener = network.NewAutoHttpsListener(listener)
			listener = tls.NewListener(listener, c)
			logger.Info("Web server running HTTPS on", listener.Addr())
		} else {
			logger.Error("Error loading certificates:", err)
			logger.Info("Web server running HTTP on", listener.Addr())
		}
	} else {
		logger.Info("Web server running HTTP on", listener.Addr())
	}
	s.listener = listener

	go func() {
		if err := app.Listener(listener); err != nil {
			logger.Errorf("Web server error: %v", err)
		}
	}()

	s.startTask(restartXray)

	if startTgBot {
		isTgbotenabled, err := s.settingService.GetTgbotEnabled()
		if (err == nil) && (isTgbotenabled) {
			tgBot := s.tgbotService.NewTgbot()
			tgBot.Start(i18nFS)
		}
	}

	return nil
}

func (s *Server) Stop() error {
	return s.stop(true, true)
}

func (s *Server) StopPanelOnly() error {
	return s.stop(false, false)
}

func (s *Server) stop(stopXray bool, stopTgBot bool) error {
	s.cancel()
	if s.cron != nil {
		s.cron.Stop()
	}
	if err := service.PersistSystemMetrics(); err != nil {
		logger.Warning("persist system metrics on shutdown failed:", err)
	}
	if stopXray {
		service.StopTrafficWriter()
	}
	if stopTgBot && s.tgbotService.IsRunning() {
		s.tgbotService.Stop()
	}
	if s.wsHub != nil {
		s.wsHub.Stop()
	}
	var err1 error
	var err2 error
	if s.app != nil {
		err1 = s.app.ShutdownWithTimeout(15 * time.Second)
	}
	if s.listener != nil {
		err2 = s.listener.Close()
	}
	return common.Combine(err1, err2)
}

func (s *Server) GetCtx() context.Context {
	return s.ctx
}

func (s *Server) GetCron() *cron.Cron {
	return s.cron
}

func (s *Server) GetWSHub() any {
	return s.wsHub
}
