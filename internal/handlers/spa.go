package handlers

import (
	"io/fs"
	"net/http"
	"strings"

	"github.com/labstack/echo/v5"
)

// RegisterSPA serves the frontend build from fsys on the catch-all route,
// falling back to index.html for paths that don't exist so client-side
// routing works on hard refresh and deep links.
func RegisterSPA(e *echo.Echo, fsys fs.FS) {
	fileServer := http.FileServerFS(fsys)
	e.GET("/*", func(c *echo.Context) error {
		req := c.Request()
		path := strings.TrimPrefix(req.URL.Path, "/")
		if path != "" {
			if _, err := fs.Stat(fsys, path); err != nil {
				req.URL.Path = "/" // fall back to index.html
			}
		}
		fileServer.ServeHTTP(c.Response(), req)
		return nil
	})
}
