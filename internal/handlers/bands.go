package handlers

import (
	"errors"
	"net/http"
	"strings"

	"github.com/labstack/echo/v5"
	"gorm.io/gorm"

	appmw "github.com/jwhumphries/bandwidth/internal/middleware"
	"github.com/jwhumphries/bandwidth/internal/model"
)

// bandAccess parses the :id band, loads the caller's role, and enforces a
// minimum. Non-members get 404 (bands are invisible to outsiders);
// insufficient role gets 403.
func (a *API) bandAccess(c *echo.Context, minRole model.BandRole) (uint, model.BandRole, error) {
	user := appmw.CurrentUser(c)
	if user == nil {
		return 0, "", echo.NewHTTPError(http.StatusInternalServerError, "user not in context")
	}
	id, err := songID(c) // shared uint :id parser
	if err != nil {
		return 0, "", echo.NewHTTPError(http.StatusNotFound, "band not found")
	}
	role, err := a.Repo.MemberRole(id, user.ID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return 0, "", echo.NewHTTPError(http.StatusNotFound, "band not found")
		}
		return 0, "", err
	}
	if !role.AtLeast(minRole) {
		return 0, "", echo.NewHTTPError(http.StatusForbidden, "insufficient role")
	}
	return id, role, nil
}

// Bands lists the caller's bands.
func (a *API) Bands(c *echo.Context) error {
	user := appmw.CurrentUser(c)
	if user == nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "user not in context")
	}
	summaries, err := a.Repo.BandsForUser(user.ID)
	if err != nil {
		return err
	}
	return c.JSON(http.StatusOK, summaries)
}

type bandNameRequest struct {
	Name string `json:"name"`
}

// CreateBand creates a band with the caller as permanent Admin.
func (a *API) CreateBand(c *echo.Context) error {
	user := appmw.CurrentUser(c)
	if user == nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "user not in context")
	}
	var req bandNameRequest
	if err := c.Bind(&req); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid request body")
	}
	req.Name = strings.TrimSpace(req.Name)
	if req.Name == "" || len(req.Name) > maxTitleLen {
		return echo.NewHTTPError(http.StatusBadRequest, "a band name is required")
	}
	band, err := a.Repo.CreateBand(user.ID, req.Name)
	if err != nil {
		return err
	}
	return c.JSON(http.StatusCreated, map[string]any{
		"id": band.ID, "name": band.Name, "creatorId": band.CreatorID,
	})
}

// Band returns band detail with the roster (any member).
func (a *API) Band(c *echo.Context) error {
	id, role, err := a.bandAccess(c, model.RoleViewer)
	if err != nil {
		return err
	}
	band, err := a.Repo.BandByID(id)
	if err != nil {
		return notFoundOr(err, "band")
	}
	members, err := a.Repo.MembersForBand(id)
	if err != nil {
		return err
	}
	return c.JSON(http.StatusOK, map[string]any{
		"id":        band.ID,
		"name":      band.Name,
		"creatorId": band.CreatorID,
		"myRole":    role,
		"members":   members,
	})
}

// RenameBand renames the band (admin).
func (a *API) RenameBand(c *echo.Context) error {
	id, _, err := a.bandAccess(c, model.RoleAdmin)
	if err != nil {
		return err
	}
	var req bandNameRequest
	if err := c.Bind(&req); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid request body")
	}
	req.Name = strings.TrimSpace(req.Name)
	if req.Name == "" || len(req.Name) > maxTitleLen {
		return echo.NewHTTPError(http.StatusBadRequest, "a band name is required")
	}
	if err := a.Repo.RenameBand(id, req.Name); err != nil {
		return err
	}
	return c.NoContent(http.StatusNoContent)
}

// DeleteBand deletes the band (creator only).
func (a *API) DeleteBand(c *echo.Context) error {
	id, _, err := a.bandAccess(c, model.RoleAdmin)
	if err != nil {
		return err
	}
	user := appmw.CurrentUser(c)
	if user == nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "user not in context")
	}
	band, err := a.Repo.BandByID(id)
	if err != nil {
		return notFoundOr(err, "band")
	}
	if band.CreatorID != user.ID {
		return echo.NewHTTPError(http.StatusForbidden, "only the band creator can delete the band")
	}
	if err := a.Repo.DeleteBand(id); err != nil {
		return err
	}
	return c.NoContent(http.StatusNoContent)
}
