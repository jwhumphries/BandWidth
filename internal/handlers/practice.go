package handlers

import (
	"net/http"
	"time"

	"github.com/labstack/echo/v5"

	appmw "github.com/jwhumphries/bandwidth/internal/middleware"
)

// validPracticeDate parses a YYYY-MM-DD date and rejects far-future entries
// (48 hours of slack absorbs timezone differences with the client).
func validPracticeDate(date string) bool {
	parsed, err := time.Parse("2006-01-02", date)
	if err != nil {
		return false
	}
	return parsed.Before(time.Now().UTC().Add(48 * time.Hour))
}

func (a *API) practiceStatsResponse(c *echo.Context, songID, userID uint) error {
	last, count, err := a.Repo.PracticeStats(songID, userID)
	if err != nil {
		return err
	}
	return c.JSON(http.StatusOK, map[string]any{
		"lastPracticedAt": last,
		"practiceCount":   count,
	})
}

type logPracticeRequest struct {
	Date string `json:"date"`
}

// LogPractice records a practice day (default: today, UTC) idempotently.
func (a *API) LogPractice(c *echo.Context) error {
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
	var req logPracticeRequest
	if err := c.Bind(&req); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid request body")
	}
	if req.Date == "" {
		req.Date = time.Now().UTC().Format("2006-01-02")
	}
	if !validPracticeDate(req.Date) {
		return echo.NewHTTPError(http.StatusBadRequest, "date must be YYYY-MM-DD and not in the future")
	}
	if err := a.Repo.LogPractice(id, user.ID, req.Date); err != nil {
		return err
	}
	return a.practiceStatsResponse(c, id, user.ID)
}

// DeletePractice removes one practiced day (undo).
func (a *API) DeletePractice(c *echo.Context) error {
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
	date := c.Param("date")
	if !validPracticeDate(date) {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid date")
	}
	if err := a.Repo.DeletePractice(id, user.ID, date); err != nil {
		return err
	}
	return a.practiceStatsResponse(c, id, user.ID)
}
