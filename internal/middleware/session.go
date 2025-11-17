package middleware

import (
	"github.com/adamscao/videoshare/internal/config"
	"github.com/gorilla/sessions"
)

var (
	AdminSessionStore *sessions.CookieStore
	VideoSessionStore *sessions.CookieStore
)

const (
	AdminSessionName = "admin-session"
	VideoSessionName = "video-session"
)

// InitSessions initializes session stores
func InitSessions(cfg *config.Config) {
	secret := []byte(cfg.Session.Secret)
	AdminSessionStore = sessions.NewCookieStore(secret)
	AdminSessionStore.Options = &sessions.Options{
		Path:     "/",
		MaxAge:   cfg.Session.MaxAge,
		HttpOnly: true,
		SameSite: sessions.SameSiteLaxMode,
	}

	VideoSessionStore = sessions.NewCookieStore(secret)
	VideoSessionStore.Options = &sessions.Options{
		Path:     "/",
		MaxAge:   cfg.Session.MaxAge,
		HttpOnly: true,
		SameSite: sessions.SameSiteLaxMode,
	}
}
