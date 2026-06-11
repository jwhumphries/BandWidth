package handlers

import (
	"net/http"
	"strings"
	"time"

	"github.com/labstack/echo/v5"

	"github.com/jwhumphries/bandwidth/internal/auth"
	appmw "github.com/jwhumphries/bandwidth/internal/middleware"
)

// TwoFactorSetup generates a pending TOTP secret for the user.
func (a *API) TwoFactorSetup(c *echo.Context) error {
	user := appmw.CurrentUser(c)
	if user == nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "user not in context")
	}
	if user.TOTPEnabled() {
		return echo.NewHTTPError(http.StatusBadRequest,
			"two-factor authentication is already enabled")
	}
	key, err := auth.NewTOTPKey(user.Username)
	if err != nil {
		return err
	}
	user.TOTPSecret = key.Secret
	user.TOTPConfirmedAt = nil
	if err := a.Repo.SaveUser(user); err != nil {
		return err
	}
	return c.JSON(http.StatusOK, map[string]string{
		"secret":     key.Secret,
		"otpauthUrl": key.URL,
	})
}

type twoFactorCodeRequest struct {
	Code string `json:"code"`
}

// TwoFactorVerify confirms a pending enrollment and returns backup codes.
func (a *API) TwoFactorVerify(c *echo.Context) error {
	user := appmw.CurrentUser(c)
	if user == nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "user not in context")
	}
	if user.TOTPSecret == "" || user.TOTPEnabled() {
		return echo.NewHTTPError(http.StatusBadRequest, "no pending two-factor enrollment")
	}
	var req twoFactorCodeRequest
	if err := c.Bind(&req); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid request body")
	}
	if !auth.ValidateTOTP(strings.TrimSpace(req.Code), user.TOTPSecret) {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid two-factor code")
	}

	now := time.Now()
	user.TOTPConfirmedAt = &now
	if err := a.Repo.SaveUser(user); err != nil {
		return err
	}
	codes := auth.NewBackupCodes()
	if err := a.Repo.ReplaceBackupCodes(user.ID, codes); err != nil {
		return err
	}
	return c.JSON(http.StatusOK, map[string]any{"backupCodes": codes})
}

// TwoFactorDisable turns 2FA off after validating a TOTP or backup code.
func (a *API) TwoFactorDisable(c *echo.Context) error {
	user := appmw.CurrentUser(c)
	if user == nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "user not in context")
	}
	if !user.TOTPEnabled() {
		return echo.NewHTTPError(http.StatusBadRequest,
			"two-factor authentication is not enabled")
	}
	var req twoFactorCodeRequest
	if err := c.Bind(&req); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid request body")
	}
	code := strings.ToUpper(strings.TrimSpace(req.Code))
	if !auth.ValidateTOTP(code, user.TOTPSecret) &&
		!a.Repo.ConsumeBackupCode(user.ID, code) {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid two-factor code")
	}

	user.TOTPSecret = ""
	user.TOTPConfirmedAt = nil
	if err := a.Repo.SaveUser(user); err != nil {
		return err
	}
	if err := a.Repo.DeleteBackupCodes(user.ID); err != nil {
		return err
	}
	return c.JSON(http.StatusOK, userResponse(user))
}
