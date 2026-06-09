// Package sub provides subscription server functionality for the l-ui panel,
// including HTTP/HTTPS servers for serving subscription links and JSON configurations.
package sub

import (
	"context"
	"crypto/tls"
	"io/fs"
	"mime"
	"net"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/drunkleen/l-ui/hub/web/locale"
	"github.com/drunkleen/l-ui/hub/web/middleware"
	"github.com/drunkleen/l-ui/hub/web/network"
	"github.com/drunkleen/l-ui/hub/web/service"
	"github.com/drunkleen/l-ui/internal/logger"
	"github.com/drunkleen/l-ui/internal/util/common"

	"github.com/gofiber/fiber/v3"
)

// Server represents the subscription server that serves subscription links and JSON configurations.
type Server struct {
	app      *fiber.App
	listener net.Listener

	sub            *SUBController
	settingService service.SettingService

	ctx    context.Context
	cancel context.CancelFunc
}

// NewServer creates a new subscription server instance with a cancellable context.
func NewServer() *Server {
	ctx, cancel := context.WithCancel(context.Background())
	return &Server{
		ctx:    ctx,
		cancel: cancel,
	}
}

// initRouter configures the subscription server's Fiber app, middleware, and
// static assets and returns the ready-to-use app.
func (s *Server) initRouter() (*fiber.App, error) {
	app := fiber.New()

	subDomain, err := s.settingService.GetSubDomain()
	if err != nil {
		return nil, err
	}

	if subDomain != "" {
		app.Use(middleware.DomainValidatorMiddleware(subDomain))
	}

	LinksPath, err := s.settingService.GetSubPath()
	if err != nil {
		return nil, err
	}

	JsonPath, err := s.settingService.GetSubJsonPath()
	if err != nil {
		return nil, err
	}

	ClashPath, err := s.settingService.GetSubClashPath()
	if err != nil {
		return nil, err
	}

	subJsonEnable, err := s.settingService.GetSubJsonEnable()
	if err != nil {
		return nil, err
	}

	subClashEnable, err := s.settingService.GetSubClashEnable()
	if err != nil {
		return nil, err
	}

	// Set base_path based on LinksPath for template rendering
	// Ensure LinksPath ends with "/" for proper asset URL generation
	basePath := LinksPath
	if basePath != "/" && !strings.HasSuffix(basePath, "/") {
		basePath += "/"
	}
	// logger.Debug("sub: Setting base_path to:", basePath)
	app.Use(func(c fiber.Ctx) error {
		c.Locals("base_path", basePath)
		return c.Next()
	})

	Encrypt, err := s.settingService.GetSubEncrypt()
	if err != nil {
		return nil, err
	}

	ShowInfo, err := s.settingService.GetSubShowInfo()
	if err != nil {
		return nil, err
	}

	RemarkModel, err := s.settingService.GetRemarkModel()
	if err != nil {
		RemarkModel = "-io"
	}

	SubUpdates, err := s.settingService.GetSubUpdates()
	if err != nil {
		SubUpdates = "10"
	}

	SubJsonFragment, err := s.settingService.GetSubJsonFragment()
	if err != nil {
		SubJsonFragment = ""
	}

	SubJsonNoises, err := s.settingService.GetSubJsonNoises()
	if err != nil {
		SubJsonNoises = ""
	}

	SubJsonMux, err := s.settingService.GetSubJsonMux()
	if err != nil {
		SubJsonMux = ""
	}

	SubJsonRules, err := s.settingService.GetSubJsonRules()
	if err != nil {
		SubJsonRules = ""
	}

	SubTitle, err := s.settingService.GetSubTitle()
	if err != nil {
		SubTitle = ""
	}

	SubSupportUrl, err := s.settingService.GetSubSupportUrl()
	if err != nil {
		SubSupportUrl = ""
	}

	SubProfileUrl, err := s.settingService.GetSubProfileUrl()
	if err != nil {
		SubProfileUrl = ""
	}

	SubAnnounce, err := s.settingService.GetSubAnnounce()
	if err != nil {
		SubAnnounce = ""
	}

	SubEnableRouting, err := s.settingService.GetSubEnableRouting()
	if err != nil {
		return nil, err
	}

	SubRoutingRules, err := s.settingService.GetSubRoutingRules()
	if err != nil {
		SubRoutingRules = ""
	}

	// set per-request localizer from headers/cookies
	app.Use(locale.LocalizerMiddleware())

	// Mount the Vite-built dist/assets/ so the subscription page's JS/CSS
	// bundles load from `/assets/...`. Also mount the same FS under the
	// subscription path prefix (LinksPath + "assets") so reverse proxies
	// running the panel under a URI prefix can resolve those URLs too.
	// Note: LinksPath always starts and ends with "/" (validated in settings).
	var linksPathForAssets string
	if LinksPath == "/" {
		linksPathForAssets = "/assets"
	} else {
		linksPathForAssets = strings.TrimRight(LinksPath, "/") + "/assets"
	}

	// Try on-disk assets first, then fall back to embedded FS.
	var assetDir string
	var embedFS fs.FS
	if _, err := os.Stat("hub/web/dist/assets"); err == nil {
		assetDir = "hub/web/dist/assets"
	} else if subFS, err := fs.Sub(distFS, "dist/assets"); err == nil {
		embedFS = subFS
	} else {
		logger.Error("sub: failed to mount embedded dist assets:", err)
	}

	// registerStaticRoute mounts static files at the given prefix, using
	// either the on-disk directory or a handler backed by the embedded FS.
	registerStaticRoute := func(prefix string) {
		prefixTrimmed := strings.TrimRight(prefix, "/")
		app.Get(prefixTrimmed+"/*", func(c fiber.Ctx) error {
			filePath := strings.TrimPrefix(c.Params("*"), "/")
			if filePath == "" {
				return c.Next()
			}
			if assetDir != "" {
				fullPath := filepath.Join(assetDir, filePath)
				if _, err := os.Stat(fullPath); err == nil {
					return c.Res().SendFile(fullPath)
				}
				return c.Next()
			}
			if embedFS != nil {
				data, err := fs.ReadFile(embedFS, filePath)
				if err != nil {
					return c.Next()
				}
				c.Set("Content-Type", mime.TypeByExtension(filepath.Ext(filePath)))
				return c.Send(data)
			}
			return c.Next()
		})
	}

	registerStaticRoute("/assets")
	if linksPathForAssets != "/assets" {
		registerStaticRoute(linksPathForAssets)
	}

	// Browser may resolve subpage assets relative to the request URL —
	// /sub/<basePath>/<subId>/assets/... — so route those to the same FS.
	if LinksPath != "/" {
		pathPrefix := strings.TrimRight(LinksPath, "/") + "/"
		app.Use(func(c fiber.Ctx) error {
			path := c.Path()
			if strings.HasPrefix(path, pathPrefix) && strings.Contains(path, "/assets/") {
				_, after, ok := strings.Cut(path, "/assets/")
				if ok {
					assetPath := strings.TrimPrefix(after, "/")
					if assetPath != "" {
						if assetDir != "" {
							fullPath := filepath.Join(assetDir, assetPath)
							if _, err := os.Stat(fullPath); err == nil {
								return c.Res().SendFile(fullPath)
							}
						}
						if embedFS != nil {
							data, err := fs.ReadFile(embedFS, assetPath)
							if err == nil {
								c.Set("Content-Type", mime.TypeByExtension(filepath.Ext(assetPath)))
								return c.Send(data)
							}
						}
					}
				}
			}
			return c.Next()
		})
	}

	g := app.Group("/")

	s.sub = NewSUBController(
		g, LinksPath, JsonPath, ClashPath, subJsonEnable, subClashEnable, Encrypt, ShowInfo, RemarkModel, SubUpdates,
		SubJsonFragment, SubJsonNoises, SubJsonMux, SubJsonRules, SubTitle, SubSupportUrl,
		SubProfileUrl, SubAnnounce, SubEnableRouting, SubRoutingRules)

	return app, nil
}

// Start initializes and starts the subscription server with configured settings.
func (s *Server) Start() (err error) {
	// This is an anonymous function, no function name
	defer func() {
		if err != nil {
			s.Stop()
		}
	}()

	subEnable, err := s.settingService.GetSubEnable()
	if err != nil {
		return err
	}
	if !subEnable {
		return nil
	}

	app, err := s.initRouter()
	if err != nil {
		return err
	}
	s.app = app

	certFile, err := s.settingService.GetSubCertFile()
	if err != nil {
		return err
	}
	keyFile, err := s.settingService.GetSubKeyFile()
	if err != nil {
		return err
	}
	listen, err := s.settingService.GetSubListen()
	if err != nil {
		return err
	}
	port, err := s.settingService.GetSubPort()
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
			logger.Info("Sub server running HTTPS on", listener.Addr())
		} else {
			logger.Error("Error loading certificates:", err)
			logger.Info("Sub server running HTTP on", listener.Addr())
		}
	} else {
		logger.Info("Sub server running HTTP on", listener.Addr())
	}
	s.listener = listener

	go func() {
		app.Server().Serve(listener)
	}()

	return nil
}

// Stop gracefully shuts down the subscription server and closes the listener.
func (s *Server) Stop() error {
	s.cancel()

	var err1 error
	var err2 error
	if s.app != nil {
		shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer shutdownCancel()
		err1 = s.app.Server().ShutdownWithContext(shutdownCtx)
	}
	if s.listener != nil {
		err2 = s.listener.Close()
	}
	return common.Combine(err1, err2)
}

// GetCtx returns the server's context for cancellation and deadline management.
func (s *Server) GetCtx() context.Context {
	return s.ctx
}
