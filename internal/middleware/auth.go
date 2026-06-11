// Package middleware holds application HTTP middleware.
package middleware

import (
	"net/http"

	"github.com/labstack/echo/v5"

	"github.com/jwhumphries/bandwidth/internal/auth"
	"github.com/jwhumphries/bandwidth/internal/model"
	"github.com/jwhumphries/bandwidth/internal/repository"
)

const userContextKey = "user"

// RequireAuth loads the session user from the cookie or rejects with 401.
func RequireAuth(repo *repository.Repo) echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c *echo.Context) error {
			cookie, err := c.Request().Cookie(auth.SessionCookieName)
			if err != nil || cookie.Value == "" {
				return echo.NewHTTPError(http.StatusUnauthorized, "authentication required")
			}
			user, err := repo.SessionUser(cookie.Value)
			if err != nil {
				return echo.NewHTTPError(http.StatusUnauthorized, "authentication required")
			}
			c.Set(userContextKey, user)
			return next(c)
		}
	}
}

// CurrentUser returns the authenticated user stored by RequireAuth, or nil
// when the route was not guarded by RequireAuth.
func CurrentUser(c *echo.Context) *model.User {
	user, _ := c.Get(userContextKey).(*model.User)
	return user
}
