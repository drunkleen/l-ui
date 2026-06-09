package session

import (
	"encoding/gob"
	"net/http"
	"time"

	"github.com/drunkleen/l-ui/internal/database"
	"github.com/drunkleen/l-ui/internal/database/model"
	"github.com/drunkleen/l-ui/internal/logger"

	"github.com/gofiber/fiber/v3"
	"github.com/gofiber/fiber/v3/extractors"
	"github.com/gofiber/fiber/v3/middleware/session"
)

const (
	loginUserKey      = "LOGIN_USER"
	loginEpochKey     = "LOGIN_EPOCH"
	apiAuthUserKey    = "api_auth_user"
	sessionCookieName = "l-ui"
)

var defaultStore *session.Store

func init() {
	gob.Register(model.User{})
}

// SetupStore initializes the session store and returns the middleware handler.
func SetupStore(secret []byte, path string, secure bool, maxAgeSec int) fiber.Handler {
	cfg := session.Config{
		CookieHTTPOnly: true,
		CookieSameSite: "Lax",
		CookieSecure:   secure,
		CookiePath:     path,
	}
	if maxAgeSec > 0 {
		cfg.IdleTimeout = time.Duration(maxAgeSec) * time.Second
	}
	cfg.Extractor = extractors.FromCookie(sessionCookieName)
	handler, store := session.NewWithStore(cfg)
	defaultStore = store
	return handler
}

func sessionForContext(c fiber.Ctx) *session.Session {
	m := session.FromContext(c)
	if m != nil {
		return m.Session
	}
	if defaultStore != nil {
		s, err := defaultStore.Get(c)
		if err == nil {
			return s
		}
	}
	return nil
}

func SetLoginUser(c fiber.Ctx, user *model.User) error {
	if user == nil {
		return nil
	}
	s := sessionForContext(c)
	if s == nil {
		return nil
	}
	s.Set(loginUserKey, user.Id)
	s.Set(loginEpochKey, user.LoginEpoch)
	return nil
}

func SetAPIAuthUser(c fiber.Ctx, user *model.User) {
	if user == nil {
		return
	}
	c.Locals(apiAuthUserKey, user)
}

func GetLoginUser(c fiber.Ctx) *model.User {
	if v := c.Locals(apiAuthUserKey); v != nil {
		if u, ok2 := v.(*model.User); ok2 {
			return u
		}
	}
	s := sessionForContext(c)
	if s == nil {
		return nil
	}
	obj := s.Get(loginUserKey)
	if obj == nil {
		return nil
	}
	userID, ok := sessionUserID(obj)
	if !ok {
		s.Delete(loginUserKey)
		s.Delete(loginEpochKey)
		return nil
	}
	if legacyUserID, ok := legacySessionUserID(obj); ok {
		s.Set(loginUserKey, legacyUserID)
	}
	user, err := getUserByID(userID)
	if err != nil {
		logger.Warning("session: failed to load user:", err)
		s.Delete(loginUserKey)
		s.Delete(loginEpochKey)
		return nil
	}
	if user.LoginEpoch == 0 {
		return user
	}
	if !sessionEpochMatches(s.Get(loginEpochKey), user.LoginEpoch) {
		s.Delete(loginUserKey)
		s.Delete(loginEpochKey)
		return nil
	}
	return user
}

func sessionEpochMatches(cookieVal any, userEpoch int64) bool {
	var got int64
	switch v := cookieVal.(type) {
	case nil:
	case int64:
		got = v
	case int:
		got = int64(v)
	case int32:
		got = int64(v)
	case float64:
		got = int64(v)
	default:
		return false
	}
	return got == userEpoch
}

func IsLogin(c fiber.Ctx) bool {
	return GetLoginUser(c) != nil
}

func sessionUserID(obj any) (int, bool) {
	switch v := obj.(type) {
	case int:
		return v, v > 0
	case int64:
		return int(v), v > 0
	case int32:
		return int(v), v > 0
	case float64:
		id := int(v)
		return id, v == float64(id) && id > 0
	case model.User:
		return v.Id, v.Id > 0
	case *model.User:
		if v == nil {
			return 0, false
		}
		return v.Id, v.Id > 0
	default:
		return 0, false
	}
}

func legacySessionUserID(obj any) (int, bool) {
	switch v := obj.(type) {
	case model.User:
		return v.Id, v.Id > 0
	case *model.User:
		if v == nil {
			return 0, false
		}
		return v.Id, v.Id > 0
	default:
		return 0, false
	}
}

func getUserByID(id int) (*model.User, error) {
	db := database.GetDB()
	if db == nil {
		return nil, http.ErrServerClosed
	}
	user := &model.User{}
	if err := db.Model(model.User{}).Where("id = ?", id).First(user).Error; err != nil {
		return nil, err
	}
	return user, nil
}

func ClearSession(c fiber.Ctx) error {
	m := session.FromContext(c)
	if m != nil {
		return m.Destroy()
	}
	return nil
}
