package handlers

import (
	"net/http"
	netmail "net/mail"
	"strings"

	"github.com/labstack/echo/v5"

	"github.com/jwhumphries/bandwidth/internal/auth"
	"github.com/jwhumphries/bandwidth/internal/repository"
)

// validEmail reports whether s is a plain RFC 5322 address (no display name).
func validEmail(s string) bool {
	addr, err := netmail.ParseAddress(s)
	return err == nil && addr.Address == s
}

type signupRequest struct {
	Username string `json:"username"`
	Email    string `json:"email"`
	Password string `json:"password"`
}

// Signup creates an account and logs the new user in.
func (a *API) Signup(c *echo.Context) error {
	var req signupRequest
	if err := c.Bind(&req); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid request body")
	}
	req.Username = strings.TrimSpace(req.Username)
	req.Email = strings.ToLower(strings.TrimSpace(req.Email))
	if req.Username == "" || len(req.Password) < 8 {
		return echo.NewHTTPError(http.StatusBadRequest,
			"username and a password of at least 8 characters are required")
	}
	if !validEmail(req.Email) {
		return echo.NewHTTPError(http.StatusBadRequest, "a valid email address is required")
	}

	hash, err := auth.HashPassword(req.Password)
	if err != nil {
		return err
	}
	user, err := a.Repo.CreateUser(req.Username, req.Email, hash)
	if err != nil {
		if repository.IsDuplicate(err) {
			return echo.NewHTTPError(http.StatusConflict, "username or email already taken")
		}
		return err
	}

	token, err := a.Repo.CreateSession(user.ID)
	if err != nil {
		return err
	}
	a.setSessionCookie(c, token)
	return c.JSON(http.StatusCreated, userResponse(user))
}

type loginRequest struct {
	Login    string `json:"login"` // username or email
	Password string `json:"password"`
	TOTPCode string `json:"totpCode"`
}

// Login authenticates by password, then by TOTP or backup code when enrolled.
func (a *API) Login(c *echo.Context) error {
	var req loginRequest
	if err := c.Bind(&req); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid request body")
	}

	user, err := a.Repo.UserByLogin(strings.TrimSpace(req.Login))
	if err != nil || !auth.VerifyPassword(req.Password, user.PasswordHash) {
		return echo.NewHTTPError(http.StatusUnauthorized, "invalid credentials")
	}

	if user.TOTPEnabled() {
		if req.TOTPCode == "" {
			return c.JSON(http.StatusUnauthorized, map[string]any{
				"message":      "two-factor code required",
				"totpRequired": true,
			})
		}
		code := strings.ToUpper(strings.TrimSpace(req.TOTPCode))
		if !auth.ValidateTOTP(code, user.TOTPSecret) &&
			!a.Repo.ConsumeBackupCode(user.ID, code) {
			return echo.NewHTTPError(http.StatusUnauthorized, "invalid two-factor code")
		}
	}

	token, err := a.Repo.CreateSession(user.ID)
	if err != nil {
		return err
	}
	a.setSessionCookie(c, token)
	return c.JSON(http.StatusOK, userResponse(user))
}

// Logout deletes the session and clears the cookie.
func (a *API) Logout(c *echo.Context) error {
	if cookie, err := c.Request().Cookie(auth.SessionCookieName); err == nil {
		_ = a.Repo.DeleteSession(cookie.Value)
	}
	a.clearSessionCookie(c)
	return c.NoContent(http.StatusNoContent)
}

// Features reports which optional features are available to the frontend.
func (a *API) Features(c *echo.Context) error {
	return c.JSON(http.StatusOK, map[string]bool{
		"passwordReset": a.Mailer.Enabled(),
	})
}
