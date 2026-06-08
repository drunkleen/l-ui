package session

import (
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"

	"github.com/drunkleen/l-ui/internal/database"
	"github.com/drunkleen/l-ui/internal/database/model"

	"github.com/gin-contrib/sessions"
	"github.com/gin-contrib/sessions/cookie"
	"github.com/gin-gonic/gin"
)

func TestSetLoginUserStoresOnlyUserID(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(sessions.Sessions(sessionCookieName, cookie.NewStore([]byte("01234567890123456789012345678901"))))
	router.GET("/", func(c *gin.Context) {
		if err := SetLoginUser(c, &model.User{Id: 7, Username: "admin", Password: "hash"}); err != nil {
			t.Fatal(err)
		}
		got := sessions.Default(c).Get(loginUserKey)
		if got != 7 {
			t.Fatalf("stored session payload = %#v, want user id only", got)
		}
		c.Status(http.StatusNoContent)
	})

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusNoContent)
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
	gin.SetMode(gin.TestMode)
	dir := t.TempDir()
	if err := database.InitDB(filepath.Join(dir, "l-ui.db")); err != nil {
		t.Fatalf("InitDB: %v", err)
	}
	defer func() { _ = database.CloseDB() }()
	if err := database.GetDB().Create(&model.User{Id: 7, Username: "admin", Password: "hash", LoginEpoch: 0}).Error; err != nil {
		t.Fatalf("create user: %v", err)
	}
	router := gin.New()
	router.Use(sessions.Sessions(sessionCookieName, cookie.NewStore([]byte("01234567890123456789012345678901"))))
	router.GET("/", func(c *gin.Context) {
		s := sessions.Default(c)
		s.Set(loginUserKey, 7)
		s.Set(loginEpochKey, 0)
		if err := s.Save(); err != nil {
			t.Fatal(err)
		}
		c.Status(http.StatusNoContent)
	})
	router.GET("/check", func(c *gin.Context) {
		if got := GetLoginUser(c); got == nil || got.Id != 7 {
			t.Fatalf("GetLoginUser returned %#v, want user id 7", got)
		}
		c.Status(http.StatusNoContent)
	})

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusNoContent {
		t.Fatalf("seed status = %d, want %d", rec.Code, http.StatusNoContent)
	}
	cookies := rec.Result().Cookies()

	rec = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodGet, "/check", nil)
	for _, cookie := range cookies {
		req.AddCookie(cookie)
	}
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusNoContent {
		t.Fatalf("check status = %d, want %d", rec.Code, http.StatusNoContent)
	}
}
