package controller

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/drunkleen/l-ui/hub/web/service"
	"github.com/drunkleen/l-ui/hub/web/websocket"
	"github.com/drunkleen/l-ui/internal/database/model"

	"github.com/gofiber/fiber/v3"
)

func notifyClientsChanged() {
	websocket.BroadcastInvalidate(websocket.MessageTypeClients)
}

type ClientController struct {
	clientService  service.ClientService
	inboundService service.InboundService
	xrayService    service.XrayService
	settingService service.SettingService
}

func NewClientController(router fiber.Router) *ClientController {
	a := &ClientController{}
	a.initRouter(router)
	return a
}

func (a *ClientController) initRouter(router fiber.Router) {
	router.Get("/list", a.list)
	router.Get("/list/paged", a.listPaged)
	router.Get("/get/:email", a.get)
	router.Get("/traffic/:email", a.getTrafficByEmail)
	router.Get("/subLinks/:subId", a.getSubLinks)
	router.Get("/links/:email", a.getClientLinks)

	router.Post("/add", a.create)
	router.Post("/update/:email", a.update)
	router.Post("/del/:email", a.delete)
	router.Post("/:email/attach", a.attach)
	router.Post("/:email/detach", a.detach)
	router.Post("/:email/move", a.move)
	router.Post("/resetAllTraffics", a.resetAllTraffics)
	router.Post("/delDepleted", a.delDepleted)
	router.Post("/bulkAdjust", a.bulkAdjust)
	router.Post("/bulkDel", a.bulkDelete)
	router.Post("/bulkCreate", a.bulkCreate)
	router.Post("/bulkAttach", a.bulkAttach)
	router.Post("/bulkDetach", a.bulkDetach)
	router.Post("/bulkMove", a.bulkMove)
	router.Post("/bulkResetTraffic", a.bulkResetTraffic)
	router.Post("/resetTraffic/:email", a.resetTrafficByEmail)
	router.Post("/updateTraffic/:email", a.updateTrafficByEmail)
	router.Post("/ips/:email", a.getIps)
	router.Post("/clearIps/:email", a.clearIps)
	router.Post("/onlines", a.onlines)
	router.Post("/onlinesByNode", a.onlinesByNode)
	router.Post("/activeInbounds", a.activeInbounds)
	router.Post("/lastOnline", a.lastOnline)
}

func (a *ClientController) list(c fiber.Ctx) error {
	rows, err := a.clientService.List()
	if err != nil {
		jsonMsg(c, I18nWeb(c, "pages.inbounds.toasts.obtain"), err)
		return nil
	}
	jsonObj(c, rows, nil)
	return nil
}

func (a *ClientController) listPaged(c fiber.Ctx) error {
	var params service.ClientPageParams
	if err := c.Bind().Query(&params); err != nil {
		jsonMsg(c, I18nWeb(c, "pages.inbounds.toasts.obtain"), err)
		return nil
	}
	resp, err := a.clientService.ListPaged(&a.inboundService, &a.settingService, params)
	if err != nil {
		jsonMsg(c, I18nWeb(c, "pages.inbounds.toasts.obtain"), err)
		return nil
	}
	jsonObj(c, resp, nil)
	return nil
}

func (a *ClientController) get(c fiber.Ctx) error {
	email := c.Params("email")
	rec, err := a.clientService.GetRecordByEmail(nil, email)
	if err != nil {
		jsonMsg(c, I18nWeb(c, "get"), err)
		return nil
	}
	inboundIds, err := a.clientService.GetInboundIdsForRecord(rec.Id)
	if err != nil {
		jsonMsg(c, I18nWeb(c, "get"), err)
		return nil
	}
	flow, err := a.clientService.EffectiveFlow(nil, rec.Id)
	if err != nil {
		jsonMsg(c, I18nWeb(c, "get"), err)
		return nil
	}
	rec.Flow = flow
	jsonObj(c, fiber.Map{"client": rec, "inboundIds": inboundIds}, nil)
	return nil
}

func (a *ClientController) create(c fiber.Ctx) error {
	var payload service.ClientCreatePayload
	if err := c.Bind().JSON(&payload); err != nil {
		jsonMsg(c, I18nWeb(c, "somethingWentWrong"), err)
		return nil
	}
	needRestart, err := a.clientService.Create(&a.inboundService, &payload)
	if err != nil {
		jsonMsg(c, I18nWeb(c, "somethingWentWrong"), err)
		return nil
	}
	jsonMsg(c, I18nWeb(c, "pages.inbounds.toasts.inboundClientAddSuccess"), nil)
	if needRestart {
		a.xrayService.SetToNeedRestart()
	}
	notifyClientsChanged()
	return nil
}

func (a *ClientController) update(c fiber.Ctx) error {
	email := c.Params("email")
	var updated model.Client
	if err := c.Bind().JSON(&updated); err != nil {
		jsonMsg(c, I18nWeb(c, "somethingWentWrong"), err)
		return nil
	}
	needRestart, err := a.clientService.UpdateByEmail(&a.inboundService, email, updated)
	if err != nil {
		jsonMsg(c, I18nWeb(c, "somethingWentWrong"), err)
		return nil
	}
	jsonMsg(c, I18nWeb(c, "pages.inbounds.toasts.inboundClientUpdateSuccess"), nil)
	if needRestart {
		a.xrayService.SetToNeedRestart()
	}
	notifyClientsChanged()
	return nil
}

func (a *ClientController) delete(c fiber.Ctx) error {
	email := c.Params("email")
	keepTraffic := c.Query("keepTraffic") == "1"
	needRestart, err := a.clientService.DeleteByEmail(&a.inboundService, email, keepTraffic)
	if err != nil {
		jsonMsg(c, I18nWeb(c, "somethingWentWrong"), err)
		return nil
	}
	jsonMsg(c, I18nWeb(c, "pages.inbounds.toasts.inboundClientDeleteSuccess"), nil)
	if needRestart {
		a.xrayService.SetToNeedRestart()
	}
	notifyClientsChanged()
	return nil
}

type attachDetachBody struct {
	InboundIds []int `json:"inboundIds"`
}

type moveBody struct {
	SourceNodeId    int `json:"sourceNodeId"`
	TargetNodeId    int `json:"targetNodeId"`
	TargetInboundId int `json:"targetInboundId"`
}

func (a *ClientController) attach(c fiber.Ctx) error {
	email := c.Params("email")
	var body attachDetachBody
	if err := c.Bind().JSON(&body); err != nil {
		jsonMsg(c, I18nWeb(c, "somethingWentWrong"), err)
		return nil
	}
	needRestart, err := a.clientService.AttachByEmail(&a.inboundService, email, body.InboundIds)
	if err != nil {
		jsonMsg(c, I18nWeb(c, "somethingWentWrong"), err)
		return nil
	}
	jsonMsg(c, I18nWeb(c, "pages.inbounds.toasts.inboundClientAddSuccess"), nil)
	if needRestart {
		a.xrayService.SetToNeedRestart()
	}
	notifyClientsChanged()
	return nil
}

func (a *ClientController) move(c fiber.Ctx) error {
	email := c.Params("email")
	var req moveBody
	if err := c.Bind().JSON(&req); err != nil {
		jsonMsg(c, I18nWeb(c, "somethingWentWrong"), err)
		return nil
	}
	needRestart, err := a.clientService.Move(&a.inboundService, email, req.SourceNodeId, req.TargetNodeId, req.TargetInboundId)
	if err != nil {
		jsonMsg(c, I18nWeb(c, "somethingWentWrong"), err)
		return nil
	}
	jsonObj(c, fiber.Map{"moved": []string{email}}, nil)
	if needRestart {
		a.xrayService.SetToNeedRestart()
	}
	notifyClientsChanged()
	return nil
}

func (a *ClientController) resetAllTraffics(c fiber.Ctx) error {
	needRestart, err := a.clientService.ResetAllTraffics()
	if err != nil {
		jsonMsg(c, I18nWeb(c, "somethingWentWrong"), err)
		return nil
	}
	jsonMsg(c, I18nWeb(c, "pages.inbounds.toasts.resetAllClientTrafficSuccess"), nil)
	if needRestart {
		a.xrayService.SetToNeedRestart()
	}
	notifyClientsChanged()
	return nil
}

type bulkAdjustRequest struct {
	Emails   []string `json:"emails"`
	AddDays  int      `json:"addDays"`
	AddBytes int64    `json:"addBytes"`
}

func (a *ClientController) bulkAdjust(c fiber.Ctx) error {
	var req bulkAdjustRequest
	if err := c.Bind().JSON(&req); err != nil {
		jsonMsg(c, I18nWeb(c, "somethingWentWrong"), err)
		return nil
	}
	result, needRestart, err := a.clientService.BulkAdjust(&a.inboundService, req.Emails, req.AddDays, req.AddBytes)
	if err != nil {
		jsonMsg(c, I18nWeb(c, "somethingWentWrong"), err)
		return nil
	}
	jsonObj(c, result, nil)
	if needRestart {
		a.xrayService.SetToNeedRestart()
	}
	notifyClientsChanged()
	return nil
}

type bulkDeleteRequest struct {
	Emails      []string `json:"emails"`
	KeepTraffic bool     `json:"keepTraffic"`
}

type bulkAttachRequest struct {
	Emails     []string `json:"emails"`
	InboundIds []int    `json:"inboundIds"`
}

func (a *ClientController) bulkAttach(c fiber.Ctx) error {
	var req bulkAttachRequest
	if err := c.Bind().JSON(&req); err != nil {
		jsonMsg(c, I18nWeb(c, "somethingWentWrong"), err)
		return nil
	}
	result, needRestart, err := a.clientService.BulkAttach(&a.inboundService, req.Emails, req.InboundIds)
	if err != nil {
		jsonMsg(c, I18nWeb(c, "somethingWentWrong"), err)
		return nil
	}
	jsonObj(c, result, nil)
	if needRestart {
		a.xrayService.SetToNeedRestart()
	}
	notifyClientsChanged()
	return nil
}

type bulkDetachRequest struct {
	Emails     []string `json:"emails"`
	InboundIds []int    `json:"inboundIds"`
}

type bulkMoveRequest struct {
	Emails          []string `json:"emails"`
	SourceNodeId    int      `json:"sourceNodeId"`
	TargetNodeId    int      `json:"targetNodeId"`
	TargetInboundId int      `json:"targetInboundId"`
}

func (a *ClientController) bulkDetach(c fiber.Ctx) error {
	var req bulkDetachRequest
	if err := c.Bind().JSON(&req); err != nil {
		jsonMsg(c, I18nWeb(c, "somethingWentWrong"), err)
		return nil
	}
	result, needRestart, err := a.clientService.BulkDetach(&a.inboundService, req.Emails, req.InboundIds)
	if err != nil {
		jsonMsg(c, I18nWeb(c, "somethingWentWrong"), err)
		return nil
	}
	jsonObj(c, result, nil)
	if needRestart {
		a.xrayService.SetToNeedRestart()
	}
	notifyClientsChanged()
	return nil
}

func (a *ClientController) bulkMove(c fiber.Ctx) error {
	var req bulkMoveRequest
	if err := c.Bind().JSON(&req); err != nil {
		jsonMsg(c, I18nWeb(c, "somethingWentWrong"), err)
		return nil
	}
	result, needRestart, err := a.clientService.BulkMove(&a.inboundService, req.Emails, req.SourceNodeId, req.TargetNodeId, req.TargetInboundId)
	if err != nil {
		jsonMsg(c, I18nWeb(c, "somethingWentWrong"), err)
		return nil
	}
	jsonObj(c, result, nil)
	if needRestart {
		a.xrayService.SetToNeedRestart()
	}
	notifyClientsChanged()
	return nil
}

func (a *ClientController) bulkDelete(c fiber.Ctx) error {
	var req bulkDeleteRequest
	if err := c.Bind().JSON(&req); err != nil {
		jsonMsg(c, I18nWeb(c, "somethingWentWrong"), err)
		return nil
	}
	result, needRestart, err := a.clientService.BulkDelete(&a.inboundService, req.Emails, req.KeepTraffic)
	if err != nil {
		jsonMsg(c, I18nWeb(c, "somethingWentWrong"), err)
		return nil
	}
	jsonObj(c, result, nil)
	if needRestart {
		a.xrayService.SetToNeedRestart()
	}
	notifyClientsChanged()
	return nil
}

func (a *ClientController) bulkCreate(c fiber.Ctx) error {
	var payloads []service.ClientCreatePayload
	if err := c.Bind().JSON(&payloads); err != nil {
		jsonMsg(c, I18nWeb(c, "somethingWentWrong"), err)
		return nil
	}
	result, needRestart, err := a.clientService.BulkCreate(&a.inboundService, payloads)
	if err != nil {
		jsonMsg(c, I18nWeb(c, "somethingWentWrong"), err)
		return nil
	}
	jsonObj(c, result, nil)
	if needRestart {
		a.xrayService.SetToNeedRestart()
	}
	notifyClientsChanged()
	return nil
}

func (a *ClientController) delDepleted(c fiber.Ctx) error {
	deleted, needRestart, err := a.clientService.DelDepleted(&a.inboundService)
	if err != nil {
		jsonMsg(c, I18nWeb(c, "somethingWentWrong"), err)
		return nil
	}
	jsonObj(c, fiber.Map{"deleted": deleted}, nil)
	if needRestart {
		a.xrayService.SetToNeedRestart()
	}
	notifyClientsChanged()
	return nil
}

func (a *ClientController) resetTrafficByEmail(c fiber.Ctx) error {
	email := c.Params("email")
	needRestart, err := a.clientService.ResetTrafficByEmail(&a.inboundService, email)
	if err != nil {
		jsonMsg(c, I18nWeb(c, "somethingWentWrong"), err)
		return nil
	}
	jsonMsg(c, I18nWeb(c, "pages.inbounds.toasts.resetInboundClientTrafficSuccess"), nil)
	if needRestart {
		a.xrayService.SetToNeedRestart()
	}
	notifyClientsChanged()
	return nil
}

type trafficUpdateRequest struct {
	Upload   int64 `json:"upload"`
	Download int64 `json:"download"`
}

func (a *ClientController) updateTrafficByEmail(c fiber.Ctx) error {
	email := c.Params("email")
	var req trafficUpdateRequest
	if err := c.Bind().JSON(&req); err != nil {
		jsonMsg(c, I18nWeb(c, "somethingWentWrong"), err)
		return nil
	}
	if err := a.inboundService.UpdateClientTrafficByEmail(email, req.Upload, req.Download); err != nil {
		jsonMsg(c, I18nWeb(c, "somethingWentWrong"), err)
		return nil
	}
	jsonMsg(c, I18nWeb(c, "pages.inbounds.toasts.inboundClientUpdateSuccess"), nil)
	notifyClientsChanged()
	return nil
}

func (a *ClientController) getIps(c fiber.Ctx) error {
	email := c.Params("email")
	ips, err := a.inboundService.GetInboundClientIps(email)
	if err != nil || ips == "" {
		jsonObj(c, "No IP Record", nil)
		return nil
	}
	type ipWithTimestamp struct {
		IP        string `json:"ip"`
		Timestamp int64  `json:"timestamp"`
	}
	var ipsWithTime []ipWithTimestamp
	if err := json.Unmarshal([]byte(ips), &ipsWithTime); err == nil && len(ipsWithTime) > 0 {
		formatted := make([]string, 0, len(ipsWithTime))
		for _, item := range ipsWithTime {
			if item.IP == "" {
				continue
			}
			if item.Timestamp > 0 {
				ts := time.Unix(item.Timestamp, 0).Local().Format("2006-01-02 15:04:05")
				formatted = append(formatted, fmt.Sprintf("%s (%s)", item.IP, ts))
				continue
			}
			formatted = append(formatted, item.IP)
		}
		jsonObj(c, formatted, nil)
		return nil
	}
	var oldIps []string
	if err := json.Unmarshal([]byte(ips), &oldIps); err == nil && len(oldIps) > 0 {
		jsonObj(c, oldIps, nil)
		return nil
	}
	jsonObj(c, ips, nil)
	return nil
}

func (a *ClientController) clearIps(c fiber.Ctx) error {
	email := c.Params("email")
	if err := a.inboundService.ClearClientIps(email); err != nil {
		jsonMsg(c, I18nWeb(c, "pages.inbounds.toasts.updateSuccess"), err)
		return nil
	}
	jsonMsg(c, I18nWeb(c, "pages.inbounds.toasts.logCleanSuccess"), nil)
	return nil
}

func (a *ClientController) onlines(c fiber.Ctx) error {
	jsonObj(c, a.inboundService.GetOnlineClients(), nil)
	return nil
}

func (a *ClientController) onlinesByNode(c fiber.Ctx) error {
	jsonObj(c, a.inboundService.GetOnlineClientsByNode(), nil)
	return nil
}

func (a *ClientController) activeInbounds(c fiber.Ctx) error {
	jsonObj(c, a.inboundService.GetActiveInboundsByNode(), nil)
	return nil
}

func (a *ClientController) lastOnline(c fiber.Ctx) error {
	data, err := a.inboundService.GetClientsLastOnline()
	jsonObj(c, data, err)
	return nil
}

func (a *ClientController) getTrafficByEmail(c fiber.Ctx) error {
	email := c.Params("email")
	traffic, err := a.inboundService.GetClientTrafficByEmail(email)
	if err != nil {
		jsonMsg(c, I18nWeb(c, "pages.inbounds.toasts.trafficGetError"), err)
		return nil
	}
	jsonObj(c, traffic, nil)
	return nil
}

func (a *ClientController) getSubLinks(c fiber.Ctx) error {
	links, err := a.inboundService.GetSubLinks(c.Hostname(), c.Params("subId"))
	if err != nil {
		jsonMsg(c, I18nWeb(c, "pages.inbounds.toasts.obtain"), err)
		return nil
	}
	jsonObj(c, links, nil)
	return nil
}

func (a *ClientController) getClientLinks(c fiber.Ctx) error {
	links, err := a.inboundService.GetAllClientLinks(c.Hostname(), c.Params("email"))
	if err != nil {
		jsonMsg(c, I18nWeb(c, "pages.inbounds.toasts.obtain"), err)
		return nil
	}
	jsonObj(c, links, nil)
	return nil
}

func (a *ClientController) detach(c fiber.Ctx) error {
	email := c.Params("email")
	var body attachDetachBody
	if err := c.Bind().JSON(&body); err != nil {
		jsonMsg(c, I18nWeb(c, "somethingWentWrong"), err)
		return nil
	}
	needRestart, err := a.clientService.DetachByEmailMany(&a.inboundService, email, body.InboundIds)
	if err != nil {
		jsonMsg(c, I18nWeb(c, "somethingWentWrong"), err)
		return nil
	}
	jsonMsg(c, I18nWeb(c, "pages.inbounds.toasts.inboundClientDeleteSuccess"), nil)
	if needRestart {
		a.xrayService.SetToNeedRestart()
	}
	notifyClientsChanged()
	return nil
}

type bulkResetRequest struct {
	Emails []string `json:"emails"`
}

func (a *ClientController) bulkResetTraffic(c fiber.Ctx) error {
	var req bulkResetRequest
	if err := c.Bind().JSON(&req); err != nil {
		jsonMsg(c, I18nWeb(c, "somethingWentWrong"), err)
		return nil
	}
	affected, err := a.clientService.BulkResetTraffic(&a.inboundService, req.Emails)
	if err != nil {
		jsonMsg(c, I18nWeb(c, "somethingWentWrong"), err)
		return nil
	}
	jsonObj(c, fiber.Map{"affected": affected}, nil)
	a.xrayService.SetToNeedRestart()
	notifyClientsChanged()
	return nil
}
