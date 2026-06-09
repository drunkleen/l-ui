package controller

import (
	"errors"
	"strconv"

	"github.com/drunkleen/l-ui/internal/database/model"
	"github.com/drunkleen/l-ui/internal/logger"
	"github.com/drunkleen/l-ui/hub/web/entity"
	"github.com/drunkleen/l-ui/hub/web/service"

	"github.com/gofiber/fiber/v3"
)

type CustomGeoController struct {
	BaseController
	customGeoService *service.CustomGeoService
}

func NewCustomGeoController(router fiber.Router, customGeo *service.CustomGeoService) *CustomGeoController {
	a := &CustomGeoController{customGeoService: customGeo}
	a.initRouter(router)
	return a
}

func (a *CustomGeoController) initRouter(router fiber.Router) {
	router.Get("/list", a.list)
	router.Get("/aliases", a.aliases)
	router.Post("/add", a.add)
	router.Post("/update/:id", a.update)
	router.Post("/delete/:id", a.delete)
	router.Post("/download/:id", a.download)
	router.Post("/update-all", a.updateAll)
}

func mapCustomGeoErr(c fiber.Ctx, err error) error {
	if err == nil {
		return nil
	}
	switch {
	case errors.Is(err, service.ErrCustomGeoInvalidType):
		return errors.New(I18nWeb(c, "pages.index.customGeoErrInvalidType"))
	case errors.Is(err, service.ErrCustomGeoAliasRequired):
		return errors.New(I18nWeb(c, "pages.index.customGeoErrAliasRequired"))
	case errors.Is(err, service.ErrCustomGeoAliasPattern):
		return errors.New(I18nWeb(c, "pages.index.customGeoErrAliasPattern"))
	case errors.Is(err, service.ErrCustomGeoAliasReserved):
		return errors.New(I18nWeb(c, "pages.index.customGeoErrAliasReserved"))
	case errors.Is(err, service.ErrCustomGeoURLRequired):
		return errors.New(I18nWeb(c, "pages.index.customGeoErrUrlRequired"))
	case errors.Is(err, service.ErrCustomGeoInvalidURL):
		return errors.New(I18nWeb(c, "pages.index.customGeoErrInvalidUrl"))
	case errors.Is(err, service.ErrCustomGeoURLScheme):
		return errors.New(I18nWeb(c, "pages.index.customGeoErrUrlScheme"))
	case errors.Is(err, service.ErrCustomGeoURLHost):
		return errors.New(I18nWeb(c, "pages.index.customGeoErrUrlHost"))
	case errors.Is(err, service.ErrCustomGeoDuplicateAlias):
		return errors.New(I18nWeb(c, "pages.index.customGeoErrDuplicateAlias"))
	case errors.Is(err, service.ErrCustomGeoNotFound):
		return errors.New(I18nWeb(c, "pages.index.customGeoErrNotFound"))
	case errors.Is(err, service.ErrCustomGeoDownload):
		logger.Warning("custom geo download:", err)
		return errors.New(I18nWeb(c, "pages.index.customGeoErrDownload"))
	case errors.Is(err, service.ErrCustomGeoSSRFBlocked):
		logger.Warning("custom geo SSRF blocked:", err)
		return errors.New(I18nWeb(c, "pages.index.customGeoErrUrlHost"))
	case errors.Is(err, service.ErrCustomGeoPathTraversal):
		logger.Warning("custom geo path traversal blocked:", err)
		return errors.New(I18nWeb(c, "pages.index.customGeoErrDownload"))
	default:
		return err
	}
}

func (a *CustomGeoController) list(c fiber.Ctx) error {
	list, err := a.customGeoService.GetAll()
	if err != nil {
		jsonMsg(c, I18nWeb(c, "pages.index.customGeoToastList"), mapCustomGeoErr(c, err))
		return nil
	}
	jsonObj(c, list, nil)
	return nil
}

func (a *CustomGeoController) aliases(c fiber.Ctx) error {
	out, err := a.customGeoService.GetAliasesForUI()
	if err != nil {
		jsonMsg(c, I18nWeb(c, "pages.index.customGeoAliasesError"), mapCustomGeoErr(c, err))
		return nil
	}
	jsonObj(c, out, nil)
	return nil
}

type customGeoForm struct {
	Type  string `json:"type" form:"type"`
	Alias string `json:"alias" form:"alias"`
	Url   string `json:"url" form:"url"`
}

func (a *CustomGeoController) add(c fiber.Ctx) error {
	var form customGeoForm
	if err := c.Bind().JSON(&form); err != nil {
		jsonMsg(c, I18nWeb(c, "pages.index.customGeoToastAdd"), err)
		return nil
	}
	r := &model.CustomGeoResource{
		Type:  form.Type,
		Alias: form.Alias,
		Url:   form.Url,
	}
	err := a.customGeoService.Create(r)
	jsonMsg(c, I18nWeb(c, "pages.index.customGeoToastAdd"), mapCustomGeoErr(c, err))
	return nil
}

func parseCustomGeoID(c fiber.Ctx, idStr string) (int, bool) {
	id, err := strconv.Atoi(idStr)
	if err != nil {
		jsonMsg(c, I18nWeb(c, "pages.index.customGeoInvalidId"), err)
		return 0, false
	}
	if id <= 0 {
		jsonMsg(c, I18nWeb(c, "pages.index.customGeoInvalidId"), errors.New(""))
		return 0, false
	}
	return id, true
}

func (a *CustomGeoController) update(c fiber.Ctx) error {
	id, ok := parseCustomGeoID(c, c.Params("id"))
	if !ok {
		return nil
	}
	var form customGeoForm
	if bindErr := c.Bind().JSON(&form); bindErr != nil {
		jsonMsg(c, I18nWeb(c, "pages.index.customGeoToastUpdate"), bindErr)
		return nil
	}
	r := &model.CustomGeoResource{
		Type:  form.Type,
		Alias: form.Alias,
		Url:   form.Url,
	}
	err := a.customGeoService.Update(id, r)
	jsonMsg(c, I18nWeb(c, "pages.index.customGeoToastUpdate"), mapCustomGeoErr(c, err))
	return nil
}

func (a *CustomGeoController) delete(c fiber.Ctx) error {
	id, ok := parseCustomGeoID(c, c.Params("id"))
	if !ok {
		return nil
	}
	name, err := a.customGeoService.Delete(id)
	jsonMsg(c, I18nWeb(c, "pages.index.customGeoToastDelete", "fileName=="+name), mapCustomGeoErr(c, err))
	return nil
}

func (a *CustomGeoController) download(c fiber.Ctx) error {
	id, ok := parseCustomGeoID(c, c.Params("id"))
	if !ok {
		return nil
	}
	name, err := a.customGeoService.TriggerUpdate(id)
	jsonMsg(c, I18nWeb(c, "pages.index.customGeoToastDownload", "fileName=="+name), mapCustomGeoErr(c, err))
	return nil
}

func (a *CustomGeoController) updateAll(c fiber.Ctx) error {
	res, err := a.customGeoService.TriggerUpdateAll()
	if err != nil {
		jsonMsg(c, I18nWeb(c, "pages.index.customGeoToastUpdateAll"), mapCustomGeoErr(c, err))
		return nil
	}
	if len(res.Failed) > 0 {
		c.Status(fiber.StatusOK).JSON(entity.Msg{
			Success: false,
			Msg:     I18nWeb(c, "pages.index.customGeoErrUpdateAllIncomplete"),
			Obj:     res,
		})
		return nil
	}
	jsonMsgObj(c, I18nWeb(c, "pages.index.customGeoToastUpdateAll"), res, nil)
	return nil
}
