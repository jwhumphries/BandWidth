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

func newBandSongsAPI(t *testing.T) (*echo.Echo, *API) {
	t.Helper()
	e, api := newBandsAPI(t) // from bands_test.go: creates band CRUD routes
	g := e.Group("/api/bands/:id/songs", appmw.RequireAuth(api.Repo))
	g.GET("", api.BandSongs)
	g.POST("", api.CreateBandSong)
	g.GET("/:songId", api.BandSong)
	g.PATCH("/:songId", api.UpdateBandSong)
	g.DELETE("/:songId", api.DeleteBandSong)
	return e, api
}

func createBandSongFor(t *testing.T, e *echo.Echo, cookie *http.Cookie, bandID uint, title string) uint {
	t.Helper()
	rec := jsonReq(e, http.MethodPost, fmt.Sprintf("/api/bands/%d/songs", bandID),
		fmt.Sprintf(`{"title":%q}`, title), cookie)
	if rec.Code != http.StatusCreated {
		t.Fatalf("create band song: %d %s", rec.Code, rec.Body.String())
	}
	var created struct {
		ID uint `json:"id"`
	}
	_ = json.Unmarshal(rec.Body.Bytes(), &created)
	return created.ID
}

func TestBandSongCRUD(t *testing.T) {
	e, api := newBandSongsAPI(t)
	alice := signupAndCookie(t, e, "alice") // creator/admin
	bob := signupAndCookie(t, e, "bob")
	band := createBandFor(t, e, alice, "Band")
	_ = api.Repo.AddMember(band, mustUserID(t, api, "bob"), model.RoleViewer)

	songID := createBandSongFor(t, e, alice, band, "Wonderwall")

	// List (any member, incl. viewer).
	rec := jsonReq(e, http.MethodGet, fmt.Sprintf("/api/bands/%d/songs", band), "", bob)
	var list []map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &list); err != nil || len(list) != 1 {
		t.Fatalf("band songs list: %s (%v)", rec.Body.String(), err)
	}

	// Detail shows band-layer fields.
	rec = jsonReq(e, http.MethodGet, fmt.Sprintf("/api/bands/%d/songs/%d", band, songID), "", bob)
	var detail struct {
		Title          string `json:"title"`
		Status         string `json:"status"`
		RehearsalCount int    `json:"rehearsalCount"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &detail); err != nil {
		t.Fatal(err)
	}
	if detail.Title != "Wonderwall" || detail.Status != "not_learned" {
		t.Errorf("band song detail = %+v", detail)
	}

	// Update band identity + band annotation (Editor+; alice is admin).
	rec = jsonReq(e, http.MethodPatch, fmt.Sprintf("/api/bands/%d/songs/%d", band, songID),
		`{"artist":"Oasis","status":"learned","notes":"capo 2"}`, alice)
	if rec.Code != http.StatusOK {
		t.Fatalf("update: %d %s", rec.Code, rec.Body.String())
	}
	rec = jsonReq(e, http.MethodGet, fmt.Sprintf("/api/bands/%d/songs/%d", band, songID), "", alice)
	var after struct {
		Artist string `json:"artist"`
		Status string `json:"status"`
		Notes  string `json:"notes"`
	}
	_ = json.Unmarshal(rec.Body.Bytes(), &after)
	if after.Artist != "Oasis" || after.Status != "learned" || after.Notes != "capo 2" {
		t.Errorf("after update = %+v", after)
	}

	// Viewer cannot create, update, or delete (403).
	if rec := jsonReq(e, http.MethodPost, fmt.Sprintf("/api/bands/%d/songs", band), `{"title":"X"}`, bob); rec.Code != http.StatusForbidden {
		t.Fatalf("viewer create: %d, want 403", rec.Code)
	}
	if rec := jsonReq(e, http.MethodPatch, fmt.Sprintf("/api/bands/%d/songs/%d", band, songID), `{"status":"nailed"}`, bob); rec.Code != http.StatusForbidden {
		t.Fatalf("viewer update: %d, want 403", rec.Code)
	}
	if rec := jsonReq(e, http.MethodDelete, fmt.Sprintf("/api/bands/%d/songs/%d", band, songID), "", bob); rec.Code != http.StatusForbidden {
		t.Fatalf("viewer delete: %d, want 403", rec.Code)
	}

	// Non-members get 404 for the band (band invisible).
	carol := signupAndCookie(t, e, "carol")
	if rec := jsonReq(e, http.MethodGet, fmt.Sprintf("/api/bands/%d/songs", band), "", carol); rec.Code != http.StatusNotFound {
		t.Fatalf("non-member list: %d, want 404", rec.Code)
	}

	// A song from another band is not reachable via this band's path.
	otherBand := createBandFor(t, e, alice, "Other")
	otherSong := createBandSongFor(t, e, alice, otherBand, "Elsewhere")
	if rec := jsonReq(e, http.MethodGet, fmt.Sprintf("/api/bands/%d/songs/%d", band, otherSong), "", alice); rec.Code != http.StatusNotFound {
		t.Fatalf("cross-band song: %d, want 404", rec.Code)
	}

	// Admin deletes.
	if rec := jsonReq(e, http.MethodDelete, fmt.Sprintf("/api/bands/%d/songs/%d", band, songID), "", alice); rec.Code != http.StatusNoContent {
		t.Fatalf("delete: %d", rec.Code)
	}
	if rec := jsonReq(e, http.MethodGet, fmt.Sprintf("/api/bands/%d/songs/%d", band, songID), "", alice); rec.Code != http.StatusNotFound {
		t.Fatalf("detail after delete: %d, want 404", rec.Code)
	}
}
