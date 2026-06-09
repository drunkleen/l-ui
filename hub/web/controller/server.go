package controller

import (
	"fmt"
	"regexp"
	"slices"
	"strconv"
	"time"

	"github.com/drunkleen/l-ui/internal/config"
	"github.com/drunkleen/l-ui/internal/database"
	"github.com/drunkleen/l-ui/internal/logger"
	"github.com/drunkleen/l-ui/hub/web/entity"
	"github.com/drunkleen/l-ui/hub/web/global"
	"github.com/drunkleen/l-ui/hub/web/service"
	"github.com/drunkleen/l-ui/hub/web/websocket"

	"github.com/gofiber/fiber/v3"
)

var filenameRegex = regexp.MustCompile(`^[a-zA-Z0-9_\-.]+$`)

// ServerController handles server management and status-related operations.
type ServerController struct {
	BaseController

	serverService      service.ServerService
	settingService     service.SettingService
	panelService       service.PanelService
	xrayMetricsService service.XrayMetricsService
}

type ServerContext struct {
	Mode             string `json:"mode"`
	Version          string `json:"version"`
	DbType           string `json:"dbType"`
	ApiPrefix        string `json:"apiPrefix"`
	LocalXrayEnabled bool   `json:"localXrayEnabled"`
}

// NewServerController creates a new ServerController, initializes routes, and starts background tasks.
func NewServerController(g fiber.Router) *ServerController {
	a := &ServerController{}
	service.RestoreSystemMetrics()
	a.initRouter(g)
	a.startTask()
	return a
}

// initRouter sets up the routes for server status, Xray management, and utility endpoints.
func (a *ServerController) initRouter(g fiber.Router) {
	isHubMode := global.GetWebServer() != nil && global.GetWebServer().ModeString() == "hub"

	g.Get("/getPanelUpdateInfo", a.getPanelUpdateInfo)
	g.Get("/getConfigJson", a.getConfigJson)
	g.Get("/getDb", a.getDb)
	g.Get("/getNewUUID", a.getNewUUID)
	g.Get("/getWebCertFiles", a.getWebCertFiles)
	g.Get("/getNewX25519Cert", a.getNewX25519Cert)
	g.Get("/getNewmldsa65", a.getNewmldsa65)
	g.Get("/getNewmlkem768", a.getNewmlkem768)
	g.Get("/getNewVlessEnc", a.getNewVlessEnc)
	g.Get("/context", a.context)
	g.Get("/health", a.health)
	g.Get("/status", a.status)
	g.Get("/cpuHistory/:bucket", a.getCpuHistoryBucket)
	g.Get("/history/:metric/:bucket", a.getMetricHistoryBucket)

	if !isHubMode {
		g.Get("/xrayMetricsState", a.getXrayMetricsState)
		g.Get("/xrayMetricsHistory/:metric/:bucket", a.getXrayMetricsHistoryBucket)
		g.Get("/xrayObservatory", a.getXrayObservatory)
		g.Get("/xrayObservatoryHistory/:tag/:bucket", a.getXrayObservatoryHistoryBucket)
		g.Get("/getXrayVersion", a.getXrayVersion)
		g.Post("/stopXrayService", a.stopXrayService)
		g.Post("/restartXrayService", a.restartXrayService)
		g.Post("/installXray/:version", a.installXray)
		g.Post("/cleanup", a.cleanup)
		g.Post("/reinstall", a.reinstall)
		g.Post("/rotateToken", a.rotateToken)
		g.Get("/ufw", a.listUfwRules)
		g.Post("/ufw/allow", a.allowUfwPort)
		g.Post("/ufw/deny", a.denyUfwPort)
	}
	g.Post("/updatePanel", a.updatePanel)
	if !isHubMode {
		g.Post("/updateGeofile", a.updateGeofile)
		g.Post("/updateGeofile/:fileName", a.updateGeofile)
		g.Post("/logs/:count", a.getLogs)
		g.Post("/xraylogs/:count", a.getXrayLogs)
	}
	g.Post("/importDB", a.importDB)
	g.Post("/getNewEchCert", a.getNewEchCert)
}

func (a *ServerController) health(c fiber.Ctx) error {
	jsonObj(c, fiber.Map{"ok": true}, nil)
	return nil
}

func (a *ServerController) context(c fiber.Ctx) error {
	mode := "hub"
	apiPrefix := "/panel/api"
	localXrayEnabled := false
	if server := global.GetWebServer(); server != nil {
		mode = server.ModeString()
		if mode == "agent" {
			apiPrefix = "/api/v1"
			localXrayEnabled = true
		}
	}
	jsonObj(c, ServerContext{
		Mode:             mode,
		Version:          config.GetVersion(),
		DbType:           config.GetDBKind(),
		ApiPrefix:        apiPrefix,
		LocalXrayEnabled: localXrayEnabled,
	}, nil)
	return nil
}

// startTask registers the @2s ticker that refreshes server status, samples
// xray metrics, and pushes the new snapshot to all websocket subscribers.
// State + sampling live in ServerService; the controller only orchestrates
// the cross-service side effects (xrayMetrics sample + websocket broadcast).
func (a *ServerController) startTask() {
	c := global.GetWebServer().GetCron()
	if global.GetWebServer() != nil && global.GetWebServer().ModeString() == "hub" {
		_ = a.serverService.RefreshStatus()
		c.AddFunc("@every 10s", func() {
			status := a.serverService.RefreshStatus()
			if status == nil {
				return
			}
		})
		c.AddFunc("@every 1m", func() {
			if err := service.PersistSystemMetrics(); err != nil {
				logger.Warning("persist system metrics failed:", err)
			}
		})
		return
	}
	c.AddFunc("@every 2s", func() {
		status := a.serverService.RefreshStatus()
		if status == nil {
			return
		}
		a.xrayMetricsService.Sample(time.Now())
		websocket.BroadcastStatus(status)
	})
	c.AddFunc("@every 1m", func() {
		if err := service.PersistSystemMetrics(); err != nil {
			logger.Warning("persist system metrics failed:", err)
		}
	})
}

// status returns the current server status information.
func (a *ServerController) status(c fiber.Ctx) error {
	jsonObj(c, a.serverService.LastStatus(), nil)
	return nil
}

func parseHistoryBucket(c fiber.Ctx) (int, bool) {
	bucket, err := strconv.Atoi(c.Params("bucket"))
	if err != nil || bucket <= 0 || !service.IsAllowedHistoryBucket(bucket) {
		jsonMsg(c, "invalid bucket", fmt.Errorf("unsupported bucket"))
		return 0, false
	}
	return bucket, true
}

// getCpuHistoryBucket retrieves aggregated CPU usage history based on the specified time bucket.
// Kept for back-compat; new callers should use /history/cpu/:bucket which
// returns {"t","v"} (uniform across all metrics) instead of {"t","cpu"}.
func (a *ServerController) getCpuHistoryBucket(c fiber.Ctx) error {
	bucket, ok := parseHistoryBucket(c)
	if !ok {
		return nil
	}
	jsonObj(c, a.serverService.AggregateCpuHistory(bucket, 60), nil)
	return nil
}

// getMetricHistoryBucket returns up to 60 buckets of history for a single
// system metric (cpu, mem, netUp, netDown, online, load1/5/15). The
// SystemHistoryModal calls one endpoint per active tab.
func (a *ServerController) getMetricHistoryBucket(c fiber.Ctx) error {
	metric := c.Params("metric")
	if !slices.Contains(service.SystemMetricKeys, metric) {
		jsonMsg(c, "invalid metric", fmt.Errorf("unknown metric"))
		return nil
	}
	bucket, ok := parseHistoryBucket(c)
	if !ok {
		return nil
	}
	jsonObj(c, a.serverService.AggregateSystemMetric(metric, bucket, 60), nil)
	return nil
}

func (a *ServerController) getXrayMetricsState(c fiber.Ctx) error {
	jsonObj(c, a.xrayMetricsService.State(), nil)
	return nil
}

func (a *ServerController) getXrayMetricsHistoryBucket(c fiber.Ctx) error {
	metric := c.Params("metric")
	if !slices.Contains(service.XrayMetricKeys, metric) {
		jsonMsg(c, "invalid metric", fmt.Errorf("unknown metric"))
		return nil
	}
	bucket, ok := parseHistoryBucket(c)
	if !ok {
		return nil
	}
	jsonObj(c, a.xrayMetricsService.AggregateMetric(metric, bucket, 60), nil)
	return nil
}

func (a *ServerController) getXrayObservatory(c fiber.Ctx) error {
	jsonObj(c, a.xrayMetricsService.ObservatorySnapshot(), nil)
	return nil
}

func (a *ServerController) getXrayObservatoryHistoryBucket(c fiber.Ctx) error {
	tag := c.Params("tag")
	if !a.xrayMetricsService.HasObservatoryTag(tag) {
		jsonMsg(c, "invalid tag", fmt.Errorf("unknown observatory tag"))
		return nil
	}
	bucket, ok := parseHistoryBucket(c)
	if !ok {
		return nil
	}
	jsonObj(c, a.xrayMetricsService.AggregateObservatory(tag, bucket, 60), nil)
	return nil
}

func (a *ServerController) getXrayVersion(c fiber.Ctx) error {
	versions, err := a.serverService.GetXrayVersionsCached()
	if err != nil {
		jsonMsg(c, I18nWeb(c, "getVersion"), err)
		return nil
	}
	jsonObj(c, versions, nil)
	return nil
}

// getPanelUpdateInfo retrieves the current and latest panel version.
func (a *ServerController) getPanelUpdateInfo(c fiber.Ctx) error {
	info, err := a.panelService.GetUpdateInfo()
	if err != nil {
		logger.Debug("panel update check failed:", err)
		c.Status(fiber.StatusOK).JSON(entity.Msg{Success: false})
		return nil
	}
	jsonObj(c, info, nil)
	return nil
}

// installXray installs or updates Xray to the specified version.
func (a *ServerController) installXray(c fiber.Ctx) error {
	version := c.Params("version")
	err := a.serverService.UpdateXray(version)
	jsonMsg(c, I18nWeb(c, "pages.index.xraySwitchVersionPopover"), err)
	return nil
}

// updatePanel starts a panel self-update to the latest release.
func (a *ServerController) updatePanel(c fiber.Ctx) error {
	err := a.panelService.StartUpdate()
	jsonMsg(c, I18nWeb(c, "pages.index.panelUpdateStartedPopover"), err)
	return nil
}

// updateGeofile updates the specified geo file for Xray.
func (a *ServerController) updateGeofile(c fiber.Ctx) error {
	fileName := c.Params("fileName")

	if fileName != "" && !a.serverService.IsValidGeofileName(fileName) {
		jsonMsg(c, I18nWeb(c, "pages.index.geofileUpdatePopover"),
			fmt.Errorf("invalid filename: contains unsafe characters or path traversal patterns"))
		return nil
	}

	err := a.serverService.UpdateGeofile(fileName)
	jsonMsg(c, I18nWeb(c, "pages.index.geofileUpdatePopover"), err)
	return nil
}

// stopXrayService stops the Xray service.
func (a *ServerController) stopXrayService(c fiber.Ctx) error {
	err := a.serverService.StopXrayService()
	if err != nil {
		jsonMsg(c, I18nWeb(c, "pages.xray.stopError"), err)
		websocket.BroadcastXrayState("error", err.Error())
		return nil
	}
	jsonMsg(c, I18nWeb(c, "pages.xray.stopSuccess"), err)
	websocket.BroadcastXrayState("stop", "")
	websocket.BroadcastNotification(
		I18nWeb(c, "pages.xray.stopSuccess"),
		"Xray service has been stopped",
		"warning",
	)
	return nil
}

// restartXrayService restarts the Xray service.
func (a *ServerController) restartXrayService(c fiber.Ctx) error {
	err := a.serverService.RestartXrayService()
	if err != nil {
		jsonMsg(c, I18nWeb(c, "pages.xray.restartError"), err)
		websocket.BroadcastXrayState("error", err.Error())
		return nil
	}
	jsonMsg(c, I18nWeb(c, "pages.xray.restartSuccess"), err)
	websocket.BroadcastXrayState("running", "")
	websocket.BroadcastNotification(
		I18nWeb(c, "pages.xray.restartSuccess"),
		"Xray service has been restarted successfully",
		"success",
	)
	return nil
}

func (a *ServerController) cleanup(c fiber.Ctx) error {
	jsonMsg(c, I18nWeb(c, "pages.nodes.toasts.delete"), a.serverService.CleanupNodeInstall())
	return nil
}

func (a *ServerController) reinstall(c fiber.Ctx) error {
	fh, err := c.FormFile("bundle")
	if err != nil {
		jsonMsg(c, I18nWeb(c, "pages.nodes.toasts.update"), err)
		return nil
	}
	file, err := fh.Open()
	if err != nil {
		jsonMsg(c, I18nWeb(c, "pages.nodes.toasts.update"), err)
		return nil
	}
	defer file.Close()
	jsonMsg(c, I18nWeb(c, "pages.nodes.toasts.update"), a.serverService.ReinstallNodeBundle(file))
	return nil
}

func (a *ServerController) rotateToken(c fiber.Ctx) error {
	var req struct {
		Token string `json:"token"`
	}
	if err := c.Bind().Body(&req); err != nil {
		jsonMsg(c, I18nWeb(c, "pages.nodes.toasts.update"), err)
		return nil
	}
	jsonMsg(c, I18nWeb(c, "pages.nodes.toasts.update"), a.serverService.RotateBootstrapToken(req.Token))
	return nil
}

func (a *ServerController) listUfwRules(c fiber.Ctx) error {
	status, err := a.serverService.ListUfwRules()
	if err != nil {
		jsonMsg(c, I18nWeb(c, "get"), err)
		return nil
	}
	jsonObj(c, status, nil)
	return nil
}

func (a *ServerController) ufwPortBody(c fiber.Ctx) (int, string, bool) {
	var req struct {
		Port     int    `json:"port" form:"port"`
		Protocol string `json:"protocol" form:"protocol"`
	}
	if err := c.Bind().Body(&req); err != nil {
		jsonMsg(c, I18nWeb(c, "somethingWentWrong"), err)
		return 0, "", false
	}
	return req.Port, req.Protocol, true
}

func (a *ServerController) allowUfwPort(c fiber.Ctx) error {
	port, protocol, ok := a.ufwPortBody(c)
	if !ok {
		return nil
	}
	jsonMsg(c, I18nWeb(c, "pages.nodes.toasts.update"), a.serverService.AllowUfwPort(port, protocol))
	return nil
}

func (a *ServerController) denyUfwPort(c fiber.Ctx) error {
	port, protocol, ok := a.ufwPortBody(c)
	if !ok {
		return nil
	}
	jsonMsg(c, I18nWeb(c, "pages.nodes.toasts.update"), a.serverService.DenyUfwPort(port, protocol))
	return nil
}

// getLogs retrieves the application logs based on count, level, and syslog filters.
func (a *ServerController) getLogs(c fiber.Ctx) error {
	logs := a.serverService.GetLogs(c.Params("count"), c.FormValue("level"), c.FormValue("syslog"))
	jsonObj(c, logs, nil)
	return nil
}

// getXrayLogs retrieves Xray logs with filtering options for direct, blocked, and proxy traffic.
func (a *ServerController) getXrayLogs(c fiber.Ctx) error {
	freedoms, blackholes := a.serverService.GetDefaultLogOutboundTags()
	logs := a.serverService.GetXrayLogs(
		c.Params("count"),
		c.FormValue("filter"),
		c.FormValue("showDirect"),
		c.FormValue("showBlocked"),
		c.FormValue("showProxy"),
		freedoms,
		blackholes,
	)
	jsonObj(c, logs, nil)
	return nil
}

// getConfigJson retrieves the Xray configuration as JSON.
func (a *ServerController) getConfigJson(c fiber.Ctx) error {
	configJson, err := a.serverService.GetConfigJson()
	if err != nil {
		jsonMsg(c, I18nWeb(c, "pages.index.getConfigError"), err)
		return nil
	}
	jsonObj(c, configJson, nil)
	return nil
}

// getDb downloads the database file.
func (a *ServerController) getDb(c fiber.Ctx) error {
	db, err := a.serverService.GetDb()
	if err != nil {
		jsonMsg(c, I18nWeb(c, "pages.index.getDatabaseError"), err)
		return nil
	}

	filename := "l-ui.db"
	if database.IsPostgres() {
		filename = "l-ui.dump"
	}
	if !filenameRegex.MatchString(filename) {
		return c.Status(fiber.StatusBadRequest).SendString("invalid filename")
	}

	c.Set("Content-Type", "application/octet-stream")
	c.Set("Content-Disposition", "attachment; filename="+filename)
	return c.Send(db)
}

// importDB imports a database file and restarts the Xray service.
func (a *ServerController) importDB(c fiber.Ctx) error {
	fh, err := c.FormFile("db")
	if err != nil {
		jsonMsg(c, I18nWeb(c, "pages.index.readDatabaseError"), err)
		return nil
	}
	file, err := fh.Open()
	if err != nil {
		jsonMsg(c, I18nWeb(c, "pages.index.readDatabaseError"), err)
		return nil
	}
	defer file.Close()
	if err := a.serverService.ImportDB(file); err != nil {
		jsonMsg(c, I18nWeb(c, "pages.index.importDatabaseError"), err)
		return nil
	}
	jsonObj(c, I18nWeb(c, "pages.index.importDatabaseSuccess"), nil)
	return nil
}

// getWebCertFiles returns this panel's own web TLS certificate and key file
// paths. The central panel calls it on a node (via the node's API token) so
// "Set Cert from Panel" can fill a node-assigned inbound with paths that exist
// on the node's filesystem instead of the central panel's — see issue #4854.
func (a *ServerController) getWebCertFiles(c fiber.Ctx) error {
	certFile, err := a.settingService.GetCertFile()
	if err != nil {
		jsonMsg(c, I18nWeb(c, "somethingWentWrong"), err)
		return nil
	}
	keyFile, err := a.settingService.GetKeyFile()
	if err != nil {
		jsonMsg(c, I18nWeb(c, "somethingWentWrong"), err)
		return nil
	}
	jsonObj(c, fiber.Map{"webCertFile": certFile, "webKeyFile": keyFile}, nil)
	return nil
}

// getNewX25519Cert generates a new X25519 certificate.
func (a *ServerController) getNewX25519Cert(c fiber.Ctx) error {
	cert, err := a.serverService.GetNewX25519Cert()
	if err != nil {
		jsonMsg(c, I18nWeb(c, "pages.inbounds.toasts.getNewX25519CertError"), err)
		return nil
	}
	jsonObj(c, cert, nil)
	return nil
}

// getNewmldsa65 generates a new ML-DSA-65 key.
func (a *ServerController) getNewmldsa65(c fiber.Ctx) error {
	cert, err := a.serverService.GetNewmldsa65()
	if err != nil {
		jsonMsg(c, I18nWeb(c, "pages.inbounds.toasts.getNewmldsa65Error"), err)
		return nil
	}
	jsonObj(c, cert, nil)
	return nil
}

// getNewEchCert generates a new ECH certificate for the given SNI.
func (a *ServerController) getNewEchCert(c fiber.Ctx) error {
	cert, err := a.serverService.GetNewEchCert(c.FormValue("sni"))
	if err != nil {
		jsonMsg(c, "get ech certificate", err)
		return nil
	}
	jsonObj(c, cert, nil)
	return nil
}

// getNewVlessEnc generates a new VLESS encryption key.
func (a *ServerController) getNewVlessEnc(c fiber.Ctx) error {
	out, err := a.serverService.GetNewVlessEnc()
	if err != nil {
		jsonMsg(c, I18nWeb(c, "pages.inbounds.toasts.getNewVlessEncError"), err)
		return nil
	}
	jsonObj(c, out, nil)
	return nil
}

// getNewUUID generates a new UUID.
func (a *ServerController) getNewUUID(c fiber.Ctx) error {
	uuidResp, err := a.serverService.GetNewUUID()
	if err != nil {
		jsonMsg(c, "Failed to generate UUID", err)
		return nil
	}
	jsonObj(c, uuidResp, nil)
	return nil
}

// getNewmlkem768 generates a new ML-KEM-768 key.
func (a *ServerController) getNewmlkem768(c fiber.Ctx) error {
	out, err := a.serverService.GetNewmlkem768()
	if err != nil {
		jsonMsg(c, "Failed to generate mlkem768 keys", err)
		return nil
	}
	jsonObj(c, out, nil)
	return nil
}
