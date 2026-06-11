package handlers

import (
	"net/http"

	"github.com/labstack/echo/v5"

	appmw "github.com/jwhumphries/bandwidth/internal/middleware"
)

// Me returns the authenticated user.
func (a *API) Me(c *echo.Context) error {
	user := appmw.CurrentUser(c)
	if user == nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "user not in context")
	}
	return c.JSON(http.StatusOK, userResponse(user))
}

// UpdateMe is implemented in the account task.
func (a *API) UpdateMe(_ *echo.Context) error {
	return echo.NewHTTPError(http.StatusNotImplemented, "not implemented")
}

// ChangePassword is implemented in the account task.
func (a *API) ChangePassword(_ *echo.Context) error {
	return echo.NewHTTPError(http.StatusNotImplemented, "not implemented")
}
