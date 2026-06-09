package controller

import (
	"encoding/json"

	"github.com/drunkleen/l-ui/hub/web/service"
	"github.com/drunkleen/l-ui/internal/util"
	"github.com/drunkleen/l-ui/internal/util/common"

	"github.com/gofiber/fiber/v3"
)

// XraySettingController handles Xray configuration and settings operations.
type XraySettingController struct {
	XraySettingService service.XraySettingService
	SettingService     service.SettingService
	InboundService     service.InboundService
	OutboundService    service.OutboundService
	XrayService        service.XrayService
	WarpService        service.WarpService
	NordService        service.NordService
}

// NewXraySettingController creates a new XraySettingController and initializes its routes.
func NewXraySettingController(router fiber.Router) *XraySettingController {
	a := &XraySettingController{}
	a.initRouter(router)
	return a
}

// initRouter sets up the routes for Xray settings management.
func (a *XraySettingController) initRouter(router fiber.Router) {
	router = router.Group("/xray")
	router.Get("/getDefaultJsonConfig", a.getDefaultXrayConfig)
	router.Get("/getOutboundsTraffic", a.getOutboundsTraffic)
	router.Get("/getXrayResult", a.getXrayResult)

	router.Post("/", a.getXraySetting)
	router.Post("/warp/:action", a.warp)
	router.Post("/nord/:action", a.nord)
	router.Post("/update", a.updateSetting)
	router.Post("/resetOutboundsTraffic", a.resetOutboundsTraffic)
	router.Post("/testOutbound", a.testOutbound)
}

// getXraySetting retrieves the Xray configuration template, inbound tags, and outbound test URL.
func (a *XraySettingController) getXraySetting(c fiber.Ctx) error {
	xraySetting, err := a.SettingService.GetXrayConfigTemplate()
	if err != nil {
		jsonMsg(c, I18nWeb(c, "pages.settings.toasts.getSettings"), err)
		return nil
	}
	// Older versions of this handler embedded the raw DB value as
	// `xraySetting` in the response without checking if the value
	// already had that wrapper shape. When the frontend saved it
	// back through the textarea verbatim, the wrapper got persisted
	// and every subsequent save nested another layer, which is what
	// eventually produced the blank Xray Settings page in #4059.
	// Strip any such wrapper here, and heal the DB if we found one so
	// the next read is O(1) instead of climbing the same pile again.
	if unwrapped := service.UnwrapXrayTemplateConfig(xraySetting); unwrapped != xraySetting {
		if saveErr := a.XraySettingService.SaveXraySetting(unwrapped); saveErr == nil {
			xraySetting = unwrapped
		} else {
			// Don't fail the read — just serve the unwrapped value
			// and leave the DB healing for a later save.
			xraySetting = unwrapped
		}
	}
	inboundTags, err := a.InboundService.GetInboundTags()
	if err != nil {
		jsonMsg(c, I18nWeb(c, "pages.settings.toasts.getSettings"), err)
		return nil
	}
	clientReverseTags, err := a.InboundService.GetClientReverseTags()
	if err != nil {
		clientReverseTags = "[]"
	}
	outboundTestUrl, _ := a.SettingService.GetXrayOutboundTestUrl()
	if outboundTestUrl == "" {
		outboundTestUrl = "https://www.google.com/generate_204"
	}
	xrayResponse := map[string]any{
		"xraySetting":       json.RawMessage(xraySetting),
		"inboundTags":       json.RawMessage(inboundTags),
		"clientReverseTags": json.RawMessage(clientReverseTags),
		"outboundTestUrl":   outboundTestUrl,
	}
	result, err := json.Marshal(xrayResponse)
	if err != nil {
		jsonMsg(c, I18nWeb(c, "pages.settings.toasts.getSettings"), err)
		return nil
	}
	jsonObj(c, string(result), nil)
	return nil
}

// updateSetting updates the Xray configuration settings.
func (a *XraySettingController) updateSetting(c fiber.Ctx) error {
	xraySetting := c.FormValue("xraySetting")
	if err := a.XraySettingService.SaveXraySetting(xraySetting); err != nil {
		jsonMsg(c, I18nWeb(c, "pages.settings.toasts.modifySettings"), err)
		return nil
	}
	outboundTestUrl := c.FormValue("outboundTestUrl")
	if outboundTestUrl == "" {
		outboundTestUrl = "https://www.google.com/generate_204"
	}
	if err := a.SettingService.SetXrayOutboundTestUrl(outboundTestUrl); err != nil {
		jsonMsg(c, I18nWeb(c, "pages.settings.toasts.modifySettings"), err)
		return nil
	}
	jsonMsg(c, I18nWeb(c, "pages.settings.toasts.modifySettings"), nil)
	return nil
}

// getDefaultXrayConfig retrieves the default Xray configuration.
func (a *XraySettingController) getDefaultXrayConfig(c fiber.Ctx) error {
	defaultJsonConfig, err := a.SettingService.GetDefaultXrayConfig()
	if err != nil {
		jsonMsg(c, I18nWeb(c, "pages.settings.toasts.getSettings"), err)
		return nil
	}
	jsonObj(c, defaultJsonConfig, nil)
	return nil
}

// getXrayResult retrieves the current Xray service result.
func (a *XraySettingController) getXrayResult(c fiber.Ctx) error {
	jsonObj(c, a.XrayService.GetXrayResult(), nil)
	return nil
}

// warp handles Warp-related operations based on the action parameter.
func (a *XraySettingController) warp(c fiber.Ctx) error {
	action := c.Params("action")
	var resp string
	var err error
	switch action {
	case "data":
		resp, err = a.WarpService.GetWarpData()
	case "del":
		err = a.WarpService.DelWarpData()
	case "config":
		resp, err = a.WarpService.GetWarpConfig()
	case "reg":
		skey := c.FormValue("privateKey")
		pkey := c.FormValue("publicKey")
		resp, err = a.WarpService.RegWarp(skey, pkey)
	case "license":
		license := c.FormValue("license")
		resp, err = a.WarpService.SetWarpLicense(license)
	}

	jsonObj(c, resp, err)
	return nil
}

// nord handles NordVPN-related operations based on the action parameter.
func (a *XraySettingController) nord(c fiber.Ctx) error {
	action := c.Params("action")
	var resp string
	var err error
	switch action {
	case "countries":
		resp, err = a.NordService.GetCountries()
	case "servers":
		countryId := c.FormValue("countryId")
		resp, err = a.NordService.GetServers(countryId)
	case "reg":
		token := c.FormValue("token")
		resp, err = a.NordService.GetCredentials(token)
	case "setKey":
		key := c.FormValue("key")
		resp, err = a.NordService.SetKey(key)
	case "data":
		resp, err = a.NordService.GetNordData()
	case "del":
		err = a.NordService.DelNordData()
	}

	jsonObj(c, resp, err)
	return nil
}

// getOutboundsTraffic retrieves the traffic statistics for outbounds.
func (a *XraySettingController) getOutboundsTraffic(c fiber.Ctx) error {
	outboundsTraffic, err := a.OutboundService.GetOutboundsTraffic()
	if err != nil {
		jsonMsg(c, I18nWeb(c, "pages.settings.toasts.getOutboundTrafficError"), err)
		return nil
	}
	jsonObj(c, outboundsTraffic, nil)
	return nil
}

// resetOutboundsTraffic resets the traffic statistics for the specified outbound tag.
func (a *XraySettingController) resetOutboundsTraffic(c fiber.Ctx) error {
	tag := c.FormValue("tag")
	err := a.OutboundService.ResetOutboundTraffic(tag)
	if err != nil {
		jsonMsg(c, I18nWeb(c, "pages.settings.toasts.resetOutboundTrafficError"), err)
		return nil
	}
	jsonObj(c, "", nil)
	return nil
}

// testOutbound tests an outbound configuration and returns the delay/response time.
// Optional form "allOutbounds": JSON array of all outbounds; used to resolve sockopt.dialerProxy dependencies.
// Optional form "mode": "tcp" for a fast dial-only probe (parallel-safe),
// anything else (default) for a full HTTP probe through a temp xray instance.
func (a *XraySettingController) testOutbound(c fiber.Ctx) error {
	outboundJSON := c.FormValue("outbound")
	allOutboundsJSON := c.FormValue("allOutbounds")
	mode := c.FormValue("mode")

	if outboundJSON == "" {
		jsonMsg(c, I18nWeb(c, "somethingWentWrong"), common.NewError("outbound parameter is required"))
		return nil
	}

	// Load the test URL from server settings to prevent SSRF via user-controlled URLs
	testURL, _ := a.SettingService.GetXrayOutboundTestUrl()
	testURL, err := util.SanitizePublicHTTPURL(testURL, false)
	if err != nil {
		jsonMsg(c, I18nWeb(c, "somethingWentWrong"), err)
		return nil
	}

	result, err := a.OutboundService.TestOutbound(outboundJSON, testURL, allOutboundsJSON, mode)
	if err != nil {
		jsonMsg(c, I18nWeb(c, "somethingWentWrong"), err)
		return nil
	}

	jsonObj(c, result, nil)
	return nil
}
