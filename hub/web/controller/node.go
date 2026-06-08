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

	"github.com/gin-gonic/gin"
)

type NodeController struct {
	nodeService  service.NodeService
	portGroupSvc *service.PortGroupService
	certSvc      *service.NodeCertService
}

func NewNodeController(g *gin.RouterGroup) *NodeController {
	a := &NodeController{
		portGroupSvc: service.NewPortGroupService(),
		certSvc:      service.NewNodeCertService(),
	}
	a.initRouter(g)
	return a
}

func (a *NodeController) initRouter(g *gin.RouterGroup) {
	g.GET("/list", a.list)
	g.GET("/get/:id", a.get)
	g.GET("/webCert/:id", a.webCert)

	g.POST("/add", a.add)
	g.POST("/update/:id", a.update)
	g.POST("/del/:id", a.del)
	g.POST("/setEnable/:id", a.setEnable)

	g.POST("/test", a.test)
	g.POST("/bootstrap", a.bootstrap)
	g.GET("/bootstrap/:id", a.bootstrapStatus)
	g.POST("/reinstall/:id", a.reinstall)
	g.POST("/rotateCredentials/:id", a.rotateCredentials)
	g.POST("/reconcile/:id", a.reconcile)
	g.GET("/ufw/:id", a.listUfwRules)
	g.POST("/ufw/:id/allow", a.allowUfwPort)
	g.POST("/ufw/:id/deny", a.denyUfwPort)
	g.POST("/ufw/:id/delete", a.deleteUfwRule)
	g.POST("/ufw/:id/enable", a.enableUfw)
	g.POST("/ufw/:id/disable", a.disableUfw)
	g.POST("/updateXray/:id/:version", a.updateXray)
	g.POST("/certFingerprint", a.certFingerprint)
	g.POST("/cert/:id/generate", a.generateCert)
	g.GET("/cert/:id/status", a.certStatus)
	g.POST("/cert/renew", a.renewCerts)
	g.POST("/probe/:id", a.probe)
	g.POST("/updatePanel", a.updatePanel)
	g.GET("/history/:id/:metric/:bucket", a.history)
	g.GET("/logs/:id", a.logs)
	g.POST("/restart/:id", a.restartAgent)
	g.POST("/xray/restart/:id", a.restartXray)
	g.POST("/pushConfig/:id", a.pushNodeConfig)

	g.GET("/groups", a.listNodeGroups)
	g.POST("/:id/setGroup", a.setNodeGroup)

	g.GET("/portGroup/list", a.listPortGroups)
	g.POST("/portGroup/add", a.addPortGroup)
	g.POST("/portGroup/update/:id", a.updatePortGroup)
	g.POST("/portGroup/del/:id", a.deletePortGroup)
	g.POST("/portGroup/push/:id/:group", a.pushPortGroup)
}

func (a *NodeController) list(c *gin.Context) {
	nodes, err := a.nodeService.GetAll()
	if err != nil {
		jsonMsg(c, I18nWeb(c, "pages.nodes.toasts.list"), err)
		return
	}
	jsonObj(c, nodes, nil)
}

func (a *NodeController) get(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		jsonMsg(c, I18nWeb(c, "get"), err)
		return
	}
	n, err := a.nodeService.GetById(id)
	if err != nil {
		jsonMsg(c, I18nWeb(c, "pages.nodes.toasts.obtain"), err)
		return
	}
	jsonObj(c, n, nil)
}

// webCert returns the node's own web TLS certificate/key file paths so the
// inbound form's "Set Cert from Panel" can fill paths that exist on the node.
func (a *NodeController) webCert(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		jsonMsg(c, I18nWeb(c, "get"), err)
		return
	}
	files, err := a.nodeService.GetWebCertFiles(id)
	if err != nil {
		jsonMsg(c, I18nWeb(c, "pages.nodes.toasts.obtain"), err)
		return
	}
	jsonObj(c, files, nil)
}

func (a *NodeController) ensureReachable(c *gin.Context, n *model.Node) error {
	ctx, cancel := context.WithTimeout(c.Request.Context(), 6*time.Second)
	defer cancel()
	if _, err := a.nodeService.Probe(ctx, n); err != nil {
		return errors.New(service.FriendlyProbeError(err.Error()))
	}
	return nil
}

func (a *NodeController) add(c *gin.Context) {
	n, ok := middleware.BindAndValidate[model.Node](c)
	if !ok {
		return
	}
	if err := a.ensureReachable(c, n); err != nil {
		jsonMsg(c, I18nWeb(c, "pages.nodes.toasts.add"), err)
		return
	}
	if err := a.nodeService.Create(n); err != nil {
		jsonMsg(c, I18nWeb(c, "pages.nodes.toasts.add"), err)
		return
	}
	jsonMsgObj(c, I18nWeb(c, "pages.nodes.toasts.add"), n, nil)
}

func (a *NodeController) update(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		jsonMsg(c, I18nWeb(c, "get"), err)
		return
	}
	n, ok := middleware.BindAndValidate[model.Node](c)
	if !ok {
		return
	}
	if err := a.ensureReachable(c, n); err != nil {
		jsonMsg(c, I18nWeb(c, "pages.nodes.toasts.update"), err)
		return
	}
	if err := a.nodeService.Update(id, n); err != nil {
		jsonMsg(c, I18nWeb(c, "pages.nodes.toasts.update"), err)
		return
	}
	jsonMsg(c, I18nWeb(c, "pages.nodes.toasts.update"), nil)
}

func (a *NodeController) del(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		jsonMsg(c, I18nWeb(c, "get"), err)
		return
	}
	body := struct {
		CleanupRemote bool `json:"cleanupRemote" form:"cleanupRemote"`
	}{}
	_ = c.ShouldBind(&body)
	if err := a.nodeService.Delete(id, body.CleanupRemote); err != nil {
		jsonMsg(c, I18nWeb(c, "pages.nodes.toasts.delete"), err)
		return
	}
	jsonMsg(c, I18nWeb(c, "pages.nodes.toasts.delete"), nil)
}

func (a *NodeController) setEnable(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		jsonMsg(c, I18nWeb(c, "get"), err)
		return
	}
	body := struct {
		Enable bool `json:"enable" form:"enable"`
	}{}
	if err := c.ShouldBind(&body); err != nil {
		jsonMsg(c, I18nWeb(c, "pages.nodes.toasts.update"), err)
		return
	}
	if err := a.nodeService.SetEnable(id, body.Enable); err != nil {
		jsonMsg(c, I18nWeb(c, "pages.nodes.toasts.update"), err)
		return
	}
	jsonMsg(c, I18nWeb(c, "pages.nodes.toasts.update"), nil)
}

func (a *NodeController) test(c *gin.Context) {
	n := &model.Node{}
	if err := c.ShouldBind(n); err != nil {
		jsonMsg(c, I18nWeb(c, "pages.nodes.toasts.test"), err)
		return
	}
	if n.Scheme == "" {
		n.Scheme = "https"
	}
	if n.BasePath == "" {
		n.BasePath = "/"
	}

	ctx, cancel := context.WithTimeout(c.Request.Context(), 6*time.Second)
	defer cancel()
	patch, err := a.nodeService.Probe(ctx, n)
	jsonObj(c, patch.ToUI(err == nil), nil)
}

func (a *NodeController) bootstrap(c *gin.Context) {
	req, ok := middleware.BindAndValidate[service.NodeBootstrapRequest](c)
	if !ok {
		return
	}
	job, err := a.nodeService.StartBootstrap(c.Request.Context(), *req)
	jsonMsgObj(c, I18nWeb(c, "pages.nodes.toasts.add"), job, err)
}

func (a *NodeController) bootstrapStatus(c *gin.Context) {
	id := c.Param("id")
	job, ok := a.nodeService.BootstrapJob(id)
	if !ok {
		jsonMsg(c, I18nWeb(c, "pages.nodes.toasts.obtain"), fmt.Errorf("bootstrap job not found"))
		return
	}
	jsonObj(c, job, nil)
}

func (a *NodeController) reinstall(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		jsonMsg(c, I18nWeb(c, "get"), err)
		return
	}
	jsonMsg(c, I18nWeb(c, "pages.nodes.toasts.update"), a.nodeService.Reinstall(id))
}

func (a *NodeController) rotateCredentials(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		jsonMsg(c, I18nWeb(c, "get"), err)
		return
	}
	jsonMsg(c, I18nWeb(c, "pages.nodes.toasts.update"), a.nodeService.RotateCredentials(id))
}

func (a *NodeController) reconcile(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		jsonMsg(c, I18nWeb(c, "get"), err)
		return
	}
	jsonMsg(c, I18nWeb(c, "pages.nodes.toasts.update"), a.nodeService.Reconcile(id))
}

func (a *NodeController) listUfwRules(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		jsonMsg(c, I18nWeb(c, "get"), err)
		return
	}
	status, err := a.nodeService.ListFirewallRules(id)
	if err != nil {
		jsonMsg(c, I18nWeb(c, "get"), err)
		return
	}
	jsonObj(c, status, nil)
}

func (a *NodeController) ufwPortBody(c *gin.Context) (int, string, bool) {
	var body struct {
		Port     int    `json:"port" form:"port"`
		Protocol string `json:"protocol" form:"protocol"`
	}
	if err := c.ShouldBind(&body); err != nil {
		jsonMsg(c, I18nWeb(c, "somethingWentWrong"), err)
		return 0, "", false
	}
	return body.Port, body.Protocol, true
}

func (a *NodeController) allowUfwPort(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		jsonMsg(c, I18nWeb(c, "get"), err)
		return
	}
	port, protocol, ok := a.ufwPortBody(c)
	if !ok {
		return
	}
	jsonMsg(c, I18nWeb(c, "pages.nodes.toasts.update"), a.nodeService.AllowFirewallPort(id, port, protocol))
}

func (a *NodeController) denyUfwPort(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		jsonMsg(c, I18nWeb(c, "get"), err)
		return
	}
	port, protocol, ok := a.ufwPortBody(c)
	if !ok {
		return
	}
	jsonMsg(c, I18nWeb(c, "pages.nodes.toasts.update"), a.nodeService.DenyFirewallPort(id, port, protocol))
}

func (a *NodeController) deleteUfwRule(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		jsonMsg(c, I18nWeb(c, "get"), err)
		return
	}
	var body struct {
		RuleNumber string `json:"rule_number" form:"rule_number"`
	}
	if err := c.ShouldBind(&body); err != nil || body.RuleNumber == "" {
		jsonMsg(c, I18nWeb(c, "somethingWentWrong"), fmt.Errorf("rule_number is required"))
		return
	}
	jsonMsg(c, I18nWeb(c, "pages.nodes.toasts.update"), a.nodeService.DeleteFirewallRule(id, body.RuleNumber))
}

func (a *NodeController) enableUfw(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		jsonMsg(c, I18nWeb(c, "get"), err)
		return
	}
	jsonMsg(c, I18nWeb(c, "pages.nodes.toasts.update"), a.nodeService.EnableNodeFirewall(id))
}

func (a *NodeController) disableUfw(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		jsonMsg(c, I18nWeb(c, "get"), err)
		return
	}
	jsonMsg(c, I18nWeb(c, "pages.nodes.toasts.update"), a.nodeService.DisableNodeFirewall(id))
}

func (a *NodeController) updateXray(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		jsonMsg(c, I18nWeb(c, "get"), err)
		return
	}
	version := c.Param("version")
	if version == "" {
		version = "latest"
	}
	jsonMsg(c, I18nWeb(c, "pages.xray.restartSuccess"), a.nodeService.UpdateXray(id, version))
}

func (a *NodeController) certFingerprint(c *gin.Context) {
	n := &model.Node{}
	if err := c.ShouldBind(n); err != nil {
		jsonMsg(c, I18nWeb(c, "pages.nodes.toasts.test"), err)
		return
	}
	if n.Scheme == "" {
		n.Scheme = "https"
	}
	if n.BasePath == "" {
		n.BasePath = "/"
	}

	ctx, cancel := context.WithTimeout(c.Request.Context(), 6*time.Second)
	defer cancel()
	fp, err := a.nodeService.FetchCertFingerprint(ctx, n)
	if err != nil {
		jsonMsg(c, I18nWeb(c, "pages.nodes.toasts.test"), err)
		return
	}
	jsonObj(c, fp, nil)
}

func (a *NodeController) generateCert(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		jsonMsg(c, I18nWeb(c, "get"), err)
		return
	}
	node, err := a.nodeService.GetById(id)
	if err != nil {
		jsonMsg(c, I18nWeb(c, "pages.nodes.toasts.obtain"), err)
		return
	}

	pair, err := a.certSvc.GenerateNodeCert(node)
	if err != nil {
		jsonMsg(c, I18nWeb(c, "pages.nodes.toasts.test"), err)
		return
	}

	ctx, cancel := context.WithTimeout(c.Request.Context(), 10*time.Second)
	defer cancel()
	if err := a.certSvc.PushCert(ctx, node, pair); err != nil {
		jsonMsg(c, I18nWeb(c, "pages.nodes.toasts.test"), err)
		return
	}

	jsonMsg(c, I18nWeb(c, "success"), nil)
}

func (a *NodeController) certStatus(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		jsonMsg(c, I18nWeb(c, "get"), err)
		return
	}
	node, err := a.nodeService.GetById(id)
	if err != nil {
		jsonMsg(c, I18nWeb(c, "pages.nodes.toasts.obtain"), err)
		return
	}

	ctx, cancel := context.WithTimeout(c.Request.Context(), 10*time.Second)
	defer cancel()
	st, err := a.certSvc.GetCertStatus(ctx, node)
	if err != nil {
		jsonMsg(c, I18nWeb(c, "pages.nodes.toasts.test"), err)
		return
	}
	jsonObj(c, st, nil)
}

func (a *NodeController) renewCerts(c *gin.Context) {
	nodes, err := a.certSvc.GetRenewableNodes()
	if err != nil {
		jsonMsg(c, I18nWeb(c, "get"), err)
		return
	}

	var results []map[string]any
	for _, node := range nodes {
		ctx, cancel := context.WithTimeout(c.Request.Context(), 30*time.Second)
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
}

func (a *NodeController) probe(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		jsonMsg(c, I18nWeb(c, "get"), err)
		return
	}
	n, err := a.nodeService.GetById(id)
	if err != nil {
		jsonMsg(c, I18nWeb(c, "pages.nodes.toasts.obtain"), err)
		return
	}
	ctx, cancel := context.WithTimeout(c.Request.Context(), 6*time.Second)
	defer cancel()
	patch, probeErr := a.nodeService.Probe(ctx, n)
	if probeErr != nil {
		patch.Status = "offline"
	} else {
		patch.Status = "online"
	}
	_ = a.nodeService.UpdateHeartbeat(id, patch)
	jsonObj(c, patch.ToUI(probeErr == nil), nil)
}

func (a *NodeController) updatePanel(c *gin.Context) {
	var req struct {
		Ids []int `json:"ids"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		jsonMsg(c, I18nWeb(c, "somethingWentWrong"), err)
		return
	}
	if len(req.Ids) == 0 {
		jsonMsg(c, I18nWeb(c, "somethingWentWrong"), fmt.Errorf("no nodes selected"))
		return
	}
	results, err := a.nodeService.UpdatePanels(req.Ids)
	jsonMsgObj(c, I18nWeb(c, "pages.nodes.toasts.updateStarted"), results, err)
}

func (a *NodeController) history(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		jsonMsg(c, I18nWeb(c, "get"), err)
		return
	}
	metric := c.Param("metric")
	if !slices.Contains(service.NodeMetricKeys, metric) {
		jsonMsg(c, "invalid metric", fmt.Errorf("unknown metric"))
		return
	}
	bucket, err := strconv.Atoi(c.Param("bucket"))
	if err != nil || bucket <= 0 || !service.IsAllowedHistoryBucket(bucket) {
		jsonMsg(c, "invalid bucket", fmt.Errorf("unsupported bucket"))
		return
	}
	jsonObj(c, a.nodeService.AggregateNodeMetric(id, metric, bucket, 60), nil)
}

func (a *NodeController) logs(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		jsonMsg(c, I18nWeb(c, "get"), err)
		return
	}
	linesStr := c.DefaultQuery("lines", "200")
	lines, err := strconv.Atoi(linesStr)
	if err != nil || lines <= 0 || lines > 5000 {
		lines = 200
	}
	logs, err := a.nodeService.FetchLogs(id, lines)
	if err != nil {
		jsonMsg(c, I18nWeb(c, "somethingWentWrong"), err)
		return
	}
	jsonObj(c, logs, nil)
}

func (a *NodeController) restartAgent(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		jsonMsg(c, I18nWeb(c, "get"), err)
		return
	}
	jsonMsg(c, I18nWeb(c, "pages.nodes.toasts.update"), a.nodeService.RestartAgent(id))
}

func (a *NodeController) restartXray(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		jsonMsg(c, I18nWeb(c, "get"), err)
		return
	}
	jsonMsg(c, I18nWeb(c, "pages.nodes.toasts.update"), a.nodeService.RestartXray(id))
}

func (a *NodeController) pushNodeConfig(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		jsonMsg(c, I18nWeb(c, "get"), err)
		return
	}
	var body struct {
		HubNodeID   string          `json:"hub_node_id"`
		HubEndpoint string          `json:"hub_endpoint"`
		XrayConfig  json.RawMessage `json:"xray_config"`
		ClientList  json.RawMessage `json:"client_list"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		jsonMsg(c, I18nWeb(c, "somethingWentWrong"), err)
		return
	}
	if body.HubNodeID == "" {
		body.HubNodeID = c.Param("id")
	}
	// Assemble payload from database when not provided in the request body
	if len(body.XrayConfig) == 0 && len(body.ClientList) == 0 {
		xc, cl, buildErr := service.BuildNodePushPayload(id)
		if buildErr != nil {
			jsonMsg(c, I18nWeb(c, "somethingWentWrong"), buildErr)
			return
		}
		body.XrayConfig = xc
		body.ClientList = cl
	}
	configVersion, err := a.nodeService.PushNodeConfig(id, body.HubNodeID, body.HubEndpoint, body.XrayConfig, body.ClientList)
	if err != nil {
		jsonMsg(c, I18nWeb(c, "pages.nodes.toasts.update"), err)
		return
	}
	jsonMsgObj(c, I18nWeb(c, "pages.nodes.toasts.update"), map[string]int{"config_version": configVersion}, nil)
}

func (a *NodeController) listNodeGroups(c *gin.Context) {
	groups, err := a.nodeService.ListGroups()
	jsonObj(c, groups, err)
}

func (a *NodeController) setNodeGroup(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		jsonMsg(c, I18nWeb(c, "get"), err)
		return
	}
	var body struct {
		Group string `json:"group"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		jsonMsg(c, I18nWeb(c, "somethingWentWrong"), err)
		return
	}
	jsonMsg(c, I18nWeb(c, "update"), a.nodeService.SetGroup(id, body.Group))
}

type portGroupController struct {
	svc *service.PortGroupService
}

func (a *NodeController) listPortGroups(c *gin.Context) {
	groups, err := a.portGroupSvc.List()
	jsonObj(c, groups, err)
}

func (a *NodeController) addPortGroup(c *gin.Context) {
	var body struct {
		Name  string                   `json:"name"`
		Ports []service.PortGroupEntry `json:"ports"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		jsonMsg(c, I18nWeb(c, "somethingWentWrong"), err)
		return
	}
	pg, err := a.portGroupSvc.Create(body.Name, body.Ports)
	jsonObj(c, pg, err)
}

func (a *NodeController) updatePortGroup(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		jsonMsg(c, I18nWeb(c, "get"), err)
		return
	}
	var body struct {
		Name  string                   `json:"name"`
		Ports []service.PortGroupEntry `json:"ports"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		jsonMsg(c, I18nWeb(c, "somethingWentWrong"), err)
		return
	}
	pg, err := a.portGroupSvc.Update(id, body.Name, body.Ports)
	jsonObj(c, pg, err)
}

func (a *NodeController) deletePortGroup(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		jsonMsg(c, I18nWeb(c, "get"), err)
		return
	}
	jsonMsg(c, I18nWeb(c, "delete"), a.portGroupSvc.Delete(id))
}

func (a *NodeController) pushPortGroup(c *gin.Context) {
	pgID, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		jsonMsg(c, I18nWeb(c, "get"), err)
		return
	}
	nodeGroup := c.Param("group")
	jsonMsg(c, I18nWeb(c, "pages.nodes.toasts.update"), a.portGroupSvc.PushToNodeGroup(pgID, nodeGroup))
}
