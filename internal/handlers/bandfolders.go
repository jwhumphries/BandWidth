package handlers

import (
	"net/http"
	"strconv"
	"strings"

	"github.com/labstack/echo/v5"

	"github.com/jwhumphries/bandwidth/internal/model"
)

// bandFolderID parses the :folderId path parameter.
func bandFolderID(c *echo.Context) (uint, error) {
	id, err := strconv.ParseUint(c.Param("folderId"), 10, 32)
	if err != nil || id == 0 {
		return 0, echo.NewHTTPError(http.StatusNotFound, "folder not found")
	}
	return uint(id), nil
}

// BandFolders lists a band's folders with ordered song IDs (Viewer+).
func (a *API) BandFolders(c *echo.Context) error {
	bandID, _, err := a.bandAccess(c, model.RoleViewer)
	if err != nil {
		return err
	}
	folders, err := a.Repo.FoldersForBand(bandID)
	if err != nil {
		return err
	}
	return c.JSON(http.StatusOK, folders)
}

// CreateBandFolder adds a folder to the band (Editor+).
func (a *API) CreateBandFolder(c *echo.Context) error {
	bandID, _, err := a.bandAccess(c, model.RoleEditor)
	if err != nil {
		return err
	}
	var req folderNameRequest
	if err := c.Bind(&req); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid request body")
	}
	req.Name = strings.TrimSpace(req.Name)
	if req.Name == "" || len(req.Name) > maxTitleLen {
		return echo.NewHTTPError(http.StatusBadRequest, "a folder name is required")
	}
	folder, err := a.Repo.CreateBandFolder(bandID, req.Name)
	if err != nil {
		return err
	}
	return c.JSON(http.StatusCreated, folderResponse(folder))
}

// UpdateBandFolder renames a band folder (Editor+).
func (a *API) UpdateBandFolder(c *echo.Context) error {
	bandID, _, err := a.bandAccess(c, model.RoleEditor)
	if err != nil {
		return err
	}
	fid, err := bandFolderID(c)
	if err != nil {
		return err
	}
	var req folderNameRequest
	if err := c.Bind(&req); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid request body")
	}
	req.Name = strings.TrimSpace(req.Name)
	if req.Name == "" || len(req.Name) > maxTitleLen {
		return echo.NewHTTPError(http.StatusBadRequest, "a folder name is required")
	}
	if err := a.Repo.RenameBandFolder(fid, bandID, req.Name); err != nil {
		return notFoundOr(err, "folder")
	}
	return c.NoContent(http.StatusNoContent)
}

// DeleteBandFolder removes a band folder; band songs are untouched (Editor+).
func (a *API) DeleteBandFolder(c *echo.Context) error {
	bandID, _, err := a.bandAccess(c, model.RoleEditor)
	if err != nil {
		return err
	}
	fid, err := bandFolderID(c)
	if err != nil {
		return err
	}
	if err := a.Repo.DeleteBandFolder(fid, bandID); err != nil {
		return notFoundOr(err, "folder")
	}
	return c.NoContent(http.StatusNoContent)
}

// ReorderBandFolders applies a new folder order (Editor+).
func (a *API) ReorderBandFolders(c *echo.Context) error {
	bandID, _, err := a.bandAccess(c, model.RoleEditor)
	if err != nil {
		return err
	}
	var req reorderFoldersRequest
	if err := c.Bind(&req); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid request body")
	}
	if err := a.Repo.ReorderBandFolders(bandID, req.FolderIDs); err != nil {
		return badRequestOrErr(err, "one or more folders not found")
	}
	return c.NoContent(http.StatusNoContent)
}

// SetBandFolderEntries replaces a band folder's membership and order (Editor+).
func (a *API) SetBandFolderEntries(c *echo.Context) error {
	bandID, _, err := a.bandAccess(c, model.RoleEditor)
	if err != nil {
		return err
	}
	fid, err := bandFolderID(c)
	if err != nil {
		return err
	}
	var req folderEntriesRequest
	if err := c.Bind(&req); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid request body")
	}
	seen := map[uint]bool{}
	songIDs := make([]uint, 0, len(req.SongIDs))
	for _, sid := range req.SongIDs {
		if !seen[sid] {
			seen[sid] = true
			songIDs = append(songIDs, sid)
		}
	}
	if err := a.Repo.SetBandFolderEntries(fid, bandID, songIDs); err != nil {
		return badRequestOrErr(err, "folder or songs not found")
	}
	return c.NoContent(http.StatusNoContent)
}
