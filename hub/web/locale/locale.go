// Package locale provides internationalization (i18n) support for the l-ui web panel.
package locale

import (
	"embed"
	"encoding/json"
	"io/fs"
	"os"
	"strings"

	"github.com/drunkleen/l-ui/internal/logger"

	"github.com/gofiber/fiber/v3"
	"github.com/nicksnyder/go-i18n/v2/i18n"
	"golang.org/x/text/language"
)

var (
	i18nBundle   *i18n.Bundle
	LocalizerWeb *i18n.Localizer
	LocalizerBot *i18n.Localizer
)

// I18nType represents the type of interface for internationalization.
type I18nType string

const (
	Bot I18nType = "bot"
	Web I18nType = "web"
)

// SettingService interface defines methods for accessing locale settings.
type SettingService interface {
	GetTgLang() (string, error)
}

// InitLocalizer initializes the internationalization system with embedded translation files.
func InitLocalizer(i18nFS embed.FS, settingService SettingService) error {
	i18nBundle = i18n.NewBundle(language.MustParse("en-US"))
	i18nBundle.RegisterUnmarshalFunc("json", json.Unmarshal)

	if err := parseTranslationFiles(i18nFS, i18nBundle); err != nil {
		return err
	}

	if err := initTGBotLocalizer(settingService); err != nil {
		return err
	}

	return nil
}

func createTemplateData(params []string, separator ...string) map[string]any {
	var sep string = "=="
	if len(separator) > 0 {
		sep = separator[0]
	}

	templateData := make(map[string]any)
	for _, param := range params {
		parts := strings.SplitN(param, sep, 2)
		templateData[parts[0]] = parts[1]
	}

	return templateData
}

// I18n retrieves a localized message for the given key and type.
func I18n(i18nType I18nType, key string, params ...string) string {
	var localizer *i18n.Localizer

	switch i18nType {
	case "bot":
		localizer = LocalizerBot
	case "web":
		localizer = LocalizerWeb
	default:
		logger.Errorf("Invalid type for I18n: %s", i18nType)
		return ""
	}

	templateData := createTemplateData(params)

	if localizer == nil {
		return key
	}

	msg, err := localizer.Localize(&i18n.LocalizeConfig{
		MessageID:    key,
		TemplateData: templateData,
	})
	if err != nil {
		logger.Errorf("Failed to localize message: %v", err)
		return ""
	}

	return msg
}

func initTGBotLocalizer(settingService SettingService) error {
	botLang, err := settingService.GetTgLang()
	if err != nil {
		return err
	}

	LocalizerBot = i18n.NewLocalizer(i18nBundle, botLang)
	return nil
}

// LocalizerMiddleware returns a middleware that sets up localization for web requests.
func LocalizerMiddleware() fiber.Handler {
	return func(c fiber.Ctx) error {
		if i18nBundle == nil {
			i18nBundle = i18n.NewBundle(language.MustParse("en-US"))
			i18nBundle.RegisterUnmarshalFunc("json", json.Unmarshal)
			if err := loadTranslationsFromDisk(i18nBundle); err != nil {
				logger.Warning("i18n lazy load failed:", err)
			}
		}

		var lang string

		if cookie := c.Cookies("lang"); cookie != "" {
			lang = cookie
		} else {
			lang = c.Get("Accept-Language")
		}

		LocalizerWeb = i18n.NewLocalizer(i18nBundle, lang)

		c.Locals("localizer", LocalizerWeb)
		c.Locals("I18n", I18n)
		return c.Next()
	}
}

func loadTranslationsFromDisk(bundle *i18n.Bundle) error {
	root := os.DirFS("web")
	return fs.WalkDir(root, "translation", func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		data, err := fs.ReadFile(root, path)
		if err != nil {
			return err
		}
		_, err = bundle.ParseMessageFileBytes(data, path)
		return err
	})
}

func parseTranslationFiles(i18nFS embed.FS, i18nBundle *i18n.Bundle) error {
	err := fs.WalkDir(i18nFS, "translation",
		func(path string, d fs.DirEntry, err error) error {
			if err != nil {
				return err
			}

			if d.IsDir() {
				return nil
			}

			data, err := i18nFS.ReadFile(path)
			if err != nil {
				return err
			}

			_, err = i18nBundle.ParseMessageFileBytes(data, path)
			return err
		})
	if err != nil {
		return err
	}

	return nil
}
