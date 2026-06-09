package session

import (
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"

	"github.com/drunkleen/l-ui/internal/database"
	"github.com/drunkleen/l-ui/internal/database/model"

	"github.com/gofiber/fiber/v3"
	"github.com/gofiber/fiber/v3/extractors"
	fiberSession "github.com/gofiber/fiber/v3/middleware/session"
)

func TestSetLoginUserStoresOnlyUserID(t *testing.T) {
	app := fiber.New()

	cfg := fiberSession.Config{
		CookieHTTPOnly: true,
		CookieSameSite: "Lax",
		CookieSecure:   false,
		CookiePath:     "/",
	}
	cfg.Extractor = extractors.FromCookie(sessionCookieName)
	handler, _ := fiberSession.NewWithStore(cfg)
	app.Use(handler)

	app.Get("/", func(c fiber.Ctx) error {
		if err := SetLoginUser(c, &model.User{Id: 7, Username: "admin", Password: "hash"}); err != nil {
			t.Fatal(err)
		}
		m := fiberSession.FromContext(c)
		if m == nil {
			t.Fatal("no session in context")
		}
		got := m.Get(loginUserKey)
		if got != 7 {
			t.Fatalf("stored session payload = %#v, want user id only", got)
		}
		return c.SendStatus(http.StatusNoContent)
	})

	resp, err := app.Test(httptest.NewRequest(http.MethodGet, "/", nil))
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusNoContent {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusNoContent)
	}
}

func TestSessionUserIDSupportsLegacyUserPayload(t *testing.T) {
	id, ok := sessionUserID(model.User{Id: 11, Username: "admin", Password: "hash"})
	if !ok || id != 11 {
		t.Fatalf("legacy session payload resolved to (%d, %v), want (11, true)", id, ok)
	}
	id, ok = sessionUserID(&model.User{Id: 12, Username: "admin", Password: "hash"})
	if !ok || id != 12 {
		t.Fatalf("legacy pointer session payload resolved to (%d, %v), want (12, true)", id, ok)
	}
}

func TestGetLoginUserAllowsZeroLoginEpoch(t *testing.T) {
	dir := t.TempDir()
	if err := database.InitDB(filepath.Join(dir, "l-ui.db")); err != nil {
		t.Fatalf("InitDB: %v", err)
	}
	defer func() { _ = database.CloseDB() }()
	if err := database.GetDB().Create(&model.User{Id: 7, Username: "admin", Password: "hash", LoginEpoch: 0}).Error; err != nil {
		t.Fatalf("create user: %v", err)
	}

	app := fiber.New()

	cfg := fiberSession.Config{
		CookieHTTPOnly: true,
		CookieSameSite: "Lax",
		CookieSecure:   false,
		CookiePath:     "/",
	}
	cfg.Extractor = extractors.FromCookie(sessionCookieName)
	handler, _ := fiberSession.NewWithStore(cfg)
	app.Use(handler)

	app.Get("/", func(c fiber.Ctx) error {
		m := fiberSession.FromContext(c)
		if m == nil {
			t.Fatal("no session in context")
		}
		m.Set(loginUserKey, 7)
		m.Set(loginEpochKey, int64(0))
		return c.SendStatus(http.StatusNoContent)
	})
	app.Get("/check", func(c fiber.Ctx) error {
		if got := GetLoginUser(c); got == nil || got.Id != 7 {
			t.Fatalf("GetLoginUser returned %#v, want user id 7", got)
		}
		return c.SendStatus(http.StatusNoContent)
	})

	// Seed session
	resp, err := app.Test(httptest.NewRequest(http.MethodGet, "/", nil))
	if err != nil {
		t.Fatalf("seed request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusNoContent {
		t.Fatalf("seed status = %d, want %d", resp.StatusCode, http.StatusNoContent)
	}
	cookies := resp.Cookies()

	// Check session
	checkReq := httptest.NewRequest(http.MethodGet, "/check", nil)
	for _, cookie := range cookies {
		checkReq.AddCookie(cookie)
	}

	checkResp, err := app.Test(checkReq)
	if err != nil {
		t.Fatalf("check request failed: %v", err)
	}
	defer checkResp.Body.Close()

	if checkResp.StatusCode != http.StatusNoContent {
		t.Fatalf("check status = %d, want %d", checkResp.StatusCode, http.StatusNoContent)
	}
}
