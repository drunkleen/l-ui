package sub

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"strconv"
	"strings"

	"github.com/drunkleen/l-ui/hub/web/service"

	"github.com/gofiber/fiber/v3"
)

// writeSubError translates a service-layer result into an HTTP response.
// A nil error with no rows means the subId doesn't match anything (deleted
// client, never-existed id) and becomes 404. A real error becomes 500. No
// body — VPN clients only look at the status.
func writeSubError(c fiber.Ctx, err error) error {
	if err == nil {
		return c.SendStatus(fiber.StatusNotFound)
	}
	return c.SendStatus(fiber.StatusInternalServerError)
}

// SUBController handles HTTP requests for subscription links and JSON configurations.
type SUBController struct {
	subTitle         string
	subSupportUrl    string
	subProfileUrl    string
	subAnnounce      string
	subEnableRouting bool
	subRoutingRules  string
	subPath          string
	subJsonPath      string
	subClashPath     string
	jsonEnabled      bool
	clashEnabled     bool
	subEncrypt       bool
	updateInterval   string

	subService      *SubService
	subJsonService  *SubJsonService
	subClashService *SubClashService
	settingService  service.SettingService
}

// NewSUBController creates a new subscription controller with the given configuration.
func NewSUBController(
	g fiber.Router,
	subPath string,
	jsonPath string,
	clashPath string,
	jsonEnabled bool,
	clashEnabled bool,
	encrypt bool,
	showInfo bool,
	rModel string,
	update string,
	jsonFragment string,
	jsonNoise string,
	jsonMux string,
	jsonRules string,
	subTitle string,
	subSupportUrl string,
	subProfileUrl string,
	subAnnounce string,
	subEnableRouting bool,
	subRoutingRules string,
) *SUBController {
	sub := NewSubService(showInfo, rModel)
	a := &SUBController{
		subTitle:         subTitle,
		subSupportUrl:    subSupportUrl,
		subProfileUrl:    subProfileUrl,
		subAnnounce:      subAnnounce,
		subEnableRouting: subEnableRouting,
		subRoutingRules:  subRoutingRules,
		subPath:          subPath,
		subJsonPath:      jsonPath,
		subClashPath:     clashPath,
		jsonEnabled:      jsonEnabled,
		clashEnabled:     clashEnabled,
		subEncrypt:       encrypt,
		updateInterval:   update,

		subService:      sub,
		subJsonService:  NewSubJsonService(jsonFragment, jsonNoise, jsonMux, jsonRules, sub),
		subClashService: NewSubClashService(sub),
	}
	a.initRouter(g)
	return a
}

// initRouter registers HTTP routes for subscription links and JSON endpoints
// on the provided router.
func (a *SUBController) initRouter(g fiber.Router) {
	gLink := g.Group(a.subPath)
	gLink.Get(":subid", a.subs)
	gLink.Head(":subid", a.subs)
	if a.jsonEnabled {
		gJson := g.Group(a.subJsonPath)
		gJson.Get(":subid", a.subJsons)
		gJson.Head(":subid", a.subJsons)
	}
	if a.clashEnabled {
		gClash := g.Group(a.subClashPath)
		gClash.Get(":subid", a.subClashs)
		gClash.Head(":subid", a.subClashs)
	}
}

// subs handles HTTP requests for subscription links, returning either HTML page or base64-encoded subscription data.
func (a *SUBController) subs(c fiber.Ctx) error {
	subId := c.Params("subid")
	scheme, host, hostWithPort, hostHeader := a.subService.ResolveRequest(c)
	subs, emails, lastOnline, traffic, err := a.subService.GetSubs(subId, host)
	if err != nil || len(subs) == 0 {
		return writeSubError(c, err)
	}

	result := ""
	for _, sub := range subs {
		result += sub + "\n"
	}

	// If the request expects HTML (e.g., browser) or explicitly asked (?html=1 or ?view=html), render the info page here
	accept := c.Get("Accept")
	if strings.Contains(strings.ToLower(accept), "text/html") || c.Query("html") == "1" || strings.EqualFold(c.Query("view"), "html") {
		subURL, subJsonURL, subClashURL := a.subService.BuildURLs(a.subPath, a.subJsonPath, a.subClashPath, subId)
		if !a.jsonEnabled {
			subJsonURL = ""
		}
		if !a.clashEnabled {
			subClashURL = ""
		}
		basePathVal := c.Locals("base_path")
		basePath, ok := basePathVal.(string)
		if !ok {
			basePath = "/"
		}
		page := a.subService.BuildPageData(subId, hostHeader, traffic, lastOnline, subs, emails, subURL, subJsonURL, subClashURL, basePath, a.subTitle, a.subSupportUrl)
		return a.serveSubPage(c, basePath, page)
	}

	// Add headers
	header := fmt.Sprintf("upload=%d; download=%d; total=%d; expire=%d", traffic.Up, traffic.Down, traffic.Total, traffic.ExpiryTime/1000)
	profileUrl := a.subProfileUrl
	if profileUrl == "" {
		profileUrl = fmt.Sprintf("%s://%s%s", scheme, hostWithPort, string(c.Request().URI().RequestURI()))
	}
	a.ApplyCommonHeaders(c, header, a.updateInterval, a.subTitle, a.subSupportUrl, profileUrl, a.subAnnounce, a.subEnableRouting, a.subRoutingRules)

	if a.subEncrypt {
		return c.Status(fiber.StatusOK).SendString(base64.StdEncoding.EncodeToString([]byte(result)))
	}
	return c.Status(fiber.StatusOK).SendString(result)
}

// serveSubPage renders web/dist/subpage.html for the current subscription
// request. The Vite-built SPA reads window.__SUB_PAGE_DATA__ on mount —
// we inject that here, along with window.L_UI_BASE_PATH so the
// page's static asset references resolve correctly when the panel runs
// behind a URL prefix.
func (a *SUBController) serveSubPage(c fiber.Ctx, basePath string, page PageData) error {
	var body []byte
	if diskBody, diskErr := os.ReadFile("hub/web/dist/subpage.html"); diskErr == nil {
		body = diskBody
	} else {
		readBody, err := distFS.ReadFile("dist/subpage.html")
		if err != nil {
			return c.Status(fiber.StatusInternalServerError).SendString("missing embedded subpage")
		}
		body = readBody
	}

	// Vite emits absolute asset URLs (`/assets/...`); when the panel is
	// installed under a custom URL prefix, rewrite them so the bundle
	// loads from `<basePath>assets/...` where the static handler is
	// actually mounted.
	if basePath != "/" && basePath != "" {
		body = bytes.ReplaceAll(body, []byte(`src="/assets/`), []byte(`src="`+basePath+`assets/`))
		body = bytes.ReplaceAll(body, []byte(`href="/assets/`), []byte(`href="`+basePath+`assets/`))
	}

	// JSON-marshal the view-model so the SPA can read it as a plain
	// The panel's "Calendar Type" setting decides whether the SubPage
	// renders dates in Gregorian or Jalali — surface it here so the SPA
	// can match the rest of the panel without a round-trip.
	datepicker, _ := a.settingService.GetDatepicker()
	if datepicker == "" {
		datepicker = "gregorian"
	}

	subData := map[string]any{
		"sId":          page.SId,
		"enabled":      page.Enabled,
		"download":     page.Download,
		"upload":       page.Upload,
		"total":        page.Total,
		"used":         page.Used,
		"remained":     page.Remained,
		"expire":       page.Expire,
		"lastOnline":   page.LastOnline,
		"downloadByte": page.DownloadByte,
		"uploadByte":   page.UploadByte,
		"totalByte":    page.TotalByte,
		"subUrl":       page.SubUrl,
		"subJsonUrl":   page.SubJsonUrl,
		"subClashUrl":  page.SubClashUrl,
		"links":        page.Result,
		"emails":       page.Emails,
		"datepicker":   datepicker,
	}
	subDataJSON, err := json.Marshal(subData)
	if err != nil {
		subDataJSON = []byte("{}")
	}

	// Defense-in-depth string-escape for the basePath embed — admin-
	// controlled but cheap to harden.
	jsEscape := strings.NewReplacer(
		`\`, `\\`,
		`"`, `\"`,
		"\n", `\n`,
		"\r", `\r`,
		"<", `<`,
		">", `>`,
		"&", `&`,
	)
	escapedBase := jsEscape.Replace(basePath)

	inject := []byte(`<script>window.L_UI_BASE_PATH="` + escapedBase + `";` +
		`window.__SUB_PAGE_DATA__=` + string(subDataJSON) + `;</script></head>`)
	out := bytes.Replace(body, []byte("</head>"), inject, 1)

	c.Set("Cache-Control", "no-cache, no-store, must-revalidate")
	c.Set("Pragma", "no-cache")
	c.Set("Expires", "0")
	c.Set("Content-Type", "text/html; charset=utf-8")
	return c.Status(fiber.StatusOK).Send(out)
}

// subJsons handles HTTP requests for JSON subscription configurations.
func (a *SUBController) subJsons(c fiber.Ctx) error {
	subId := c.Params("subid")
	scheme, host, hostWithPort, _ := a.subService.ResolveRequest(c)
	jsonSub, header, err := a.subJsonService.GetJson(subId, host)
	if err != nil || len(jsonSub) == 0 {
		return writeSubError(c, err)
	}

	profileUrl := a.subProfileUrl
	if profileUrl == "" {
		profileUrl = fmt.Sprintf("%s://%s%s", scheme, hostWithPort, string(c.Request().URI().RequestURI()))
	}
	a.ApplyCommonHeaders(c, header, a.updateInterval, a.subTitle, a.subSupportUrl, profileUrl, a.subAnnounce, a.subEnableRouting, a.subRoutingRules)

	return c.Status(fiber.StatusOK).SendString(jsonSub)
}

// subClashs handles HTTP requests for Clash subscription configurations.
func (a *SUBController) subClashs(c fiber.Ctx) error {
	subId := c.Params("subid")
	scheme, host, hostWithPort, _ := a.subService.ResolveRequest(c)
	clashSub, header, err := a.subClashService.GetClash(subId, host)
	if err != nil || len(clashSub) == 0 {
		return writeSubError(c, err)
	}

	profileUrl := a.subProfileUrl
	if profileUrl == "" {
		profileUrl = fmt.Sprintf("%s://%s%s", scheme, hostWithPort, string(c.Request().URI().RequestURI()))
	}
	a.ApplyCommonHeaders(c, header, a.updateInterval, a.subTitle, a.subSupportUrl, profileUrl, a.subAnnounce, a.subEnableRouting, a.subRoutingRules)
	if a.subTitle != "" {
		// Clash clients commonly use Content-Disposition to choose the imported profile name.
		c.Response().Header.Set("Content-Disposition", fmt.Sprintf(`attachment; filename*=UTF-8''%s`, url.PathEscape(a.subTitle)))
	}
	c.Set("Content-Type", "application/yaml; charset=utf-8")
	return c.Status(fiber.StatusOK).Send([]byte(clashSub))
}

// ApplyCommonHeaders sets common HTTP headers for subscription responses including user info, update interval, and profile title.
func (a *SUBController) ApplyCommonHeaders(
	c fiber.Ctx,
	header,
	updateInterval,
	profileTitle string,
	profileSupportUrl string,
	profileUrl string,
	profileAnnounce string,
	profileEnableRouting bool,
	profileRoutingRules string,
) {
	c.Response().Header.Set("Subscription-Userinfo", header)
	c.Response().Header.Set("Profile-Update-Interval", updateInterval)

	//Basics
	if profileTitle != "" {
		c.Response().Header.Set("Profile-Title", "base64:"+base64.StdEncoding.EncodeToString([]byte(profileTitle)))
	}
	if profileSupportUrl != "" {
		c.Response().Header.Set("Support-Url", profileSupportUrl)
	}
	if profileUrl != "" {
		c.Response().Header.Set("Profile-Web-Page-Url", profileUrl)
	}
	if profileAnnounce != "" {
		c.Response().Header.Set("Announce", "base64:"+base64.StdEncoding.EncodeToString([]byte(profileAnnounce)))
	}

	//Advanced (Happ)
	c.Response().Header.Set("Routing-Enable", strconv.FormatBool(profileEnableRouting))
	if profileRoutingRules != "" {
		c.Response().Header.Set("Routing", profileRoutingRules)
	}
}
