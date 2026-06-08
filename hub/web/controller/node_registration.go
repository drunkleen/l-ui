package controller

import (
	"errors"
	"net/http"
	"strconv"
	"time"

	"github.com/drunkleen/l-ui/hub/web/entity"
	"github.com/drunkleen/l-ui/hub/web/service"
	"github.com/drunkleen/l-ui/internal/database/model"
	"github.com/drunkleen/l-ui/internal/util/random"

	"github.com/gin-gonic/gin"
)

type RegistrationController struct {
	registrationService service.RegistrationService
	nodeService         service.NodeService
}

func NewRegistrationController() *RegistrationController {
	return &RegistrationController{}
}

type generateTokenForm struct {
	NodeName    string `json:"nodeName" form:"nodeName"`
	NodeAddress string `json:"nodeAddress" form:"nodeAddress"`
	TTLMinutes  int    `json:"ttlMinutes" form:"ttlMinutes"`
}

func (a *RegistrationController) Generate(c *gin.Context) {
	var form generateTokenForm
	if err := c.ShouldBind(&form); err != nil {
		jsonMsg(c, I18nWeb(c, "somethingWentWrong"), err)
		return
	}
	ttl := time.Duration(form.TTLMinutes) * time.Minute
	if ttl <= 0 {
		ttl = 24 * time.Hour
	}
	token, err := a.registrationService.GenerateToken(form.NodeName, form.NodeAddress, ttl)
	if err != nil {
		jsonMsg(c, I18nWeb(c, "somethingWentWrong"), err)
		return
	}
	c.JSON(http.StatusOK, entity.Msg{
		Success: true,
		Msg:     "registration token generated",
		Obj:     token,
	})
}

type registerForm struct {
	Name    string `json:"name" form:"name" validate:"required"`
	Address string `json:"address" form:"address" validate:"required"`
	Port    int    `json:"port" form:"port" validate:"omitempty,gte=1,lte=65535"`
	Version string `json:"version" form:"version"`
}

type registerResponse struct {
	NodeID   int    `json:"nodeId"`
	APIToken string `json:"apiToken"`
}

func (a *RegistrationController) Register(c *gin.Context) {
	auth := c.GetHeader("Authorization")
	if auth == "" {
		c.JSON(http.StatusUnauthorized, entity.Msg{Success: false, Msg: "authorization header is required"})
		return
	}

	var form registerForm
	if err := c.ShouldBindJSON(&form); err != nil {
		jsonMsg(c, "invalid registration payload", err)
		return
	}

	regToken, err := a.registrationService.ValidateToken(auth)
	if err != nil {
		status := http.StatusForbidden
		if errors.Is(err, service.ErrTokenNotFound) {
			status = http.StatusNotFound
		}
		c.JSON(status, entity.Msg{Success: false, Msg: err.Error()})
		return
	}

	nodeName := form.Name
	if nodeName == "" && regToken.NodeName != "" {
		nodeName = regToken.NodeName
	}
	nodeAddress := form.Address
	if nodeAddress == "" && regToken.NodeAddress != "" {
		nodeAddress = regToken.NodeAddress
	}
	port := form.Port
	if port <= 0 {
		port = 2054
	}

	apiToken := random.Seq(48)
	node := &model.Node{
		Name:          nodeName,
		Scheme:        "http",
		Address:       nodeAddress,
		Port:          port,
		BasePath:      "/",
		ApiToken:      apiToken,
		Enable:        true,
		Status:        "online",
		LastHeartbeat: time.Now().UnixMilli(),
	}
	if err := a.nodeService.Create(node); err != nil {
		jsonMsg(c, "failed to create node", err)
		return
	}

	if err := a.registrationService.ConsumeToken(auth, node.Id); err != nil {
		jsonMsg(c, "failed to consume token", err)
		return
	}

	c.JSON(http.StatusOK, entity.Msg{
		Success: true,
		Msg:     "node registered successfully",
		Obj: registerResponse{
			NodeID:   node.Id,
			APIToken: apiToken,
		},
	})
}

func (a *RegistrationController) List(c *gin.Context) {
	tokens, err := a.registrationService.ListTokens()
	if err != nil {
		jsonMsg(c, I18nWeb(c, "somethingWentWrong"), err)
		return
	}
	jsonObj(c, tokens, nil)
}

func (a *RegistrationController) Delete(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		jsonMsg(c, I18nWeb(c, "somethingWentWrong"), err)
		return
	}
	if err := a.registrationService.DeleteToken(id); err != nil {
		jsonMsg(c, I18nWeb(c, "somethingWentWrong"), err)
		return
	}
	jsonMsg(c, I18nWeb(c, "pages.nodes.toasts.delete"), nil)
}
