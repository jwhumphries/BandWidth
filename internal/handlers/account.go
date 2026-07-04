package handlers

import (
	"net/http"
	"strings"

	"github.com/labstack/echo/v5"

	"github.com/jwhumphries/bandwidth/internal/auth"
	appmw "github.com/jwhumphries/bandwidth/internal/middleware"
	"github.com/jwhumphries/bandwidth/internal/repository"
)

// Me returns the authenticated user.
func (a *API) Me(c *echo.Context) error {
	user := appmw.CurrentUser(c)
	if user == nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "user not in context")
	}
	return c.JSON(http.StatusOK, a.userResponse(user))
}

type updateMeRequest struct {
	Username *string `json:"username"`
	Email    *string `json:"email"`
}

// UpdateMe updates the authenticated user's username and/or email.
func (a *API) UpdateMe(c *echo.Context) error {
	user := appmw.CurrentUser(c)
	if user == nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "user not in context")
	}
	var req updateMeRequest
	if err := c.Bind(&req); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid request body")
	}
	if req.Username != nil {
		username := strings.TrimSpace(*req.Username)
		if !validUsername(username) {
			return echo.NewHTTPError(http.StatusBadRequest,
				"a username of at most 100 characters (without @) is required")
		}
		user.Username = username
	}
	if req.Email != nil {
		email := strings.ToLower(strings.TrimSpace(*req.Email))
		if !validEmail(email) {
			return echo.NewHTTPError(http.StatusBadRequest, "a valid email address is required")
		}
		user.Email = email
	}
	if err := a.Repo.SaveUser(user); err != nil {
		if repository.IsDuplicate(err) {
			return echo.NewHTTPError(http.StatusConflict, "username or email already taken")
		}
		return err
	}
	return c.JSON(http.StatusOK, a.userResponse(user))
}

type changePasswordRequest struct {
	CurrentPassword string `json:"currentPassword"`
	NewPassword     string `json:"newPassword"`
}

// ChangePassword verifies the current password, sets the new one, and
// rotates every session (a fresh one is issued for this browser).
func (a *API) ChangePassword(c *echo.Context) error {
	user := appmw.CurrentUser(c)
	if user == nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "user not in context")
	}
	var req changePasswordRequest
	if err := c.Bind(&req); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid request body")
	}
	if !auth.VerifyPassword(req.CurrentPassword, user.PasswordHash) {
		return echo.NewHTTPError(http.StatusUnauthorized, "current password is incorrect")
	}
	if len(req.NewPassword) < 8 {
		return echo.NewHTTPError(http.StatusBadRequest,
			"new password must be at least 8 characters")
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
	token, err := a.Repo.CreateSession(user.ID)
	if err != nil {
		return err
	}
	a.setSessionCookie(c, token)
	return c.JSON(http.StatusOK, a.userResponse(user))
}
