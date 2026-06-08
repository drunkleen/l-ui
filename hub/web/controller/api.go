package controller

import (
	"bytes"
	"io"
	"net/http"
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

	"github.com/gin-gonic/gin"
)

var nodeAuthReplay = struct {
	m sync.Mutex
	v map[string]int64
}{v: map[string]int64{}}

// APIController handles the main API routes for the l-ui panel, including inbounds and server management.
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

// NewAPIController creates a new APIController instance and initializes its routes.
func NewAPIController(g *gin.RouterGroup, customGeo *service.CustomGeoService) *APIController {
	a := &APIController{
		registrationController: NewRegistrationController(),
	}
	a.initRouter(g, customGeo)
	return a
}

func (a *APIController) checkAPIAuth(c *gin.Context) {
	auth := c.GetHeader("Authorization")
	if after, ok := strings.CutPrefix(auth, "Bearer "); ok {
		tok := strings.TrimSpace(after)
		if a.hasSignedAPIHeaders(c) && !a.apiTokenService.Match(tok) {
			abortJSONError(c, http.StatusUnauthorized, service.NodeErrAuth, "invalid node credentials")
			return
		}
		if a.hasSignedAPIHeaders(c) {
			if a.apiTokenService.Match(tok) && a.verifySignedAPIRequest(c, tok) {
				if u, err := a.userService.GetFirstUser(); err == nil {
					session.SetAPIAuthUser(c, u)
				}
				c.Set("api_authed", true)
				c.Next()
				return
			}
		} else if a.apiTokenService.Match(tok) {
			if u, err := a.userService.GetFirstUser(); err == nil {
				session.SetAPIAuthUser(c, u)
			}
			c.Set("api_authed", true)
			c.Next()
			return
		}
	}
	if !session.IsLogin(c) {
		if c.GetHeader("X-Requested-With") == "XMLHttpRequest" {
			c.AbortWithStatus(http.StatusUnauthorized)
		} else {
			c.AbortWithStatus(http.StatusNotFound)
		}
		return
	}
	c.Next()
}

func abortJSONError(c *gin.Context, status int, code, msg string) {
	if msg == "" {
		msg = http.StatusText(status)
	}
	c.AbortWithStatusJSON(status, entity.Msg{Success: false, Code: code, Msg: msg})
}

func (a *APIController) hasSignedAPIHeaders(c *gin.Context) bool {
	return strings.TrimSpace(c.GetHeader(nodeauth.HeaderTimestamp)) != "" &&
		strings.TrimSpace(c.GetHeader(nodeauth.HeaderNonce)) != "" &&
		strings.TrimSpace(c.GetHeader(nodeauth.HeaderSignature)) != ""
}

func (a *APIController) verifySignedAPIRequest(c *gin.Context, secret string) bool {
	if secret == "" {
		return false
	}
	timestampStr := strings.TrimSpace(c.GetHeader(nodeauth.HeaderTimestamp))
	nonce := strings.TrimSpace(c.GetHeader(nodeauth.HeaderNonce))
	signature := strings.TrimSpace(c.GetHeader(nodeauth.HeaderSignature))
	if timestampStr == "" || nonce == "" || signature == "" {
		return false
	}
	timestamp, err := strconv.ParseInt(timestampStr, 10, 64)
	if err != nil {
		return false
	}
	body, err := io.ReadAll(c.Request.Body)
	if err != nil {
		return false
	}
	c.Request.Body = io.NopCloser(bytes.NewReader(body))
	digest := strings.TrimSpace(c.GetHeader(nodeauth.HeaderBodyDigest))
	if digest == "" || !strings.EqualFold(digest, nodeauth.BodyDigest(body)) {
		return false
	}
	if !nodeauth.Verify(secret, c.Request.Method, c.Request.URL.Path, body, timestamp, nonce, signature, time.Now(), 5*time.Minute) {
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

// initRouter sets up the API routes for inbounds, server, and other endpoints.
func (a *APIController) initRouter(g *gin.RouterGroup, customGeo *service.CustomGeoService) {
	// Main API group
	apiPrefix := "/panel/api"
	if global.GetWebServer() != nil && global.GetWebServer().ModeString() != "hub" {
		apiPrefix = "/api/v1"
	}
	api := g.Group(apiPrefix)
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

	// Nodes API — multi-panel management
	nodes := api.Group("/nodes")
	a.nodeController = NewNodeController(nodes)
	a.registrationController.nodeService = a.nodeController.nodeService

	// Node registration management (auth required)
	reg := api.Group("/node-registration")
	reg.POST("/generate", a.registrationController.Generate)
	reg.GET("/list", a.registrationController.List)
	reg.DELETE("/:id", a.registrationController.Delete)

	NewCustomGeoController(api.Group("/custom-geo"), customGeo)

	// Node registration endpoint — no auth required (uses one-time token)
	// Registered here (nodeService must be wired first) but outside any auth
	// middleware so agents can call it before they have a secret.
	regPath := apiPrefix + "/node-registration"
	g.POST(regPath+"/register", a.registrationController.Register)

	// Extra routes
	api.POST("/backuptotgbot", a.BackuptoTgbot)
}

// BackuptoTgbot sends a backup of the panel data to Telegram bot admins.
func (a *APIController) BackuptoTgbot(c *gin.Context) {
	a.Tgbot.SendBackupToAdmins()
}
