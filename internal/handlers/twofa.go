package handlers

import (
	"net/http"

	"github.com/labstack/echo/v5"
)

// TwoFactorSetup is implemented in the 2FA task.
func (a *API) TwoFactorSetup(_ *echo.Context) error {
	return echo.NewHTTPError(http.StatusNotImplemented, "not implemented")
}

// TwoFactorVerify is implemented in the 2FA task.
func (a *API) TwoFactorVerify(_ *echo.Context) error {
	return echo.NewHTTPError(http.StatusNotImplemented, "not implemented")
}

// TwoFactorDisable is implemented in the 2FA task.
func (a *API) TwoFactorDisable(_ *echo.Context) error {
	return echo.NewHTTPError(http.StatusNotImplemented, "not implemented")
}
