package controller

import (
	"io"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/drunkleen/l-ui/hub/web/entity"
	"github.com/drunkleen/l-ui/hub/web/global"
	"github.com/drunkleen/l-ui/hub/web/middleware"
	"github.com/drunkleen/l-ui/hub/web/service"
	"github.com/drunkleen/l-ui/hub/web/session"
	"github.com/drunkleen/l-ui/internal/nodeauth"

	"github.com/gofiber/fiber/v3"
)

var nodeAuthReplay = struct {
	m sync.Mutex
	v map[string]int64
}{v: map[string]int64{}}

// APIController handles the main API routes for the l-ui panel.
type APIController struct {
	BaseController
	inboundController      *InboundController
	serverController       *ServerController
	nodeController         *NodeController
	registrationController *RegistrationController
	settingService         service.SettingService
	userService            service.UserService
	apiTokenService        service.ApiTokenService
	Tgbot                  service.Tgbot
}

func NewAPIController(router fiber.Router, customGeo *service.CustomGeoService) *APIController {
	a := &APIController{
		registrationController: NewRegistrationController(),
	}
	a.initRouter(router, customGeo)
	return a
}

func (a *APIController) checkAPIAuth(c fiber.Ctx) error {
	auth := c.Get("Authorization")
	if after, ok := strings.CutPrefix(auth, "Bearer "); ok {
		tok := strings.TrimSpace(after)
		if a.hasSignedAPIHeaders(c) && !a.apiTokenService.Match(tok) {
			abortJSONError(c, fiber.StatusUnauthorized, service.NodeErrAuth, "invalid node credentials")
			return nil
		}
		if a.hasSignedAPIHeaders(c) {
			if a.apiTokenService.Match(tok) && a.verifySignedAPIRequest(c, tok) {
				if u, err := a.userService.GetFirstUser(); err == nil {
					session.SetAPIAuthUser(c, u)
				}
				c.Locals("api_authed", true)
				return c.Next()
			}
		} else if a.apiTokenService.Match(tok) {
			if u, err := a.userService.GetFirstUser(); err == nil {
				session.SetAPIAuthUser(c, u)
			}
			c.Locals("api_authed", true)
			return c.Next()
		}
	}
	if !session.IsLogin(c) {
		if c.Get("X-Requested-With") == "XMLHttpRequest" {
			return c.SendStatus(fiber.StatusUnauthorized)
		}
		return c.SendStatus(fiber.StatusNotFound)
	}
	return c.Next()
}

func abortJSONError(c fiber.Ctx, status int, code, msg string) {
	if msg == "" {
		msg = strings.ToLower(strings.TrimPrefix(httpStatusText(status), " "))
	}
	c.Status(status).JSON(entity.Msg{Success: false, Code: code, Msg: msg})
}

func httpStatusText(code int) string {
	switch code {
	case fiber.StatusUnauthorized:
		return "Unauthorized"
	case fiber.StatusNotFound:
		return "Not Found"
	case fiber.StatusInternalServerError:
		return "Internal Server Error"
	case fiber.StatusBadRequest:
		return "Bad Request"
	case fiber.StatusForbidden:
		return "Forbidden"
	default:
		return "Unknown"
	}
}

func (a *APIController) hasSignedAPIHeaders(c fiber.Ctx) bool {
	return strings.TrimSpace(c.Get(nodeauth.HeaderTimestamp)) != "" &&
		strings.TrimSpace(c.Get(nodeauth.HeaderNonce)) != "" &&
		strings.TrimSpace(c.Get(nodeauth.HeaderSignature)) != ""
}

func (a *APIController) verifySignedAPIRequest(c fiber.Ctx, secret string) bool {
	if secret == "" {
		return false
	}
	timestampStr := strings.TrimSpace(c.Get(nodeauth.HeaderTimestamp))
	nonce := strings.TrimSpace(c.Get(nodeauth.HeaderNonce))
	signature := strings.TrimSpace(c.Get(nodeauth.HeaderSignature))
	if timestampStr == "" || nonce == "" || signature == "" {
		return false
	}
	timestamp, err := strconv.ParseInt(timestampStr, 10, 64)
	if err != nil {
		return false
	}
	body := c.Body()
	digest := strings.TrimSpace(c.Get(nodeauth.HeaderBodyDigest))
	if digest == "" || !strings.EqualFold(digest, nodeauth.BodyDigest(body)) {
		return false
	}
	if !nodeauth.Verify(secret, c.Method(), c.Path(), body, timestamp, nonce, signature, time.Now(), 5*time.Minute) {
		return false
	}
	if !acceptReplay(secret, nonce, timestamp) {
		return false
	}
	return true
}

func acceptReplay(secret, nonce string, ts int64) bool {
	key := strings.TrimSpace(secret) + "|" + strings.TrimSpace(nonce)
	if key == "|" {
		return false
	}
	nodeAuthReplay.m.Lock()
	defer nodeAuthReplay.m.Unlock()
	cutoff := time.Now().Add(-10 * time.Minute).Unix()
	for k, v := range nodeAuthReplay.v {
		if v < cutoff {
			delete(nodeAuthReplay.v, k)
		}
	}
	if _, ok := nodeAuthReplay.v[key]; ok {
		return false
	}
	nodeAuthReplay.v[key] = ts
	return true
}

func (a *APIController) initRouter(router fiber.Router, customGeo *service.CustomGeoService) {
	apiPrefix := "/panel/api"
	if global.GetWebServer() != nil && global.GetWebServer().ModeString() != "hub" {
		apiPrefix = "/api/v1"
	}
	api := router.Group(apiPrefix)
	api.Use(a.checkAPIAuth)
	api.Use(middleware.CSRFMiddleware())

	// Inbounds API
	inbounds := api.Group("/inbounds")
	a.inboundController = NewInboundController(inbounds)

	clients := api.Group("/clients")
	NewClientController(clients)
	NewGroupController(clients)

	// Server API
	server := api.Group("/server")
	a.serverController = NewServerController(server)

	// Nodes API
	nodes := api.Group("/nodes")
	a.nodeController = NewNodeController(nodes)
	a.registrationController.nodeService = a.nodeController.nodeService


	// Node registration management (auth required)
	reg := api.Group("/node-registration")
	reg.Post("/generate", a.registrationController.Generate)
	reg.Get("/list", a.registrationController.List)
	reg.Delete("/:id", a.registrationController.Delete)

	NewCustomGeoController(api.Group("/custom-geo"), customGeo)

	// Node registration endpoint — no auth required (uses one-time token)
	regPath := apiPrefix + "/node-registration"
	router.Post(regPath+"/register", a.registrationController.Register)

	// Extra routes
	api.Post("/backuptotgbot", a.BackuptoTgbot)
}

func (a *APIController) BackuptoTgbot(c fiber.Ctx) error {
	a.Tgbot.SendBackupToAdmins()
	return nil
}

// Ensure io is used (Body reader)
var _ = io.ReadAll
