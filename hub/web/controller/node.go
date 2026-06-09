package controller

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"slices"
	"strconv"
	"time"

	"github.com/drunkleen/l-ui/hub/web/middleware"
	"github.com/drunkleen/l-ui/hub/web/service"
	"github.com/drunkleen/l-ui/internal/database/model"

	"github.com/gofiber/fiber/v3"
)

type NodeController struct {
	nodeService  *service.NodeService
	portGroupSvc *service.PortGroupService
	certSvc      *service.NodeCertService
}

func NewNodeController(router fiber.Router) *NodeController {
	a := &NodeController{
		nodeService:  &service.NodeService{},
		portGroupSvc: service.NewPortGroupService(),
		certSvc:      service.NewNodeCertService(),
	}
	a.initRouter(router)
	return a
}

func (a *NodeController) initRouter(router fiber.Router) {
	router.Get("/list", a.list)
	router.Get("/get/:id", a.get)
	router.Get("/webCert/:id", a.webCert)

	router.Post("/add", a.add)
	router.Post("/update/:id", a.update)
	router.Post("/del/:id", a.del)
	router.Post("/setEnable/:id", a.setEnable)

	router.Post("/test", a.test)
	router.Post("/bootstrap", a.bootstrap)
	router.Get("/bootstrap/:id", a.bootstrapStatus)
	router.Post("/reinstall/:id", a.reinstall)
	router.Post("/rotateCredentials/:id", a.rotateCredentials)
	router.Post("/reconcile/:id", a.reconcile)
	router.Get("/ufw/:id", a.listUfwRules)
	router.Post("/ufw/:id/allow", a.allowUfwPort)
	router.Post("/ufw/:id/deny", a.denyUfwPort)
	router.Post("/ufw/:id/delete", a.deleteUfwRule)
	router.Post("/ufw/:id/enable", a.enableUfw)
	router.Post("/ufw/:id/disable", a.disableUfw)
	router.Post("/updateXray/:id/:version", a.updateXray)
	router.Post("/certFingerprint", a.certFingerprint)
	router.Post("/cert/:id/generate", a.generateCert)
	router.Get("/cert/:id/status", a.certStatus)
	router.Post("/cert/renew", a.renewCerts)
	router.Post("/probe/:id", a.probe)
	router.Post("/updatePanel", a.updatePanel)
	router.Get("/history/:id/:metric/:bucket", a.history)
	router.Get("/logs/:id", a.logs)
	router.Post("/restart/:id", a.restartAgent)
	router.Post("/xray/restart/:id", a.restartXray)
	router.Post("/pushConfig/:id", a.pushNodeConfig)

	router.Get("/groups", a.listNodeGroups)
	router.Post("/:id/setGroup", a.setNodeGroup)

	router.Get("/portGroup/list", a.listPortGroups)
	router.Post("/portGroup/add", a.addPortGroup)
	router.Post("/portGroup/update/:id", a.updatePortGroup)
	router.Post("/portGroup/del/:id", a.deletePortGroup)
	router.Post("/portGroup/push/:id/:group", a.pushPortGroup)
}

func (a *NodeController) list(c fiber.Ctx) error {
	nodes, err := a.nodeService.GetAll()
	if err != nil {
		jsonMsg(c, I18nWeb(c, "pages.nodes.toasts.list"), err)
		return nil
	}
	jsonObj(c, nodes, nil)
	return nil
}

func (a *NodeController) get(c fiber.Ctx) error {
	id, err := strconv.Atoi(c.Params("id"))
	if err != nil {
		jsonMsg(c, I18nWeb(c, "get"), err)
		return nil
	}
	n, err := a.nodeService.GetById(id)
	if err != nil {
		jsonMsg(c, I18nWeb(c, "pages.nodes.toasts.obtain"), err)
		return nil
	}
	jsonObj(c, n, nil)
	return nil
}

// webCert returns the node's own web TLS certificate/key file paths so the
// inbound form's "Set Cert from Panel" can fill paths that exist on the node.
func (a *NodeController) webCert(c fiber.Ctx) error {
	id, err := strconv.Atoi(c.Params("id"))
	if err != nil {
		jsonMsg(c, I18nWeb(c, "get"), err)
		return nil
	}
	files, err := a.nodeService.GetWebCertFiles(id)
	if err != nil {
		jsonMsg(c, I18nWeb(c, "pages.nodes.toasts.obtain"), err)
		return nil
	}
	jsonObj(c, files, nil)
	return nil
}

func (a *NodeController) ensureReachable(c fiber.Ctx, n *model.Node) error {
	ctx, cancel := context.WithTimeout(c.Context(), 6*time.Second)
	defer cancel()
	if _, err := a.nodeService.Probe(ctx, n); err != nil {
		return errors.New(service.FriendlyProbeError(err.Error()))
	}
	return nil
}

func (a *NodeController) add(c fiber.Ctx) error {
	n, ok := middleware.BindAndValidate[model.Node](c)
	if !ok {
		return nil
	}
	if err := a.ensureReachable(c, n); err != nil {
		jsonMsg(c, I18nWeb(c, "pages.nodes.toasts.add"), err)
		return nil
	}
	if err := a.nodeService.Create(n); err != nil {
		jsonMsg(c, I18nWeb(c, "pages.nodes.toasts.add"), err)
		return nil
	}
	jsonMsgObj(c, I18nWeb(c, "pages.nodes.toasts.add"), n, nil)
	return nil
}

func (a *NodeController) update(c fiber.Ctx) error {
	id, err := strconv.Atoi(c.Params("id"))
	if err != nil {
		jsonMsg(c, I18nWeb(c, "get"), err)
		return nil
	}
	n, ok := middleware.BindAndValidate[model.Node](c)
	if !ok {
		return nil
	}
	if err := a.ensureReachable(c, n); err != nil {
		jsonMsg(c, I18nWeb(c, "pages.nodes.toasts.update"), err)
		return nil
	}
	if err := a.nodeService.Update(id, n); err != nil {
		jsonMsg(c, I18nWeb(c, "pages.nodes.toasts.update"), err)
		return nil
	}
	jsonMsg(c, I18nWeb(c, "pages.nodes.toasts.update"), nil)
	return nil
}

func (a *NodeController) del(c fiber.Ctx) error {
	id, err := strconv.Atoi(c.Params("id"))
	if err != nil {
		jsonMsg(c, I18nWeb(c, "get"), err)
		return nil
	}
	body := struct {
		CleanupRemote bool `json:"cleanupRemote" form:"cleanupRemote"`
	}{}
	_ = c.Bind().Body(&body)
	if err := a.nodeService.Delete(id, body.CleanupRemote); err != nil {
		jsonMsg(c, I18nWeb(c, "pages.nodes.toasts.delete"), err)
		return nil
	}
	jsonMsg(c, I18nWeb(c, "pages.nodes.toasts.delete"), nil)
	return nil
}

func (a *NodeController) setEnable(c fiber.Ctx) error {
	id, err := strconv.Atoi(c.Params("id"))
	if err != nil {
		jsonMsg(c, I18nWeb(c, "get"), err)
		return nil
	}
	body := struct {
		Enable bool `json:"enable" form:"enable"`
	}{}
	if err := c.Bind().Body(&body); err != nil {
		jsonMsg(c, I18nWeb(c, "pages.nodes.toasts.update"), err)
		return nil
	}
	if err := a.nodeService.SetEnable(id, body.Enable); err != nil {
		jsonMsg(c, I18nWeb(c, "pages.nodes.toasts.update"), err)
		return nil
	}
	jsonMsg(c, I18nWeb(c, "pages.nodes.toasts.update"), nil)
	return nil
}

func (a *NodeController) test(c fiber.Ctx) error {
	n := &model.Node{}
	if err := c.Bind().Body(n); err != nil {
		jsonMsg(c, I18nWeb(c, "pages.nodes.toasts.test"), err)
		return nil
	}
	if n.Scheme == "" {
		n.Scheme = "https"
	}
	if n.BasePath == "" {
		n.BasePath = "/"
	}

	ctx, cancel := context.WithTimeout(c.Context(), 6*time.Second)
	defer cancel()
	patch, err := a.nodeService.Probe(ctx, n)
	jsonObj(c, patch.ToUI(err == nil), nil)
	return nil
}

func (a *NodeController) bootstrap(c fiber.Ctx) error {
	req, ok := middleware.BindAndValidate[service.NodeBootstrapRequest](c)
	if !ok {
		return nil
	}
	job, err := a.nodeService.StartBootstrap(c.Context(), *req)
	jsonMsgObj(c, I18nWeb(c, "pages.nodes.toasts.add"), job, err)
	return nil
}

func (a *NodeController) bootstrapStatus(c fiber.Ctx) error {
	id := c.Params("id")
	job, ok := a.nodeService.BootstrapJob(id)
	if !ok {
		jsonMsg(c, I18nWeb(c, "pages.nodes.toasts.obtain"), fmt.Errorf("bootstrap job not found"))
		return nil
	}
	jsonObj(c, job, nil)
	return nil
}

func (a *NodeController) reinstall(c fiber.Ctx) error {
	id, err := strconv.Atoi(c.Params("id"))
	if err != nil {
		jsonMsg(c, I18nWeb(c, "get"), err)
		return nil
	}
	jsonMsg(c, I18nWeb(c, "pages.nodes.toasts.update"), a.nodeService.Reinstall(id))
	return nil
}

func (a *NodeController) rotateCredentials(c fiber.Ctx) error {
	id, err := strconv.Atoi(c.Params("id"))
	if err != nil {
		jsonMsg(c, I18nWeb(c, "get"), err)
		return nil
	}
	jsonMsg(c, I18nWeb(c, "pages.nodes.toasts.update"), a.nodeService.RotateCredentials(id))
	return nil
}

func (a *NodeController) reconcile(c fiber.Ctx) error {
	id, err := strconv.Atoi(c.Params("id"))
	if err != nil {
		jsonMsg(c, I18nWeb(c, "get"), err)
		return nil
	}
	jsonMsg(c, I18nWeb(c, "pages.nodes.toasts.update"), a.nodeService.Reconcile(id))
	return nil
}

func (a *NodeController) listUfwRules(c fiber.Ctx) error {
	id, err := strconv.Atoi(c.Params("id"))
	if err != nil {
		jsonMsg(c, I18nWeb(c, "get"), err)
		return nil
	}
	status, err := a.nodeService.ListFirewallRules(id)
	if err != nil {
		jsonMsg(c, I18nWeb(c, "get"), err)
		return nil
	}
	jsonObj(c, status, nil)
	return nil
}

func (a *NodeController) ufwPortBody(c fiber.Ctx) (int, string, bool) {
	var body struct {
		Port     int    `json:"port" form:"port"`
		Protocol string `json:"protocol" form:"protocol"`
	}
	if err := c.Bind().Body(&body); err != nil {
		jsonMsg(c, I18nWeb(c, "somethingWentWrong"), err)
		return 0, "", false
	}
	return body.Port, body.Protocol, true
}

func (a *NodeController) allowUfwPort(c fiber.Ctx) error {
	id, err := strconv.Atoi(c.Params("id"))
	if err != nil {
		jsonMsg(c, I18nWeb(c, "get"), err)
		return nil
	}
	port, protocol, ok := a.ufwPortBody(c)
	if !ok {
		return nil
	}
	jsonMsg(c, I18nWeb(c, "pages.nodes.toasts.update"), a.nodeService.AllowFirewallPort(id, port, protocol))
	return nil
}

func (a *NodeController) denyUfwPort(c fiber.Ctx) error {
	id, err := strconv.Atoi(c.Params("id"))
	if err != nil {
		jsonMsg(c, I18nWeb(c, "get"), err)
		return nil
	}
	port, protocol, ok := a.ufwPortBody(c)
	if !ok {
		return nil
	}
	jsonMsg(c, I18nWeb(c, "pages.nodes.toasts.update"), a.nodeService.DenyFirewallPort(id, port, protocol))
	return nil
}

func (a *NodeController) deleteUfwRule(c fiber.Ctx) error {
	id, err := strconv.Atoi(c.Params("id"))
	if err != nil {
		jsonMsg(c, I18nWeb(c, "get"), err)
		return nil
	}
	var body struct {
		RuleNumber string `json:"rule_number" form:"rule_number"`
	}
	if err := c.Bind().Body(&body); err != nil || body.RuleNumber == "" {
		jsonMsg(c, I18nWeb(c, "somethingWentWrong"), fmt.Errorf("rule_number is required"))
		return nil
	}
	jsonMsg(c, I18nWeb(c, "pages.nodes.toasts.update"), a.nodeService.DeleteFirewallRule(id, body.RuleNumber))
	return nil
}

func (a *NodeController) enableUfw(c fiber.Ctx) error {
	id, err := strconv.Atoi(c.Params("id"))
	if err != nil {
		jsonMsg(c, I18nWeb(c, "get"), err)
		return nil
	}
	jsonMsg(c, I18nWeb(c, "pages.nodes.toasts.update"), a.nodeService.EnableNodeFirewall(id))
	return nil
}

func (a *NodeController) disableUfw(c fiber.Ctx) error {
	id, err := strconv.Atoi(c.Params("id"))
	if err != nil {
		jsonMsg(c, I18nWeb(c, "get"), err)
		return nil
	}
	jsonMsg(c, I18nWeb(c, "pages.nodes.toasts.update"), a.nodeService.DisableNodeFirewall(id))
	return nil
}

func (a *NodeController) updateXray(c fiber.Ctx) error {
	id, err := strconv.Atoi(c.Params("id"))
	if err != nil {
		jsonMsg(c, I18nWeb(c, "get"), err)
		return nil
	}
	version := c.Params("version")
	if version == "" {
		version = "latest"
	}
	jsonMsg(c, I18nWeb(c, "pages.xray.restartSuccess"), a.nodeService.UpdateXray(id, version))
	return nil
}

func (a *NodeController) certFingerprint(c fiber.Ctx) error {
	n := &model.Node{}
	if err := c.Bind().Body(n); err != nil {
		jsonMsg(c, I18nWeb(c, "pages.nodes.toasts.test"), err)
		return nil
	}
	if n.Scheme == "" {
		n.Scheme = "https"
	}
	if n.BasePath == "" {
		n.BasePath = "/"
	}

	ctx, cancel := context.WithTimeout(c.Context(), 6*time.Second)
	defer cancel()
	fp, err := a.nodeService.FetchCertFingerprint(ctx, n)
	if err != nil {
		jsonMsg(c, I18nWeb(c, "pages.nodes.toasts.test"), err)
		return nil
	}
	jsonObj(c, fp, nil)
	return nil
}

func (a *NodeController) generateCert(c fiber.Ctx) error {
	id, err := strconv.Atoi(c.Params("id"))
	if err != nil {
		jsonMsg(c, I18nWeb(c, "get"), err)
		return nil
	}
	node, err := a.nodeService.GetById(id)
	if err != nil {
		jsonMsg(c, I18nWeb(c, "pages.nodes.toasts.obtain"), err)
		return nil
	}

	pair, err := a.certSvc.GenerateNodeCert(node)
	if err != nil {
		jsonMsg(c, I18nWeb(c, "pages.nodes.toasts.test"), err)
		return nil
	}

	ctx, cancel := context.WithTimeout(c.Context(), 10*time.Second)
	defer cancel()
	if err := a.certSvc.PushCert(ctx, node, pair); err != nil {
		jsonMsg(c, I18nWeb(c, "pages.nodes.toasts.test"), err)
		return nil
	}

	jsonMsg(c, I18nWeb(c, "success"), nil)
	return nil
}

func (a *NodeController) certStatus(c fiber.Ctx) error {
	id, err := strconv.Atoi(c.Params("id"))
	if err != nil {
		jsonMsg(c, I18nWeb(c, "get"), err)
		return nil
	}
	node, err := a.nodeService.GetById(id)
	if err != nil {
		jsonMsg(c, I18nWeb(c, "pages.nodes.toasts.obtain"), err)
		return nil
	}

	ctx, cancel := context.WithTimeout(c.Context(), 10*time.Second)
	defer cancel()
	st, err := a.certSvc.GetCertStatus(ctx, node)
	if err != nil {
		jsonMsg(c, I18nWeb(c, "pages.nodes.toasts.test"), err)
		return nil
	}
	jsonObj(c, st, nil)
	return nil
}

func (a *NodeController) renewCerts(c fiber.Ctx) error {
	nodes, err := a.certSvc.GetRenewableNodes()
	if err != nil {
		jsonMsg(c, I18nWeb(c, "get"), err)
		return nil
	}

	var results []map[string]any
	for _, node := range nodes {
		ctx, cancel := context.WithTimeout(c.Context(), 30*time.Second)
		err := a.certSvc.RenewNodeCert(ctx, node)
		cancel()
		r := map[string]any{
			"id":   node.Id,
			"name": node.Name,
		}
		if err != nil {
			r["error"] = err.Error()
		}
		results = append(results, r)
	}
	jsonObj(c, results, nil)
	return nil
}

func (a *NodeController) probe(c fiber.Ctx) error {
	id, err := strconv.Atoi(c.Params("id"))
	if err != nil {
		jsonMsg(c, I18nWeb(c, "get"), err)
		return nil
	}
	n, err := a.nodeService.GetById(id)
	if err != nil {
		jsonMsg(c, I18nWeb(c, "pages.nodes.toasts.obtain"), err)
		return nil
	}
	ctx, cancel := context.WithTimeout(c.Context(), 6*time.Second)
	defer cancel()
	patch, probeErr := a.nodeService.Probe(ctx, n)
	if probeErr != nil {
		patch.Status = "offline"
	} else {
		patch.Status = "online"
	}
	_ = a.nodeService.UpdateHeartbeat(id, patch)
	jsonObj(c, patch.ToUI(probeErr == nil), nil)
	return nil
}

func (a *NodeController) updatePanel(c fiber.Ctx) error {
	var req struct {
		Ids []int `json:"ids"`
	}
	if err := c.Bind().JSON(&req); err != nil {
		jsonMsg(c, I18nWeb(c, "somethingWentWrong"), err)
		return nil
	}
	if len(req.Ids) == 0 {
		jsonMsg(c, I18nWeb(c, "somethingWentWrong"), fmt.Errorf("no nodes selected"))
		return nil
	}
	results, err := a.nodeService.UpdatePanels(req.Ids)
	jsonMsgObj(c, I18nWeb(c, "pages.nodes.toasts.updateStarted"), results, err)
	return nil
}

func (a *NodeController) history(c fiber.Ctx) error {
	id, err := strconv.Atoi(c.Params("id"))
	if err != nil {
		jsonMsg(c, I18nWeb(c, "get"), err)
		return nil
	}
	metric := c.Params("metric")
	if !slices.Contains(service.NodeMetricKeys, metric) {
		jsonMsg(c, "invalid metric", fmt.Errorf("unknown metric"))
		return nil
	}
	bucket, err := strconv.Atoi(c.Params("bucket"))
	if err != nil || bucket <= 0 || !service.IsAllowedHistoryBucket(bucket) {
		jsonMsg(c, "invalid bucket", fmt.Errorf("unsupported bucket"))
		return nil
	}
	jsonObj(c, a.nodeService.AggregateNodeMetric(id, metric, bucket, 60), nil)
	return nil
}

func (a *NodeController) logs(c fiber.Ctx) error {
	id, err := strconv.Atoi(c.Params("id"))
	if err != nil {
		jsonMsg(c, I18nWeb(c, "get"), err)
		return nil
	}
	linesStr := c.Query("lines", "200")
	lines, err := strconv.Atoi(linesStr)
	if err != nil || lines <= 0 || lines > 5000 {
		lines = 200
	}
	logs, err := a.nodeService.FetchLogs(id, lines)
	if err != nil {
		jsonMsg(c, I18nWeb(c, "somethingWentWrong"), err)
		return nil
	}
	jsonObj(c, logs, nil)
	return nil
}

func (a *NodeController) restartAgent(c fiber.Ctx) error {
	id, err := strconv.Atoi(c.Params("id"))
	if err != nil {
		jsonMsg(c, I18nWeb(c, "get"), err)
		return nil
	}
	jsonMsg(c, I18nWeb(c, "pages.nodes.toasts.update"), a.nodeService.RestartAgent(id))
	return nil
}

func (a *NodeController) restartXray(c fiber.Ctx) error {
	id, err := strconv.Atoi(c.Params("id"))
	if err != nil {
		jsonMsg(c, I18nWeb(c, "get"), err)
		return nil
	}
	jsonMsg(c, I18nWeb(c, "pages.nodes.toasts.update"), a.nodeService.RestartXray(id))
	return nil
}

func (a *NodeController) pushNodeConfig(c fiber.Ctx) error {
	id, err := strconv.Atoi(c.Params("id"))
	if err != nil {
		jsonMsg(c, I18nWeb(c, "get"), err)
		return nil
	}
	var body struct {
		HubNodeID   string          `json:"hub_node_id"`
		HubEndpoint string          `json:"hub_endpoint"`
		XrayConfig  json.RawMessage `json:"xray_config"`
		ClientList  json.RawMessage `json:"client_list"`
	}
	if err := c.Bind().JSON(&body); err != nil {
		jsonMsg(c, I18nWeb(c, "somethingWentWrong"), err)
		return nil
	}
	if body.HubNodeID == "" {
		body.HubNodeID = c.Params("id")
	}
	// Assemble payload from database when not provided in the request body
	if len(body.XrayConfig) == 0 && len(body.ClientList) == 0 {
		xc, cl, buildErr := service.BuildNodePushPayload(id)
		if buildErr != nil {
			jsonMsg(c, I18nWeb(c, "somethingWentWrong"), buildErr)
			return nil
		}
		body.XrayConfig = xc
		body.ClientList = cl
	}
	configVersion, err := a.nodeService.PushNodeConfig(id, body.HubNodeID, body.HubEndpoint, body.XrayConfig, body.ClientList)
	if err != nil {
		jsonMsg(c, I18nWeb(c, "pages.nodes.toasts.update"), err)
		return nil
	}
	jsonMsgObj(c, I18nWeb(c, "pages.nodes.toasts.update"), map[string]int{"config_version": configVersion}, nil)
	return nil
}

func (a *NodeController) listNodeGroups(c fiber.Ctx) error {
	groups, err := a.nodeService.ListGroups()
	jsonObj(c, groups, err)
	return nil
}

func (a *NodeController) setNodeGroup(c fiber.Ctx) error {
	id, err := strconv.Atoi(c.Params("id"))
	if err != nil {
		jsonMsg(c, I18nWeb(c, "get"), err)
		return nil
	}
	var body struct {
		Group string `json:"group"`
	}
	if err := c.Bind().JSON(&body); err != nil {
		jsonMsg(c, I18nWeb(c, "somethingWentWrong"), err)
		return nil
	}
	jsonMsg(c, I18nWeb(c, "update"), a.nodeService.SetGroup(id, body.Group))
	return nil
}

type portGroupController struct {
	svc *service.PortGroupService
}

func (a *NodeController) listPortGroups(c fiber.Ctx) error {
	groups, err := a.portGroupSvc.List()
	jsonObj(c, groups, err)
	return nil
}

func (a *NodeController) addPortGroup(c fiber.Ctx) error {
	var body struct {
		Name  string                   `json:"name"`
		Ports []service.PortGroupEntry `json:"ports"`
	}
	if err := c.Bind().JSON(&body); err != nil {
		jsonMsg(c, I18nWeb(c, "somethingWentWrong"), err)
		return nil
	}
	pg, err := a.portGroupSvc.Create(body.Name, body.Ports)
	jsonObj(c, pg, err)
	return nil
}

func (a *NodeController) updatePortGroup(c fiber.Ctx) error {
	id, err := strconv.Atoi(c.Params("id"))
	if err != nil {
		jsonMsg(c, I18nWeb(c, "get"), err)
		return nil
	}
	var body struct {
		Name  string                   `json:"name"`
		Ports []service.PortGroupEntry `json:"ports"`
	}
	if err := c.Bind().JSON(&body); err != nil {
		jsonMsg(c, I18nWeb(c, "somethingWentWrong"), err)
		return nil
	}
	pg, err := a.portGroupSvc.Update(id, body.Name, body.Ports)
	jsonObj(c, pg, err)
	return nil
}

func (a *NodeController) deletePortGroup(c fiber.Ctx) error {
	id, err := strconv.Atoi(c.Params("id"))
	if err != nil {
		jsonMsg(c, I18nWeb(c, "get"), err)
		return nil
	}
	jsonMsg(c, I18nWeb(c, "delete"), a.portGroupSvc.Delete(id))
	return nil
}

func (a *NodeController) pushPortGroup(c fiber.Ctx) error {
	pgID, err := strconv.Atoi(c.Params("id"))
	if err != nil {
		jsonMsg(c, I18nWeb(c, "get"), err)
		return nil
	}
	nodeGroup := c.Params("group")
	jsonMsg(c, I18nWeb(c, "pages.nodes.toasts.update"), a.portGroupSvc.PushToNodeGroup(pgID, nodeGroup))
	return nil
}
