package handlers

import (
	"net/http"
	"strings"
	"time"

	"github.com/labstack/echo/v5"

	"github.com/jwhumphries/bandwidth/internal/model"
)

func (a *API) bandRehearsalStatsResponse(c *echo.Context, songID, bandID uint) error {
	last, count, err := a.Repo.BandPracticeStats(songID, bandID)
	if err != nil {
		return err
	}
	return c.JSON(http.StatusOK, map[string]any{
		"lastRehearsedAt": last,
		"rehearsalCount":  count,
	})
}

type logRehearsalRequest struct {
	Date string `json:"date"`
}

// LogBandRehearsal records a band rehearsal day (Editor+). Default: today UTC.
func (a *API) LogBandRehearsal(c *echo.Context) error {
	bandID, _, err := a.bandAccess(c, model.RoleEditor)
	if err != nil {
		return err
	}
	song, err := a.bandSongForRequest(c, bandID)
	if err != nil {
		return err
	}
	var req logRehearsalRequest
	if err := c.Bind(&req); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid request body")
	}
	if req.Date == "" {
		req.Date = time.Now().UTC().Format("2006-01-02")
	}
	if !validPracticeDate(req.Date) {
		return echo.NewHTTPError(http.StatusBadRequest, "date must be YYYY-MM-DD and not in the future")
	}
	if err := a.Repo.LogBandPractice(song.ID, bandID, req.Date); err != nil {
		return err
	}
	return a.bandRehearsalStatsResponse(c, song.ID, bandID)
}

// DeleteBandRehearsal removes a band rehearsal day (Editor+).
func (a *API) DeleteBandRehearsal(c *echo.Context) error {
	bandID, _, err := a.bandAccess(c, model.RoleEditor)
	if err != nil {
		return err
	}
	song, err := a.bandSongForRequest(c, bandID)
	if err != nil {
		return err
	}
	date := c.Param("date")
	if !validPracticeDate(date) {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid date")
	}
	if err := a.Repo.DeleteBandPractice(song.ID, bandID, date); err != nil {
		return err
	}
	return a.bandRehearsalStatsResponse(c, song.ID, bandID)
}

func bandResourceResponse(r *model.Resource) map[string]any {
	return map[string]any{"id": r.ID, "url": r.URL, "label": r.Label}
}

// CreateBandResource appends a band resource (Editor+).
func (a *API) CreateBandResource(c *echo.Context) error {
	bandID, _, err := a.bandAccess(c, model.RoleEditor)
	if err != nil {
		return err
	}
	song, err := a.bandSongForRequest(c, bandID)
	if err != nil {
		return err
	}
	var req resourceRequest // {URL *string; Label *string} from resources.go
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
	res, err := a.Repo.CreateBandResource(song.ID, bandID, *req.URL, label)
	if err != nil {
		return err
	}
	return c.JSON(http.StatusCreated, bandResourceResponse(res))
}

// UpdateBandResource patches a band resource (Editor+).
func (a *API) UpdateBandResource(c *echo.Context) error {
	bandID, _, err := a.bandAccess(c, model.RoleEditor)
	if err != nil {
		return err
	}
	song, err := a.bandSongForRequest(c, bandID)
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
	res, err := a.Repo.UpdateBandResource(rid, song.ID, bandID, req.URL, req.Label)
	if err != nil {
		return notFoundOr(err, "resource")
	}
	return c.JSON(http.StatusOK, bandResourceResponse(res))
}

// DeleteBandResource removes a band resource (Editor+).
func (a *API) DeleteBandResource(c *echo.Context) error {
	bandID, _, err := a.bandAccess(c, model.RoleEditor)
	if err != nil {
		return err
	}
	song, err := a.bandSongForRequest(c, bandID)
	if err != nil {
		return err
	}
	rid, err := resourceID(c)
	if err != nil {
		return err
	}
	if err := a.Repo.DeleteBandResource(rid, song.ID, bandID); err != nil {
		return notFoundOr(err, "resource")
	}
	return c.NoContent(http.StatusNoContent)
}
