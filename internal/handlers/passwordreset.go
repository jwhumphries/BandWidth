package handlers

import (
	"fmt"
	"net/http"
	"strings"

	"github.com/labstack/echo/v5"

	"github.com/jwhumphries/bandwidth/internal/auth"
)

// RequestPasswordReset emails a reset link. Always 204 for enabled mailers
// (no account enumeration); 404 when mail is not configured.
func (a *API) RequestPasswordReset(c *echo.Context) error {
	if !a.Mailer.Enabled() {
		return echo.NewHTTPError(http.StatusNotFound, "not found")
	}
	var req struct {
		Email string `json:"email"`
	}
	if err := c.Bind(&req); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid request body")
	}

	email := strings.ToLower(strings.TrimSpace(req.Email))
	user, err := a.Repo.UserByLogin(email)
	if err != nil {
		// Burn a comparable token-hash computation so an unknown email
		// doesn't return faster than a known one. This doesn't hide the
		// SMTP round-trip that only happens for real accounts — closing
		// that gap would mean sending a real email on every request.
		auth.HashToken(auth.NewToken())
		return c.NoContent(http.StatusNoContent)
	}
	if token, err := a.Repo.CreatePasswordReset(user.ID); err == nil {
		link := fmt.Sprintf("%s/reset-password?token=%s", a.BaseURL, token)
		if err := a.Mailer.Send(user.Email, "Reset your BandWidth password",
			"Someone (hopefully you) asked to reset your BandWidth password.\n\n"+
				"Reset it within the next hour: "+link+"\n\n"+
				"If this wasn't you, ignore this email."); err != nil {
			// Response stays 204 (no account enumeration), but operators
			// need to know the relay is broken.
			a.logger().Warn("password reset email failed", "error", err)
		}
	}
	return c.NoContent(http.StatusNoContent)
}

// ConfirmPasswordReset sets a new password from a valid reset token and
// revokes all existing sessions.
func (a *API) ConfirmPasswordReset(c *echo.Context) error {
	if !a.Mailer.Enabled() {
		return echo.NewHTTPError(http.StatusNotFound, "not found")
	}
	var req struct {
		Token       string `json:"token"`
		NewPassword string `json:"newPassword"`
	}
	if err := c.Bind(&req); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid request body")
	}
	if len(req.NewPassword) < 8 {
		return echo.NewHTTPError(http.StatusBadRequest,
			"new password must be at least 8 characters")
	}

	// The token is consumed before the password write; if hashing or saving
	// fails the token is burned and the user re-requests a link. Accepted
	// trade-off to keep single-use enforcement atomic.
	userID, err := a.Repo.ConsumePasswordReset(req.Token)
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid or expired reset token")
	}
	user, err := a.Repo.UserByID(userID)
	if err != nil {
		return err
	}
	hash, err := auth.HashPassword(req.NewPassword)
	if err != nil {
		return err
	}
	user.PasswordHash = hash
	if err := a.Repo.SaveUser(user); err != nil {
		return err
	}
	if err := a.Repo.DeleteUserSessions(user.ID); err != nil {
		return err
	}
	return c.NoContent(http.StatusNoContent)
}
