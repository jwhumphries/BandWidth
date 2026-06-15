package handlers

import (
	"errors"
	"net/http"
	"strconv"
	"strings"

	"github.com/labstack/echo/v5"
	"gorm.io/gorm"

	"github.com/jwhumphries/bandwidth/internal/model"
)

// bandSongID parses the :songId path parameter.
func bandSongID(c *echo.Context) (uint, error) {
	id, err := strconv.ParseUint(c.Param("songId"), 10, 32)
	if err != nil || id == 0 {
		return 0, echo.NewHTTPError(http.StatusNotFound, "song not found")
	}
	return uint(id), nil
}

// bandSongForRequest resolves :songId and confirms it belongs to bandID.
func (a *API) bandSongForRequest(c *echo.Context, bandID uint) (*model.Song, error) {
	sid, err := bandSongID(c)
	if err != nil {
		return nil, err
	}
	song, err := a.Repo.SongForBand(sid, bandID)
	if err != nil {
		return nil, notFoundOr(err, "song")
	}
	return song, nil
}

// bandSongDetailResponse builds the band-layer detail for one band song.
func (a *API) bandSongDetailResponse(song *model.Song, bandID uint) (map[string]any, error) {
	status := model.StatusNotLearned
	notes := ""
	if ann, err := a.Repo.BandAnnotationForSong(song.ID, bandID); err == nil {
		status = ann.Status
		notes = ann.Notes
	} else if !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, err
	}
	resources, err := a.Repo.ResourcesForSongBand(song.ID, bandID)
	if err != nil {
		return nil, err
	}
	last, count, err := a.Repo.BandPracticeStats(song.ID, bandID)
	if err != nil {
		return nil, err
	}
	resList := make([]map[string]any, 0, len(resources))
	for _, r := range resources {
		resList = append(resList, map[string]any{"id": r.ID, "url": r.URL, "label": r.Label})
	}
	return map[string]any{
		"id":              song.ID,
		"title":           song.Title,
		"artist":          song.Artist,
		"status":          status,
		"notes":           notes,
		"resources":       resList,
		"lastRehearsedAt": last,
		"rehearsalCount":  count,
	}, nil
}

// BandSongs lists a band's songs with the band layer (any member).
func (a *API) BandSongs(c *echo.Context) error {
	bandID, _, err := a.bandAccess(c, model.RoleViewer)
	if err != nil {
		return err
	}
	items, err := a.Repo.SongsForBand(bandID)
	if err != nil {
		return err
	}
	return c.JSON(http.StatusOK, items)
}

type bandSongCreateRequest struct {
	Title  string `json:"title"`
	Artist string `json:"artist"`
}

// CreateBandSong adds a band song (Editor+).
func (a *API) CreateBandSong(c *echo.Context) error {
	bandID, _, err := a.bandAccess(c, model.RoleEditor)
	if err != nil {
		return err
	}
	var req bandSongCreateRequest
	if err := c.Bind(&req); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid request body")
	}
	req.Title = strings.TrimSpace(req.Title)
	req.Artist = strings.TrimSpace(req.Artist)
	if req.Title == "" || len(req.Title) > maxTitleLen || len(req.Artist) > maxTitleLen {
		return echo.NewHTTPError(http.StatusBadRequest,
			"a title (at most 200 characters) is required")
	}
	song, err := a.Repo.CreateBandSong(bandID, req.Title, req.Artist)
	if err != nil {
		return err
	}
	detail, err := a.bandSongDetailResponse(song, bandID)
	if err != nil {
		return err
	}
	return c.JSON(http.StatusCreated, detail)
}

// BandSong returns a band song's band-layer detail (any member).
func (a *API) BandSong(c *echo.Context) error {
	bandID, _, err := a.bandAccess(c, model.RoleViewer)
	if err != nil {
		return err
	}
	song, err := a.bandSongForRequest(c, bandID)
	if err != nil {
		return err
	}
	detail, err := a.bandSongDetailResponse(song, bandID)
	if err != nil {
		return err
	}
	return c.JSON(http.StatusOK, detail)
}

type bandSongUpdateRequest struct {
	Title  *string `json:"title"`
	Artist *string `json:"artist"`
	Status *string `json:"status"`
	Notes  *string `json:"notes"`
}

// UpdateBandSong patches a band song's identity and band annotation (Editor+),
// validating all fields before any write.
func (a *API) UpdateBandSong(c *echo.Context) error {
	bandID, _, err := a.bandAccess(c, model.RoleEditor)
	if err != nil {
		return err
	}
	song, err := a.bandSongForRequest(c, bandID)
	if err != nil {
		return err
	}
	var req bandSongUpdateRequest
	if err := c.Bind(&req); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid request body")
	}

	var stagedTitle *string
	if req.Title != nil {
		title := strings.TrimSpace(*req.Title)
		if title == "" || len(title) > maxTitleLen {
			return echo.NewHTTPError(http.StatusBadRequest, "title must be 1-200 characters")
		}
		stagedTitle = &title
	}
	var stagedArtist *string
	if req.Artist != nil {
		artist := strings.TrimSpace(*req.Artist)
		if len(artist) > maxTitleLen {
			return echo.NewHTTPError(http.StatusBadRequest, "artist must be at most 200 characters")
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
		if err := a.Repo.UpsertBandAnnotation(song.ID, bandID, status, req.Notes); err != nil {
			return err
		}
	}

	detail, err := a.bandSongDetailResponse(song, bandID)
	if err != nil {
		return err
	}
	return c.JSON(http.StatusOK, detail)
}

// DeleteBandSong removes a band song (Editor+); each member's personal work
// is converted to a personal copy by the repository.
func (a *API) DeleteBandSong(c *echo.Context) error {
	bandID, _, err := a.bandAccess(c, model.RoleEditor)
	if err != nil {
		return err
	}
	song, err := a.bandSongForRequest(c, bandID)
	if err != nil {
		return err
	}
	if err := a.Repo.DeleteBandSong(song.ID, bandID); err != nil {
		return err
	}
	return c.NoContent(http.StatusNoContent)
}
