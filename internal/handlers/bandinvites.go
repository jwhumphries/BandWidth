package handlers

import (
	"errors"
	"net/http"
	"strconv"
	"strings"

	"github.com/labstack/echo/v5"

	appmw "github.com/jwhumphries/bandwidth/internal/middleware"
	"github.com/jwhumphries/bandwidth/internal/model"
	"github.com/jwhumphries/bandwidth/internal/repository"
)

func inviteID(c *echo.Context) (uint, error) {
	param := c.Param("inviteId")
	if param == "" {
		param = c.Param("id")
	}
	id, err := strconv.ParseUint(param, 10, 32)
	if err != nil || id == 0 {
		return 0, echo.NewHTTPError(http.StatusNotFound, "invite not found")
	}
	return uint(id), nil
}

type createInviteRequest struct {
	Username string `json:"username"`
	Role     string `json:"role"`
	Link     bool   `json:"link"`
}

// CreateInvite creates a direct invite (by username/email) or a share link
// (admin). The raw link token is returned exactly once.
func (a *API) CreateInvite(c *echo.Context) error {
	bandID, _, err := a.bandAccess(c, model.RoleAdmin)
	if err != nil {
		return err
	}
	user := appmw.CurrentUser(c)
	var req createInviteRequest
	if err := c.Bind(&req); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid request body")
	}

	role := model.RoleEditor
	if req.Role != "" {
		role = model.BandRole(req.Role)
		if !role.Valid() {
			return echo.NewHTTPError(http.StatusBadRequest, "invalid role")
		}
	}

	if req.Link {
		token, err := a.Repo.CreateLinkInvite(bandID, role, user.ID)
		if err != nil {
			return err
		}
		return c.JSON(http.StatusCreated, map[string]any{
			"role": role, "token": token, "isLink": true,
		})
	}

	username := strings.TrimSpace(req.Username)
	if username == "" {
		return echo.NewHTTPError(http.StatusBadRequest, "a username or email is required")
	}
	invitee, err := a.Repo.UserByLogin(username)
	if err != nil {
		return echo.NewHTTPError(http.StatusNotFound, "no such user")
	}
	invite, err := a.Repo.CreateDirectInvite(bandID, invitee.ID, role, user.ID)
	switch {
	case errors.Is(err, repository.ErrAlreadyMember):
		return echo.NewHTTPError(http.StatusConflict, "already a member")
	case errors.Is(err, repository.ErrInvitePending):
		return echo.NewHTTPError(http.StatusConflict, "an invite is already pending")
	case err != nil:
		return err
	}
	return c.JSON(http.StatusCreated, map[string]any{
		"id": invite.ID, "role": invite.Role, "invitedUsername": invitee.Username,
	})
}

// BandInvites lists a band's pending invites (admin).
func (a *API) BandInvites(c *echo.Context) error {
	bandID, _, err := a.bandAccess(c, model.RoleAdmin)
	if err != nil {
		return err
	}
	invites, err := a.Repo.InvitesForBand(bandID)
	if err != nil {
		return err
	}
	return c.JSON(http.StatusOK, invites)
}

// RevokeInvite revokes a pending invite (admin).
func (a *API) RevokeInvite(c *echo.Context) error {
	bandID, _, err := a.bandAccess(c, model.RoleAdmin)
	if err != nil {
		return err
	}
	id, err := inviteID(c)
	if err != nil {
		return err
	}
	if err := a.Repo.RevokeInvite(id, bandID); err != nil {
		return notFoundOr(err, "invite")
	}
	return c.NoContent(http.StatusNoContent)
}

// MyInvites lists the caller's pending invites.
func (a *API) MyInvites(c *echo.Context) error {
	user := appmw.CurrentUser(c)
	if user == nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "user not in context")
	}
	pending, err := a.Repo.PendingInvitesForUser(user.ID)
	if err != nil {
		return err
	}
	return c.JSON(http.StatusOK, pending)
}

// AcceptInvite accepts a direct invite and returns the joined band's id.
func (a *API) AcceptInvite(c *echo.Context) error {
	user := appmw.CurrentUser(c)
	if user == nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "user not in context")
	}
	id, err := inviteID(c)
	if err != nil {
		return err
	}
	bandID, err := a.Repo.AcceptInvite(id, user.ID)
	if err != nil {
		return notFoundOr(err, "invite")
	}
	return c.JSON(http.StatusOK, map[string]any{"bandId": bandID})
}

// DeclineInvite declines a direct invite.
func (a *API) DeclineInvite(c *echo.Context) error {
	user := appmw.CurrentUser(c)
	if user == nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "user not in context")
	}
	id, err := inviteID(c)
	if err != nil {
		return err
	}
	if err := a.Repo.DeclineInvite(id, user.ID); err != nil {
		return notFoundOr(err, "invite")
	}
	return c.NoContent(http.StatusNoContent)
}

// JoinByLink joins a band via a share-link token.
func (a *API) JoinByLink(c *echo.Context) error {
	user := appmw.CurrentUser(c)
	if user == nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "user not in context")
	}
	token := c.Param("token")
	if token == "" {
		return echo.NewHTTPError(http.StatusNotFound, "invite not found")
	}
	bandID, err := a.Repo.JoinByLink(token, user.ID)
	if err != nil {
		return notFoundOr(err, "invite")
	}
	return c.JSON(http.StatusOK, map[string]any{"bandId": bandID})
}
