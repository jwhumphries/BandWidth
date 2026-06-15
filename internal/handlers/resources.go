package handlers

import (
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"github.com/labstack/echo/v5"

	appmw "github.com/jwhumphries/bandwidth/internal/middleware"
	"github.com/jwhumphries/bandwidth/internal/model"
)

// validResourceURL accepts absolute http(s) URLs only.
func validResourceURL(raw string) bool {
	u, err := url.Parse(raw)
	return err == nil && (u.Scheme == "http" || u.Scheme == "https") && u.Host != ""
}

func resourceID(c *echo.Context) (uint, error) {
	id, err := strconv.ParseUint(c.Param("resourceId"), 10, 32)
	if err != nil || id == 0 {
		return 0, echo.NewHTTPError(http.StatusNotFound, "resource not found")
	}
	return uint(id), nil
}

func resourceResponse(r *model.Resource) map[string]any {
	return map[string]any{"id": r.ID, "url": r.URL, "label": r.Label}
}

type resourceRequest struct {
	URL   *string `json:"url"`
	Label *string `json:"label"`
}

// CreateResource appends a link to the user's resource list for a song.
func (a *API) CreateResource(c *echo.Context) error {
	user := appmw.CurrentUser(c)
	if user == nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "user not in context")
	}
	id, err := songID(c)
	if err != nil {
		return err
	}
	if _, err := a.Repo.SongVisibleToUser(id, user.ID); err != nil {
		return notFoundOr(err, "song")
	}
	var req resourceRequest
	if err := c.Bind(&req); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid request body")
	}
	if req.URL == nil || !validResourceURL(*req.URL) {
		return echo.NewHTTPError(http.StatusBadRequest, "a valid http(s) url is required")
	}
	label := ""
	if req.Label != nil {
		label = strings.TrimSpace(*req.Label)
		if len(label) > maxTitleLen {
			return echo.NewHTTPError(http.StatusBadRequest, "label too long")
		}
	}
	res, err := a.Repo.CreateResource(id, user.ID, *req.URL, label)
	if err != nil {
		return err
	}
	return c.JSON(http.StatusCreated, resourceResponse(res))
}

// UpdateResource patches a resource's url and/or label.
func (a *API) UpdateResource(c *echo.Context) error {
	user := appmw.CurrentUser(c)
	if user == nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "user not in context")
	}
	sid, err := songID(c)
	if err != nil {
		return err
	}
	rid, err := resourceID(c)
	if err != nil {
		return err
	}
	var req resourceRequest
	if err := c.Bind(&req); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid request body")
	}
	if req.URL != nil && !validResourceURL(*req.URL) {
		return echo.NewHTTPError(http.StatusBadRequest, "a valid http(s) url is required")
	}
	if req.Label != nil && len(*req.Label) > maxTitleLen {
		return echo.NewHTTPError(http.StatusBadRequest, "label too long")
	}
	res, err := a.Repo.UpdateResource(rid, sid, user.ID, req.URL, req.Label)
	if err != nil {
		return notFoundOr(err, "resource")
	}
	return c.JSON(http.StatusOK, resourceResponse(res))
}

// DeleteResource removes a resource.
func (a *API) DeleteResource(c *echo.Context) error {
	user := appmw.CurrentUser(c)
	if user == nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "user not in context")
	}
	sid, err := songID(c)
	if err != nil {
		return err
	}
	rid, err := resourceID(c)
	if err != nil {
		return err
	}
	if err := a.Repo.DeleteResource(rid, sid, user.ID); err != nil {
		return notFoundOr(err, "resource")
	}
	return c.NoContent(http.StatusNoContent)
}
