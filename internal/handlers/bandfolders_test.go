package handlers

import (
	"encoding/json"
	"fmt"
	"net/http"
	"testing"

	"github.com/labstack/echo/v5"

	appmw "github.com/jwhumphries/bandwidth/internal/middleware"
	"github.com/jwhumphries/bandwidth/internal/model"
)

// newBandFoldersAPI wires band CRUD, band song creation, and band folder routes.
func newBandFoldersAPI(t *testing.T) (*echo.Echo, *API) {
	t.Helper()
	e, api := newBandSongsAPI(t)
	g := e.Group("/api/bands/:id/folders", appmw.RequireAuth(api.Repo))
	g.GET("", api.BandFolders)
	g.POST("", api.CreateBandFolder)
	g.PUT("/order", api.ReorderBandFolders)
	g.PATCH("/:folderId", api.UpdateBandFolder)
	g.DELETE("/:folderId", api.DeleteBandFolder)
	g.PUT("/:folderId/entries", api.SetBandFolderEntries)
	return e, api
}

// addMemberAs adds username as a member of band with the given role string
// ("viewer", "editor", "admin") using the repo directly.
func addMemberAs(t *testing.T, _ *echo.Echo, _ *http.Cookie, api *API, band uint, username string, roleStr string) {
	t.Helper()
	var role model.BandRole
	switch roleStr {
	case "viewer":
		role = model.RoleViewer
	case "editor":
		role = model.RoleEditor
	case "admin":
		role = model.RoleAdmin
	default:
		t.Fatalf("addMemberAs: unknown role %q", roleStr)
	}
	userID := mustUserID(t, api, username)
	if err := api.Repo.AddMember(band, userID, role); err != nil {
		t.Fatalf("addMemberAs %s as %s: %v", username, roleStr, err)
	}
}

func TestBandFolderHandlers(t *testing.T) {
	e, api := newBandFoldersAPI(t)
	alice := signupAndCookie(t, e, "alice") // creator/admin
	bob := signupAndCookie(t, e, "bob")
	band := createBandFor(t, e, alice, "The Quietones")
	addMemberAs(t, e, alice, api, band, "bob", "viewer")
	songID := createBandSongFor(t, e, alice, band, "Wonderwall")
	base := fmt.Sprintf("/api/bands/%d/folders", band)

	// Editor/Admin creates a folder.
	rec := jsonReq(e, http.MethodPost, base, `{"name":"Set 1"}`, alice)
	if rec.Code != http.StatusCreated {
		t.Fatalf("create folder: %d %s", rec.Code, rec.Body.String())
	}
	var folder struct {
		ID uint `json:"id"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &folder); err != nil {
		t.Fatalf("decode folder: %v", err)
	}

	// Put the band song in it.
	if rec := jsonReq(e, http.MethodPut, fmt.Sprintf("%s/%d/entries", base, folder.ID),
		fmt.Sprintf(`{"songIds":[%d]}`, songID), alice); rec.Code != http.StatusNoContent {
		t.Fatalf("set entries: %d %s", rec.Code, rec.Body.String())
	}

	// Viewer can read the folder list...
	rec = jsonReq(e, http.MethodGet, base, "", bob)
	if rec.Code != http.StatusOK {
		t.Fatalf("viewer list: %d %s", rec.Code, rec.Body.String())
	}
	var list []map[string]any
	_ = json.Unmarshal(rec.Body.Bytes(), &list)
	if len(list) != 1 {
		t.Fatalf("folder list = %s", rec.Body.String())
	}

	// ...but cannot create, rename, reorder, set entries, or delete.
	if rec := jsonReq(e, http.MethodPost, base, `{"name":"Nope"}`, bob); rec.Code != http.StatusForbidden {
		t.Fatalf("viewer create: %d, want 403", rec.Code)
	}
	if rec := jsonReq(e, http.MethodPatch, fmt.Sprintf("%s/%d", base, folder.ID), `{"name":"Nope"}`, bob); rec.Code != http.StatusForbidden {
		t.Fatalf("viewer rename: %d, want 403", rec.Code)
	}
	if rec := jsonReq(e, http.MethodPut, fmt.Sprintf("%s/%d/entries", base, folder.ID), `{"songIds":[]}`, bob); rec.Code != http.StatusForbidden {
		t.Fatalf("viewer set entries: %d, want 403", rec.Code)
	}
	if rec := jsonReq(e, http.MethodDelete, fmt.Sprintf("%s/%d", base, folder.ID), "", bob); rec.Code != http.StatusForbidden {
		t.Fatalf("viewer delete: %d, want 403", rec.Code)
	}

	// Editor renames then deletes.
	if rec := jsonReq(e, http.MethodPatch, fmt.Sprintf("%s/%d", base, folder.ID), `{"name":"Opener"}`, alice); rec.Code != http.StatusNoContent {
		t.Fatalf("rename: %d %s", rec.Code, rec.Body.String())
	}
	if rec := jsonReq(e, http.MethodDelete, fmt.Sprintf("%s/%d", base, folder.ID), "", alice); rec.Code != http.StatusNoContent {
		t.Fatalf("delete: %d %s", rec.Code, rec.Body.String())
	}
}
