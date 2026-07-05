package handlers

import (
	"net/http"
	"strconv"
	"strings"

	"github.com/labstack/echo/v5"

	appmw "github.com/jwhumphries/bandwidth/internal/middleware"
	"github.com/jwhumphries/bandwidth/internal/repository"
)

// adminUserID parses the :id path parameter for user-targeted admin routes.
func adminUserID(c *echo.Context) (uint, error) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil || id == 0 {
		return 0, echo.NewHTTPError(http.StatusNotFound, "user not found")
	}
	return uint(id), nil
}

// adminBandID parses the :id path parameter for band-targeted admin routes.
func adminBandID(c *echo.Context) (uint, error) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil || id == 0 {
		return 0, echo.NewHTTPError(http.StatusNotFound, "band not found")
	}
	return uint(id), nil
}

// adminAllowedEmailID parses the :id path parameter for allow-list admin routes.
func adminAllowedEmailID(c *echo.Context) (uint, error) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil || id == 0 {
		return 0, echo.NewHTTPError(http.StatusNotFound, "allow-list entry not found")
	}
	return uint(id), nil
}

// AdminUsers lists every account.
func (a *API) AdminUsers(c *echo.Context) error {
	users, err := a.Repo.AllUsers()
	if err != nil {
		return err
	}
	return c.JSON(http.StatusOK, users)
}

// AdminDeleteUser deletes a user and everything they solely own. Admins
// cannot delete their own account (avoids self-lockout).
func (a *API) AdminDeleteUser(c *echo.Context) error {
	admin := appmw.CurrentUser(c)
	if admin == nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "user not in context")
	}
	id, err := adminUserID(c)
	if err != nil {
		return err
	}
	if id == admin.ID {
		return echo.NewHTTPError(http.StatusBadRequest, "cannot delete your own account")
	}
	if _, err := a.Repo.UserByID(id); err != nil {
		return notFoundOr(err, "user")
	}
	if err := a.Repo.DeleteUser(id); err != nil {
		return err
	}
	return c.NoContent(http.StatusNoContent)
}

// AdminBands lists every band.
func (a *API) AdminBands(c *echo.Context) error {
	bands, err := a.Repo.AllBands()
	if err != nil {
		return err
	}
	return c.JSON(http.StatusOK, bands)
}

// AdminDeleteBand deletes any band.
func (a *API) AdminDeleteBand(c *echo.Context) error {
	id, err := adminBandID(c)
	if err != nil {
		return err
	}
	if _, err := a.Repo.BandByID(id); err != nil {
		return notFoundOr(err, "band")
	}
	if err := a.Repo.DeleteBand(id); err != nil {
		return err
	}
	return c.NoContent(http.StatusNoContent)
}

// AdminGetAccessPolicy returns the signup gate state and its allow-list.
func (a *API) AdminGetAccessPolicy(c *echo.Context) error {
	enabled, err := a.Repo.AccessPolicyEnabled()
	if err != nil {
		return err
	}
	emails, err := a.Repo.AllowedEmails()
	if err != nil {
		return err
	}
	return c.JSON(http.StatusOK, map[string]any{
		"enabled":       enabled,
		"allowedEmails": emails,
	})
}

type setAccessPolicyRequest struct {
	Enabled bool `json:"enabled"`
}

// AdminSetAccessPolicy toggles signup gating.
func (a *API) AdminSetAccessPolicy(c *echo.Context) error {
	var req setAccessPolicyRequest
	if err := c.Bind(&req); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid request body")
	}
	if err := a.Repo.SetAccessPolicyEnabled(req.Enabled); err != nil {
		return err
	}
	return c.NoContent(http.StatusNoContent)
}

type addAllowedEmailRequest struct {
	Email string `json:"email"`
}

// AdminAddAllowedEmail adds an email to the signup allow-list.
func (a *API) AdminAddAllowedEmail(c *echo.Context) error {
	admin := appmw.CurrentUser(c)
	if admin == nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "user not in context")
	}
	var req addAllowedEmailRequest
	if err := c.Bind(&req); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid request body")
	}
	email := strings.ToLower(strings.TrimSpace(req.Email))
	if !validEmail(email) {
		return echo.NewHTTPError(http.StatusBadRequest, "a valid email address is required")
	}
	entry, err := a.Repo.AddAllowedEmail(email, admin.ID)
	if err != nil {
		if repository.IsDuplicate(err) {
			return echo.NewHTTPError(http.StatusConflict, "email already on the allow-list")
		}
		return err
	}
	return c.JSON(http.StatusCreated, map[string]any{
		"id":        entry.ID,
		"email":     entry.Email,
		"createdAt": entry.CreatedAt,
	})
}

// AdminRemoveAllowedEmail removes an allow-list entry.
func (a *API) AdminRemoveAllowedEmail(c *echo.Context) error {
	id, err := adminAllowedEmailID(c)
	if err != nil {
		return err
	}
	if err := a.Repo.RemoveAllowedEmail(id); err != nil {
		return notFoundOr(err, "allow-list entry")
	}
	return c.NoContent(http.StatusNoContent)
}
