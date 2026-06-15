package handlers

import (
	"errors"
	"net/http"
	"strconv"
	"strings"

	"github.com/labstack/echo/v5"
	"gorm.io/gorm"

	appmw "github.com/jwhumphries/bandwidth/internal/middleware"
	"github.com/jwhumphries/bandwidth/internal/model"
)

const (
	maxTitleLen = 200
	maxNotesLen = 10000
)

// songID parses the :id path parameter.
func songID(c *echo.Context) (uint, error) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil || id == 0 {
		return 0, echo.NewHTTPError(http.StatusNotFound, "song not found")
	}
	return uint(id), nil
}

// notFoundOr maps gorm.ErrRecordNotFound to a 404 and passes other errors through.
func notFoundOr(err error, what string) error {
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return echo.NewHTTPError(http.StatusNotFound, what+" not found")
	}
	return err
}

// songDetailResponse builds the flat detail payload for one song. The bands
// plan adds a nested "band" object alongside these fields.
func (a *API) songDetailResponse(song *model.Song, userID uint) (map[string]any, error) {
	status := model.StatusNotLearned
	notes := ""
	if ann, err := a.Repo.AnnotationForSongUser(song.ID, userID); err == nil {
		status = ann.Status
		notes = ann.Notes
	} else if !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, err
	}
	resources, err := a.Repo.ResourcesForSongUser(song.ID, userID)
	if err != nil {
		return nil, err
	}
	last, count, err := a.Repo.PracticeStats(song.ID, userID)
	if err != nil {
		return nil, err
	}
	resList := make([]map[string]any, 0, len(resources))
	for _, r := range resources {
		resList = append(resList, map[string]any{
			"id": r.ID, "url": r.URL, "label": r.Label,
		})
	}
	detail := map[string]any{
		"id":              song.ID,
		"title":           song.Title,
		"artist":          song.Artist,
		"status":          status,
		"notes":           notes,
		"resources":       resList,
		"lastPracticedAt": last,
		"practiceCount":   count,
	}
	if song.OwnerBandID != nil {
		band, err := a.Repo.BandByID(*song.OwnerBandID)
		if err != nil {
			return nil, err
		}
		bandLayer, err := a.bandSongDetailResponse(song, *song.OwnerBandID)
		if err != nil {
			return nil, err
		}
		detail["band"] = map[string]any{
			"bandId":          band.ID,
			"bandName":        band.Name,
			"status":          bandLayer["status"],
			"notes":           bandLayer["notes"],
			"resources":       bandLayer["resources"],
			"lastRehearsedAt": bandLayer["lastRehearsedAt"],
			"rehearsalCount":  bandLayer["rehearsalCount"],
		}
	}
	return detail, nil
}

// Songs returns the user's library list.
func (a *API) Songs(c *echo.Context) error {
	user := appmw.CurrentUser(c)
	if user == nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "user not in context")
	}
	items, err := a.Repo.SongsForUser(user.ID)
	if err != nil {
		return err
	}
	return c.JSON(http.StatusOK, items)
}

type createSongRequest struct {
	Title  string `json:"title"`
	Artist string `json:"artist"`
}

// CreateSong adds a personal song.
func (a *API) CreateSong(c *echo.Context) error {
	user := appmw.CurrentUser(c)
	if user == nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "user not in context")
	}
	var req createSongRequest
	if err := c.Bind(&req); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid request body")
	}
	req.Title = strings.TrimSpace(req.Title)
	req.Artist = strings.TrimSpace(req.Artist)
	if req.Title == "" || len(req.Title) > maxTitleLen {
		return echo.NewHTTPError(http.StatusBadRequest,
			"title must be 1-200 characters")
	}
	if len(req.Artist) > maxTitleLen {
		return echo.NewHTTPError(http.StatusBadRequest,
			"artist must be at most 200 characters")
	}
	song, err := a.Repo.CreateSong(user.ID, req.Title, req.Artist)
	if err != nil {
		return err
	}
	detail, err := a.songDetailResponse(song, user.ID)
	if err != nil {
		return err
	}
	return c.JSON(http.StatusCreated, detail)
}

// Song returns one song's detail (identity + the user's metadata layer).
func (a *API) Song(c *echo.Context) error {
	user := appmw.CurrentUser(c)
	if user == nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "user not in context")
	}
	id, err := songID(c)
	if err != nil {
		return err
	}
	song, err := a.Repo.SongVisibleToUser(id, user.ID)
	if err != nil {
		return notFoundOr(err, "song")
	}
	detail, err := a.songDetailResponse(song, user.ID)
	if err != nil {
		return err
	}
	return c.JSON(http.StatusOK, detail)
}

type updateSongRequest struct {
	Title  *string `json:"title"`
	Artist *string `json:"artist"`
	Status *string `json:"status"`
	Notes  *string `json:"notes"`
}

// UpdateSong patches identity (owner only) and/or the user's annotation.
func (a *API) UpdateSong(c *echo.Context) error {
	user := appmw.CurrentUser(c)
	if user == nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "user not in context")
	}
	id, err := songID(c)
	if err != nil {
		return err
	}
	song, err := a.Repo.SongVisibleToUser(id, user.ID)
	if err != nil {
		return notFoundOr(err, "song")
	}
	var req updateSongRequest
	if err := c.Bind(&req); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid request body")
	}
	if song.OwnerBandID != nil && (req.Title != nil || req.Artist != nil) {
		return echo.NewHTTPError(http.StatusForbidden,
			"a band song's title and artist are managed in the band view")
	}

	// Validate all fields before any write.
	var stagedTitle *string
	if req.Title != nil {
		title := strings.TrimSpace(*req.Title)
		if title == "" || len(title) > maxTitleLen {
			return echo.NewHTTPError(http.StatusBadRequest,
				"title must be 1-200 characters")
		}
		stagedTitle = &title
	}
	var stagedArtist *string
	if req.Artist != nil {
		artist := strings.TrimSpace(*req.Artist)
		if len(artist) > maxTitleLen {
			return echo.NewHTTPError(http.StatusBadRequest,
				"artist must be at most 200 characters")
		}
		stagedArtist = &artist
	}
	var status *model.SongStatus
	if req.Status != nil {
		s := model.SongStatus(*req.Status)
		if !s.Valid() {
			return echo.NewHTTPError(http.StatusBadRequest, "invalid status")
		}
		status = &s
	}
	if req.Notes != nil && len(*req.Notes) > maxNotesLen {
		return echo.NewHTTPError(http.StatusBadRequest, "notes too long")
	}

	// All fields valid — now write.
	if stagedTitle != nil || stagedArtist != nil {
		if stagedTitle != nil {
			song.Title = *stagedTitle
		}
		if stagedArtist != nil {
			song.Artist = *stagedArtist
		}
		if err := a.Repo.SaveSong(song); err != nil {
			return err
		}
	}
	if status != nil || req.Notes != nil {
		if err := a.Repo.UpsertAnnotation(song.ID, user.ID, status, req.Notes); err != nil {
			return err
		}
	}

	detail, err := a.songDetailResponse(song, user.ID)
	if err != nil {
		return err
	}
	return c.JSON(http.StatusOK, detail)
}

// DeleteSong removes an owned song and all attached data.
func (a *API) DeleteSong(c *echo.Context) error {
	user := appmw.CurrentUser(c)
	if user == nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "user not in context")
	}
	id, err := songID(c)
	if err != nil {
		return err
	}
	song, err := a.Repo.SongVisibleToUser(id, user.ID)
	if err != nil {
		return notFoundOr(err, "song")
	}
	if song.OwnerBandID != nil {
		return echo.NewHTTPError(http.StatusForbidden,
			"band songs cannot be deleted from your library")
	}
	if err := a.Repo.DeleteSong(song.ID, user.ID); err != nil {
		return notFoundOr(err, "song")
	}
	return c.NoContent(http.StatusNoContent)
}
