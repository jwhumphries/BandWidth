package handlers

import (
	"net/http"
	"strings"

	"github.com/labstack/echo/v5"

	appmw "github.com/jwhumphries/bandwidth/internal/middleware"
	"github.com/jwhumphries/bandwidth/internal/model"
)

func folderResponse(f *model.Folder) map[string]any {
	return map[string]any{
		"id": f.ID, "name": f.Name, "position": f.Position, "songIds": []uint{},
	}
}

// Folders lists the user's folders with ordered song IDs.
func (a *API) Folders(c *echo.Context) error {
	user := appmw.CurrentUser(c)
	if user == nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "user not in context")
	}
	folders, err := a.Repo.FoldersForUser(user.ID)
	if err != nil {
		return err
	}
	return c.JSON(http.StatusOK, folders)
}

type folderNameRequest struct {
	Name string `json:"name"`
}

// CreateFolder adds a folder.
func (a *API) CreateFolder(c *echo.Context) error {
	user := appmw.CurrentUser(c)
	if user == nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "user not in context")
	}
	var req folderNameRequest
	if err := c.Bind(&req); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid request body")
	}
	req.Name = strings.TrimSpace(req.Name)
	if req.Name == "" || len(req.Name) > maxTitleLen {
		return echo.NewHTTPError(http.StatusBadRequest, "a folder name is required")
	}
	folder, err := a.Repo.CreateFolder(user.ID, req.Name)
	if err != nil {
		return err
	}
	return c.JSON(http.StatusCreated, folderResponse(folder))
}

// UpdateFolder renames a folder.
func (a *API) UpdateFolder(c *echo.Context) error {
	user := appmw.CurrentUser(c)
	if user == nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "user not in context")
	}
	id, err := songID(c) // same :id uint parsing
	if err != nil {
		return echo.NewHTTPError(http.StatusNotFound, "folder not found")
	}
	var req folderNameRequest
	if err := c.Bind(&req); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid request body")
	}
	req.Name = strings.TrimSpace(req.Name)
	if req.Name == "" || len(req.Name) > maxTitleLen {
		return echo.NewHTTPError(http.StatusBadRequest, "a folder name is required")
	}
	if err := a.Repo.RenameFolder(id, user.ID, req.Name); err != nil {
		return notFoundOr(err, "folder")
	}
	return c.NoContent(http.StatusNoContent)
}

// DeleteFolder removes a folder (entries only; songs are untouched).
func (a *API) DeleteFolder(c *echo.Context) error {
	user := appmw.CurrentUser(c)
	if user == nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "user not in context")
	}
	id, err := songID(c)
	if err != nil {
		return echo.NewHTTPError(http.StatusNotFound, "folder not found")
	}
	if err := a.Repo.DeleteFolder(id, user.ID); err != nil {
		return notFoundOr(err, "folder")
	}
	return c.NoContent(http.StatusNoContent)
}

type reorderFoldersRequest struct {
	FolderIDs []uint `json:"folderIds"`
}

// ReorderFolders applies a new folder order.
func (a *API) ReorderFolders(c *echo.Context) error {
	user := appmw.CurrentUser(c)
	if user == nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "user not in context")
	}
	var req reorderFoldersRequest
	if err := c.Bind(&req); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid request body")
	}
	if err := a.Repo.ReorderFolders(user.ID, req.FolderIDs); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "one or more folders not found")
	}
	return c.NoContent(http.StatusNoContent)
}

type folderEntriesRequest struct {
	SongIDs []uint `json:"songIds"`
}

// SetFolderEntries replaces a folder's membership and order.
func (a *API) SetFolderEntries(c *echo.Context) error {
	user := appmw.CurrentUser(c)
	if user == nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "user not in context")
	}
	id, err := songID(c)
	if err != nil {
		return echo.NewHTTPError(http.StatusNotFound, "folder not found")
	}
	var req folderEntriesRequest
	if err := c.Bind(&req); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid request body")
	}
	// Dedupe while preserving first-seen order.
	seen := map[uint]bool{}
	songIDs := make([]uint, 0, len(req.SongIDs))
	for _, sid := range req.SongIDs {
		if !seen[sid] {
			seen[sid] = true
			songIDs = append(songIDs, sid)
		}
	}
	if err := a.Repo.SetFolderEntries(id, user.ID, songIDs); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "folder or songs not found")
	}
	return c.NoContent(http.StatusNoContent)
}
