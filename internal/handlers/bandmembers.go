package handlers

import (
	"net/http"
	"strconv"

	"github.com/labstack/echo/v5"

	appmw "github.com/jwhumphries/bandwidth/internal/middleware"
	"github.com/jwhumphries/bandwidth/internal/model"
)

func memberID(c *echo.Context) (uint, error) {
	id, err := strconv.ParseUint(c.Param("userId"), 10, 32)
	if err != nil || id == 0 {
		return 0, echo.NewHTTPError(http.StatusNotFound, "member not found")
	}
	return uint(id), nil
}

type setRoleRequest struct {
	Role string `json:"role"`
}

// SetMemberRole changes a member's role (admin). The creator is immutable.
func (a *API) SetMemberRole(c *echo.Context) error {
	bandID, _, err := a.bandAccess(c, model.RoleAdmin)
	if err != nil {
		return err
	}
	targetID, err := memberID(c)
	if err != nil {
		return err
	}
	var req setRoleRequest
	if err := c.Bind(&req); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid request body")
	}
	role := model.BandRole(req.Role)
	if !role.Valid() {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid role")
	}
	band, err := a.Repo.BandByID(bandID)
	if err != nil {
		return notFoundOr(err, "band")
	}
	if band.CreatorID == targetID {
		return echo.NewHTTPError(http.StatusBadRequest, "the band creator is always an admin")
	}
	if err := a.Repo.SetMemberRole(bandID, targetID, role); err != nil {
		return notFoundOr(err, "member")
	}
	return c.NoContent(http.StatusNoContent)
}

// RemoveMember removes a member (admin), or lets a member remove
// themselves (leave). The creator can do neither — they delete the band.
func (a *API) RemoveMember(c *echo.Context) error {
	bandID, role, err := a.bandAccess(c, model.RoleViewer)
	if err != nil {
		return err
	}
	user := appmw.CurrentUser(c)
	targetID, err := memberID(c)
	if err != nil {
		return err
	}
	if targetID != user.ID && !role.AtLeast(model.RoleAdmin) {
		return echo.NewHTTPError(http.StatusForbidden, "insufficient role")
	}
	band, err := a.Repo.BandByID(bandID)
	if err != nil {
		return notFoundOr(err, "band")
	}
	if band.CreatorID == targetID {
		return echo.NewHTTPError(http.StatusBadRequest,
			"the creator cannot leave or be removed; delete the band instead")
	}
	if err := a.Repo.RemoveMember(bandID, targetID); err != nil {
		return notFoundOr(err, "member")
	}
	return c.NoContent(http.StatusNoContent)
}
