package controller

import (
	"errors"
	"strconv"
	"time"

	"github.com/drunkleen/l-ui/internal/util/crypto"
	"github.com/drunkleen/l-ui/hub/web/entity"
	"github.com/drunkleen/l-ui/hub/web/middleware"
	"github.com/drunkleen/l-ui/hub/web/service"
	"github.com/drunkleen/l-ui/hub/web/session"

	"github.com/gofiber/fiber/v3"
)

// updateUserForm represents the form for updating user credentials.
type updateUserForm struct {
	OldUsername string `json:"oldUsername" form:"oldUsername"`
	OldPassword string `json:"oldPassword" form:"oldPassword"`
	NewUsername string `json:"newUsername" form:"newUsername"`
	NewPassword string `json:"newPassword" form:"newPassword"`
}

// SettingController handles settings and user management operations.
type SettingController struct {
	settingService  service.SettingService
	userService     service.UserService
	panelService    service.PanelService
	apiTokenService service.ApiTokenService
}

// NewSettingController creates a new SettingController and initializes its routes.
func NewSettingController(router fiber.Router) *SettingController {
	a := &SettingController{}
	a.initRouter(router)
	return a
}

// initRouter sets up the routes for settings management.
func (a *SettingController) initRouter(router fiber.Router) {
	router = router.Group("/setting")

	router.Post("/all", a.getAllSetting)
	router.Get("/users", a.listUsers)
	router.Post("/defaultSettings", a.getDefaultSettings)
	router.Post("/update", a.updateSetting)
	router.Post("/updateUser", a.updateUser)
	router.Post("/restartPanel", a.restartPanel)
	router.Get("/getDefaultJsonConfig", a.getDefaultXrayConfig)
	router.Get("/apiTokens", a.listApiTokens)
	router.Post("/apiTokens/create", a.createApiToken)
	router.Post("/apiTokens/delete/:id", a.deleteApiToken)
	router.Post("/apiTokens/setEnabled/:id", a.setApiTokenEnabled)
}

// getAllSetting retrieves all current settings.
func (a *SettingController) getAllSetting(c fiber.Ctx) error {
	allSetting, err := a.settingService.GetAllSetting()
	if err != nil {
		jsonMsg(c, I18nWeb(c, "pages.settings.toasts.getSettings"), err)
		return nil
	}
	jsonObj(c, allSetting, nil)
	return nil
}

// getDefaultSettings retrieves the default settings based on the host.
func (a *SettingController) getDefaultSettings(c fiber.Ctx) error {
	result, err := a.settingService.GetDefaultSettings(c.Hostname())
	if err != nil {
		jsonMsg(c, I18nWeb(c, "pages.settings.toasts.getSettings"), err)
		return nil
	}
	jsonObj(c, result, nil)
	return nil
}

func (a *SettingController) listUsers(c fiber.Ctx) error {
	users, err := a.userService.ListUsers()
	if err != nil {
		jsonMsg(c, I18nWeb(c, "pages.settings.toasts.getSettings"), err)
		return nil
	}
	jsonObj(c, users, nil)
	return nil
}

// updateSetting updates all settings with the provided data.
func (a *SettingController) updateSetting(c fiber.Ctx) error {
	allSetting, ok := middleware.BindAndValidate[entity.AllSetting](c)
	if !ok {
		return nil
	}
	oldTwoFactor, twoFactorErr := a.settingService.GetTwoFactorEnable()
	err := a.settingService.UpdateAllSetting(allSetting)
	if err == nil && twoFactorErr == nil && !oldTwoFactor && allSetting.TwoFactorEnable {
		if bumpErr := a.userService.BumpLoginEpoch(); bumpErr != nil {
			err = bumpErr
		}
	}
	jsonMsg(c, I18nWeb(c, "pages.settings.toasts.modifySettings"), err)
	return nil
}

// updateUser updates the current user's username and password.
func (a *SettingController) updateUser(c fiber.Ctx) error {
	form := &updateUserForm{}
	err := c.Bind().Body(form)
	if err != nil {
		jsonMsg(c, I18nWeb(c, "pages.settings.toasts.modifySettings"), err)
		return nil
	}
	user := session.GetLoginUser(c)
	if user.Username != form.OldUsername || !crypto.CheckPasswordHash(user.Password, form.OldPassword) {
		jsonMsg(c, I18nWeb(c, "pages.settings.toasts.modifyUserError"), errors.New(I18nWeb(c, "pages.settings.toasts.originalUserPassIncorrect")))
		return nil
	}
	if form.NewUsername == "" || form.NewPassword == "" {
		jsonMsg(c, I18nWeb(c, "pages.settings.toasts.modifyUserError"), errors.New(I18nWeb(c, "pages.settings.toasts.userPassMustBeNotEmpty")))
		return nil
	}
	err = a.userService.UpdateUser(user.Id, form.NewUsername, form.NewPassword)
	if err == nil {
		user.Username = form.NewUsername
		user.Password, _ = crypto.HashPasswordAsBcrypt(form.NewPassword)
		if saveErr := session.SetLoginUser(c, user); saveErr != nil {
			err = saveErr
		}
	}
	jsonMsg(c, I18nWeb(c, "pages.settings.toasts.modifyUser"), err)
	return nil
}

// restartPanel restarts the panel service after a delay.
func (a *SettingController) restartPanel(c fiber.Ctx) error {
	err := a.panelService.RestartPanel(time.Second * 3)
	jsonMsg(c, I18nWeb(c, "pages.settings.restartPanelSuccess"), err)
	return nil
}

// getDefaultXrayConfig retrieves the default Xray configuration.
func (a *SettingController) getDefaultXrayConfig(c fiber.Ctx) error {
	defaultJsonConfig, err := a.settingService.GetDefaultXrayConfig()
	if err != nil {
		jsonMsg(c, I18nWeb(c, "pages.settings.toasts.getSettings"), err)
		return nil
	}
	jsonObj(c, defaultJsonConfig, nil)
	return nil
}

type apiTokenCreateForm struct {
	Name string `json:"name" form:"name"`
}

type apiTokenEnabledForm struct {
	Enabled bool `json:"enabled" form:"enabled"`
}

func (a *SettingController) listApiTokens(c fiber.Ctx) error {
	rows, err := a.apiTokenService.List()
	if err != nil {
		jsonMsg(c, I18nWeb(c, "pages.settings.toasts.getSettings"), err)
		return nil
	}
	jsonObj(c, rows, nil)
	return nil
}

func (a *SettingController) createApiToken(c fiber.Ctx) error {
	form := &apiTokenCreateForm{}
	if err := c.Bind().Body(form); err != nil {
		jsonMsg(c, I18nWeb(c, "pages.settings.toasts.modifySettings"), err)
		return nil
	}
	row, err := a.apiTokenService.Create(form.Name)
	if err != nil {
		jsonMsg(c, I18nWeb(c, "pages.settings.toasts.modifySettings"), err)
		return nil
	}
	jsonObj(c, row, nil)
	return nil
}

func (a *SettingController) deleteApiToken(c fiber.Ctx) error {
	id, err := strconv.Atoi(c.Params("id"))
	if err != nil {
		jsonMsg(c, I18nWeb(c, "pages.settings.toasts.modifySettings"), err)
		return nil
	}
	jsonMsg(c, I18nWeb(c, "pages.settings.toasts.modifySettings"), a.apiTokenService.Delete(id))
	return nil
}

func (a *SettingController) setApiTokenEnabled(c fiber.Ctx) error {
	id, err := strconv.Atoi(c.Params("id"))
	if err != nil {
		jsonMsg(c, I18nWeb(c, "pages.settings.toasts.modifySettings"), err)
		return nil
	}
	form := &apiTokenEnabledForm{}
	if bindErr := c.Bind().Body(form); bindErr != nil {
		jsonMsg(c, I18nWeb(c, "pages.settings.toasts.modifySettings"), bindErr)
		return nil
	}
	jsonMsg(c, I18nWeb(c, "pages.settings.toasts.modifySettings"), a.apiTokenService.SetEnabled(id, form.Enabled))
	return nil
}
