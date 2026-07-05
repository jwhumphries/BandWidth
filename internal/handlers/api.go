package handlers

import (
	"log/slog"
	"net/http"
	"strings"

	"github.com/labstack/echo/v5"

	"github.com/jwhumphries/bandwidth/internal/auth"
	"github.com/jwhumphries/bandwidth/internal/mail"
	"github.com/jwhumphries/bandwidth/internal/model"
	"github.com/jwhumphries/bandwidth/internal/repository"
)

// API holds the dependencies shared by all HTTP handlers.
type API struct {
	Repo          *repository.Repo
	Mailer        mail.Mailer
	Logger        *slog.Logger
	BaseURL       string
	SecureCookies bool
	AdminEmails   map[string]bool
}

// IsAdminEmail reports whether email belongs to a configured site admin.
func (a *API) IsAdminEmail(email string) bool {
	return a.AdminEmails[strings.ToLower(strings.TrimSpace(email))]
}

// logger returns the configured logger, or the process default.
func (a *API) logger() *slog.Logger {
	if a.Logger != nil {
		return a.Logger
	}
	return slog.Default()
}

func (a *API) userResponse(u *model.User) map[string]any {
	return map[string]any{
		"id":          u.ID,
		"username":    u.Username,
		"email":       u.Email,
		"totpEnabled": u.TOTPEnabled(),
		"isAdmin":     a.IsAdminEmail(u.Email),
	}
}

func (a *API) setSessionCookie(c *echo.Context, token string) {
	c.SetCookie(&http.Cookie{
		Name:     auth.SessionCookieName,
		Value:    token,
		Path:     "/",
		MaxAge:   int(auth.SessionDuration.Seconds()),
		HttpOnly: true,
		Secure:   a.SecureCookies,
		SameSite: http.SameSiteLaxMode,
	})
}

func (a *API) clearSessionCookie(c *echo.Context) {
	c.SetCookie(&http.Cookie{
		Name:     auth.SessionCookieName,
		Value:    "",
		Path:     "/",
		MaxAge:   -1,
		HttpOnly: true,
		Secure:   a.SecureCookies,
		SameSite: http.SameSiteLaxMode,
	})
}
