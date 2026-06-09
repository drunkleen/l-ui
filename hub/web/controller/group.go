package controller

import (
	"strings"

	"github.com/drunkleen/l-ui/internal/util/common"
	"github.com/drunkleen/l-ui/hub/web/service"

	"github.com/gofiber/fiber/v3"
)

type GroupController struct {
	clientService service.ClientService
	xrayService   service.XrayService
}

func NewGroupController(router fiber.Router) *GroupController {
	a := &GroupController{}
	a.initRouter(router)
	return a
}

func (a *GroupController) initRouter(router fiber.Router) {
	router.Get("/groups", a.list)
	router.Get("/groups/:name/emails", a.emails)
	router.Post("/groups/create", a.create)
	router.Post("/groups/rename", a.rename)
	router.Post("/groups/delete", a.delete)
	router.Post("/groups/bulkAdd", a.bulkAdd)
	router.Post("/groups/bulkRemove", a.bulkRemove)
}

func (a *GroupController) list(c fiber.Ctx) error {
	rows, err := a.clientService.ListGroups()
	if err != nil {
		jsonMsg(c, I18nWeb(c, "somethingWentWrong"), err)
		return nil
	}
	jsonObj(c, rows, nil)
	return nil
}

func (a *GroupController) emails(c fiber.Ctx) error {
	name := c.Params("name")
	emails, err := a.clientService.EmailsByGroup(name)
	if err != nil {
		jsonMsg(c, I18nWeb(c, "somethingWentWrong"), err)
		return nil
	}
	jsonObj(c, emails, nil)
	return nil
}

type groupCreateBody struct {
	Name string `json:"name"`
}

func (a *GroupController) create(c fiber.Ctx) error {
	var body groupCreateBody
	if err := c.Bind().JSON(&body); err != nil {
		jsonMsg(c, I18nWeb(c, "somethingWentWrong"), err)
		return nil
	}
	if err := a.clientService.CreateGroup(body.Name); err != nil {
		jsonMsg(c, I18nWeb(c, "somethingWentWrong"), err)
		return nil
	}
	jsonObj(c, fiber.Map{"name": body.Name}, nil)
	notifyClientsChanged()
	return nil
}

type groupRenameBody struct {
	OldName string `json:"oldName"`
	NewName string `json:"newName"`
}

func (a *GroupController) rename(c fiber.Ctx) error {
	var body groupRenameBody
	if err := c.Bind().JSON(&body); err != nil {
		jsonMsg(c, I18nWeb(c, "somethingWentWrong"), err)
		return nil
	}
	affected, err := a.clientService.RenameGroup(body.OldName, body.NewName)
	if err != nil {
		jsonMsg(c, I18nWeb(c, "somethingWentWrong"), err)
		return nil
	}
	a.xrayService.SetToNeedRestart()
	jsonObj(c, fiber.Map{"affected": affected}, nil)
	notifyClientsChanged()
	return nil
}

type groupDeleteBody struct {
	Name string `json:"name"`
}

func (a *GroupController) delete(c fiber.Ctx) error {
	var body groupDeleteBody
	if err := c.Bind().JSON(&body); err != nil {
		jsonMsg(c, I18nWeb(c, "somethingWentWrong"), err)
		return nil
	}
	affected, err := a.clientService.DeleteGroup(body.Name)
	if err != nil {
		jsonMsg(c, I18nWeb(c, "somethingWentWrong"), err)
		return nil
	}
	a.xrayService.SetToNeedRestart()
	jsonObj(c, fiber.Map{"affected": affected}, nil)
	notifyClientsChanged()
	return nil
}

type bulkAddToGroupRequest struct {
	Emails []string `json:"emails"`
	Group  string   `json:"group"`
}

func (a *GroupController) bulkAdd(c fiber.Ctx) error {
	var req bulkAddToGroupRequest
	if err := c.Bind().JSON(&req); err != nil {
		jsonMsg(c, I18nWeb(c, "somethingWentWrong"), err)
		return nil
	}
	if strings.TrimSpace(req.Group) == "" {
		jsonMsg(c, I18nWeb(c, "somethingWentWrong"), common.NewError("group name is required"))
		return nil
	}
	affected, err := a.clientService.AddToGroup(req.Emails, req.Group)
	if err != nil {
		jsonMsg(c, I18nWeb(c, "somethingWentWrong"), err)
		return nil
	}
	jsonObj(c, fiber.Map{"affected": affected}, nil)
	a.xrayService.SetToNeedRestart()
	notifyClientsChanged()
	return nil
}

type bulkRemoveFromGroupRequest struct {
	Emails []string `json:"emails"`
}

func (a *GroupController) bulkRemove(c fiber.Ctx) error {
	var req bulkRemoveFromGroupRequest
	if err := c.Bind().JSON(&req); err != nil {
		jsonMsg(c, I18nWeb(c, "somethingWentWrong"), err)
		return nil
	}
	affected, err := a.clientService.RemoveFromGroup(req.Emails)
	if err != nil {
		jsonMsg(c, I18nWeb(c, "somethingWentWrong"), err)
		return nil
	}
	jsonObj(c, fiber.Map{"affected": affected}, nil)
	a.xrayService.SetToNeedRestart()
	notifyClientsChanged()
	return nil
}
