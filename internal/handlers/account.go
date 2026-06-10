package handlers

import (
	"net/http"

	"github.com/labstack/echo/v5"

	appmw "github.com/jwhumphries/bandwidth/internal/middleware"
)

// Me returns the authenticated user.
func (a *API) Me(c *echo.Context) error {
	return c.JSON(http.StatusOK, userResponse(appmw.CurrentUser(c)))
}
