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

// newInterleaveAPI wires the personal song routes plus band-song creation,
// so a band song can be set up and then viewed from the personal side.
func newInterleaveAPI(t *testing.T) (*echo.Echo, *API) {
	t.Helper()
	e, api := newBandSongsAPI(t) // band CRUD + band song routes
	g := e.Group("/api/songs", appmw.RequireAuth(api.Repo))
	g.GET("", api.Songs)
	g.GET("/:id", api.Song)
	g.PATCH("/:id", api.UpdateSong)
	g.DELETE("/:id", api.DeleteSong)
	g.PUT("/:id/practice", api.LogPractice)
	g.POST("/:id/resources", api.CreateResource)
	return e, api
}

func TestBandSongInPersonalView(t *testing.T) {
	e, api := newInterleaveAPI(t)
	alice := signupAndCookie(t, e, "alice")
	band := createBandFor(t, e, alice, "The Quietones")
	songID := createBandSongFor(t, e, alice, band, "Wonderwall")
	// Give the band layer a status + a band resource.
	_ = api.Repo.UpsertBandAnnotation(songID, band, ptrStatus(model.StatusNailed), ptrString("band notes"))
	_, _ = api.Repo.CreateBandResource(songID, band, "https://example.com/band", "band tab")

	// The band song appears in alice's personal library, tagged.
	rec := jsonReq(e, http.MethodGet, "/api/songs", "", alice)
	var list []map[string]any
	_ = json.Unmarshal(rec.Body.Bytes(), &list)
	if len(list) != 1 || list[0]["bandId"] == nil {
		t.Fatalf("personal list missing tagged band song: %s", rec.Body.String())
	}

	// Personal detail shows the user's own (default) layer plus a read-only
	// band section.
	rec = jsonReq(e, http.MethodGet, fmt.Sprintf("/api/songs/%d", songID), "", alice)
	if rec.Code != http.StatusOK {
		t.Fatalf("personal detail: %d %s", rec.Code, rec.Body.String())
	}
	var detail struct {
		Status string `json:"status"`
		Band   *struct {
			BandName       string `json:"bandName"`
			Status         string `json:"status"`
			Notes          string `json:"notes"`
			RehearsalCount int    `json:"rehearsalCount"`
			Resources      []any  `json:"resources"`
		} `json:"band"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &detail); err != nil {
		t.Fatal(err)
	}
	if detail.Status != "not_learned" {
		t.Errorf("personal status = %q, want the member's own default", detail.Status)
	}
	if detail.Band == nil || detail.Band.BandName != "The Quietones" || detail.Band.Status != "nailed" || detail.Band.Notes != "band notes" {
		t.Fatalf("band section = %+v", detail.Band)
	}
	if len(detail.Band.Resources) != 1 {
		t.Errorf("band resources in section = %d", len(detail.Band.Resources))
	}

	// The member CAN set their personal status on the band song...
	if rec := jsonReq(e, http.MethodPatch, fmt.Sprintf("/api/songs/%d", songID), `{"status":"learning","notes":"my take"}`, alice); rec.Code != http.StatusOK {
		t.Fatalf("personal status patch: %d %s", rec.Code, rec.Body.String())
	}
	// ...and log personal practice on it...
	if rec := jsonReq(e, http.MethodPut, fmt.Sprintf("/api/songs/%d/practice", songID), `{"date":"2026-06-10"}`, alice); rec.Code != http.StatusOK {
		t.Fatalf("personal practice: %d", rec.Code)
	}
	// ...but NOT edit the band-owned identity from the personal view...
	if rec := jsonReq(e, http.MethodPatch, fmt.Sprintf("/api/songs/%d", songID), `{"title":"Hijacked"}`, alice); rec.Code != http.StatusForbidden {
		t.Fatalf("personal identity edit: %d, want 403", rec.Code)
	}
	// ...nor delete the band song from their library.
	if rec := jsonReq(e, http.MethodDelete, fmt.Sprintf("/api/songs/%d", songID), "", alice); rec.Code != http.StatusForbidden {
		t.Fatalf("personal delete of band song: %d, want 403", rec.Code)
	}

	// The band layer is unchanged by the member's personal edits.
	bandAnn, _ := api.Repo.BandAnnotationForSong(songID, band)
	if bandAnn.Status != model.StatusNailed {
		t.Errorf("band status changed by member: %q", bandAnn.Status)
	}
}

func ptrStatus(s model.SongStatus) *model.SongStatus { return &s }
func ptrString(s string) *string                     { return &s }
