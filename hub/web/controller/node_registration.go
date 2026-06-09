package controller

import (
	"errors"
	"strconv"
	"time"

	"github.com/drunkleen/l-ui/hub/web/entity"
	"github.com/drunkleen/l-ui/hub/web/service"
	"github.com/drunkleen/l-ui/internal/database/model"
	"github.com/drunkleen/l-ui/internal/util/random"

	"github.com/gofiber/fiber/v3"
)

type RegistrationController struct {
	registrationService service.RegistrationService
	nodeService         *service.NodeService
}

func NewRegistrationController() *RegistrationController {
	return &RegistrationController{}
}

type generateTokenForm struct {
	NodeName    string `json:"nodeName" form:"nodeName"`
	NodeAddress string `json:"nodeAddress" form:"nodeAddress"`
	TTLMinutes  int    `json:"ttlMinutes" form:"ttlMinutes"`
}

func (a *RegistrationController) Generate(c fiber.Ctx) error {
	var form generateTokenForm
	if err := c.Bind().Body(&form); err != nil {
		jsonMsg(c, I18nWeb(c, "somethingWentWrong"), err)
		return nil
	}
	ttl := time.Duration(form.TTLMinutes) * time.Minute
	if ttl <= 0 {
		ttl = 24 * time.Hour
	}
	token, err := a.registrationService.GenerateToken(form.NodeName, form.NodeAddress, ttl)
	if err != nil {
		jsonMsg(c, I18nWeb(c, "somethingWentWrong"), err)
		return nil
	}
	c.Status(fiber.StatusOK).JSON(entity.Msg{
		Success: true,
		Msg:     "registration token generated",
		Obj:     token,
	})
	return nil
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

func (a *RegistrationController) Register(c fiber.Ctx) error {
	auth := c.Get("Authorization")
	if auth == "" {
		c.Status(fiber.StatusUnauthorized).JSON(entity.Msg{Success: false, Msg: "authorization header is required"})
		return nil
	}

	var form registerForm
	if err := c.Bind().JSON(&form); err != nil {
		jsonMsg(c, "invalid registration payload", err)
		return nil
	}

	regToken, err := a.registrationService.ValidateToken(auth)
	if err != nil {
		status := fiber.StatusForbidden
		if errors.Is(err, service.ErrTokenNotFound) {
			status = fiber.StatusNotFound
		}
		c.Status(status).JSON(entity.Msg{Success: false, Msg: err.Error()})
		return nil
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
		return nil
	}

	if err := a.registrationService.ConsumeToken(auth, node.Id); err != nil {
		jsonMsg(c, "failed to consume token", err)
		return nil
	}

	c.Status(fiber.StatusOK).JSON(entity.Msg{
		Success: true,
		Msg:     "node registered successfully",
		Obj: registerResponse{
			NodeID:   node.Id,
			APIToken: apiToken,
		},
	})
	return nil
}

func (a *RegistrationController) List(c fiber.Ctx) error {
	tokens, err := a.registrationService.ListTokens()
	if err != nil {
		jsonMsg(c, I18nWeb(c, "somethingWentWrong"), err)
		return nil
	}
	jsonObj(c, tokens, nil)
	return nil
}

func (a *RegistrationController) Delete(c fiber.Ctx) error {
	id, err := strconv.Atoi(c.Params("id"))
	if err != nil {
		jsonMsg(c, I18nWeb(c, "somethingWentWrong"), err)
		return nil
	}
	if err := a.registrationService.DeleteToken(id); err != nil {
		jsonMsg(c, I18nWeb(c, "somethingWentWrong"), err)
		return nil
	}
	jsonMsg(c, I18nWeb(c, "pages.nodes.toasts.delete"), nil)
	return nil
}
