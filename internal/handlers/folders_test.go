package handlers

import (
	"encoding/json"
	"fmt"
	"net/http"
	"testing"

	"github.com/labstack/echo/v5"

	appmw "github.com/jwhumphries/bandwidth/internal/middleware"
)

func newFoldersAPI(t *testing.T) (*echo.Echo, *API) {
	t.Helper()
	e, api := newSongsAPI(t)
	g := e.Group("/api/folders", appmw.RequireAuth(api.Repo))
	g.GET("", api.Folders)
	g.POST("", api.CreateFolder)
	g.PATCH("/:id", api.UpdateFolder)
	g.DELETE("/:id", api.DeleteFolder)
	g.PUT("/order", api.ReorderFolders)
	g.PUT("/:id/entries", api.SetFolderEntries)
	return e, api
}

func TestFolderEndpoints(t *testing.T) {
	e, _ := newFoldersAPI(t)
	cookie := signupAndCookie(t, e, "alice")
	song := createSongFor(t, e, cookie)

	// Create two folders.
	rec := jsonReq(e, http.MethodPost, "/api/folders", `{"name":"Setlist"}`, cookie)
	if rec.Code != http.StatusCreated {
		t.Fatalf("create folder: %d %s", rec.Code, rec.Body.String())
	}
	var f1 struct {
		ID uint `json:"id"`
	}
	_ = json.Unmarshal(rec.Body.Bytes(), &f1)
	rec = jsonReq(e, http.MethodPost, "/api/folders", `{"name":"Queue"}`, cookie)
	var f2 struct {
		ID uint `json:"id"`
	}
	_ = json.Unmarshal(rec.Body.Bytes(), &f2)

	// Blank name rejected.
	if rec := jsonReq(e, http.MethodPost, "/api/folders", `{"name":" "}`, cookie); rec.Code != http.StatusBadRequest {
		t.Fatalf("blank folder name: %d, want 400", rec.Code)
	}

	// Entries (membership + order).
	rec = jsonReq(e, http.MethodPut, fmt.Sprintf("/api/folders/%d/entries", f1.ID),
		fmt.Sprintf(`{"songIds":[%d]}`, song), cookie)
	if rec.Code != http.StatusNoContent {
		t.Fatalf("set entries: %d %s", rec.Code, rec.Body.String())
	}

	// Unknown song rejected.
	rec = jsonReq(e, http.MethodPut, fmt.Sprintf("/api/folders/%d/entries", f1.ID),
		`{"songIds":[99999]}`, cookie)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("bad entries: %d, want 400", rec.Code)
	}

	// List reflects entries and order.
	rec = jsonReq(e, http.MethodGet, "/api/folders", "", cookie)
	var folders []struct {
		ID      uint   `json:"id"`
		Name    string `json:"name"`
		SongIDs []uint `json:"songIds"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &folders); err != nil || len(folders) != 2 {
		t.Fatalf("folders list: %s (%v)", rec.Body.String(), err)
	}
	if len(folders[0].SongIDs) != 1 || folders[0].SongIDs[0] != song {
		t.Errorf("f1 entries = %v", folders[0].SongIDs)
	}

	// Rename.
	rec = jsonReq(e, http.MethodPatch, fmt.Sprintf("/api/folders/%d", f1.ID),
		`{"name":"Gig"}`, cookie)
	if rec.Code != http.StatusNoContent {
		t.Fatalf("rename: %d", rec.Code)
	}

	// Reorder.
	rec = jsonReq(e, http.MethodPut, "/api/folders/order",
		fmt.Sprintf(`{"folderIds":[%d,%d]}`, f2.ID, f1.ID), cookie)
	if rec.Code != http.StatusNoContent {
		t.Fatalf("reorder: %d %s", rec.Code, rec.Body.String())
	}
	rec = jsonReq(e, http.MethodGet, "/api/folders", "", cookie)
	_ = json.Unmarshal(rec.Body.Bytes(), &folders)
	if folders[0].ID != f2.ID {
		t.Error("reorder not applied")
	}

	// Delete folder; songs survive.
	rec = jsonReq(e, http.MethodDelete, fmt.Sprintf("/api/folders/%d", f1.ID), "", cookie)
	if rec.Code != http.StatusNoContent {
		t.Fatalf("delete folder: %d", rec.Code)
	}
	rec = jsonReq(e, http.MethodGet, "/api/songs", "", cookie)
	var songs []map[string]any
	_ = json.Unmarshal(rec.Body.Bytes(), &songs)
	if len(songs) != 1 {
		t.Errorf("songs after folder delete = %d", len(songs))
	}

	// Cross-user isolation.
	bob := signupAndCookie(t, e, "bob")
	if rec := jsonReq(e, http.MethodPatch, fmt.Sprintf("/api/folders/%d", f2.ID), `{"name":"x"}`, bob); rec.Code != http.StatusNotFound {
		t.Errorf("bob renamed alice folder: %d, want 404", rec.Code)
	}
}
