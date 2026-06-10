// Package handlers contains the HTTP handlers for the BandWidth server.
package handlers

import (
	"net/http"

	"github.com/labstack/echo/v5"
)

// Healthz reports server liveness.
func Healthz(c *echo.Context) error {
	return c.JSON(http.StatusOK, map[string]string{"status": "ok"})
}
