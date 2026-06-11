package handlers

import (
	"net/http"

	"github.com/labstack/echo/v5"
)

// RequestPasswordReset is implemented in the password reset task.
func (a *API) RequestPasswordReset(_ *echo.Context) error {
	return echo.NewHTTPError(http.StatusNotImplemented, "not implemented")
}

// ConfirmPasswordReset is implemented in the password reset task.
func (a *API) ConfirmPasswordReset(_ *echo.Context) error {
	return echo.NewHTTPError(http.StatusNotImplemented, "not implemented")
}
