package handlers

import (
	"encoding/json"
	"fmt"
	"net/http"
	"testing"
	"time"

	"github.com/labstack/echo/v5"

	appmw "github.com/jwhumphries/bandwidth/internal/middleware"
	"github.com/jwhumphries/bandwidth/internal/model"
)

func newBandPracticeAPI(t *testing.T) (*echo.Echo, *API) {
	t.Helper()
	e, api := newBandSongsAPI(t)
	g := e.Group("/api/bands/:id/songs/:songId", appmw.RequireAuth(api.Repo))
	g.PUT("/rehearsal", api.LogBandRehearsal)
	g.DELETE("/rehearsal/:date", api.DeleteBandRehearsal)
	g.POST("/resources", api.CreateBandResource)
	g.PATCH("/resources/:resourceId", api.UpdateBandResource)
	g.DELETE("/resources/:resourceId", api.DeleteBandResource)
	return e, api
}

func TestBandRehearsalAndResources(t *testing.T) {
	e, api := newBandPracticeAPI(t)
	alice := signupAndCookie(t, e, "alice")
	bob := signupAndCookie(t, e, "bob")
	band := createBandFor(t, e, alice, "Band")
	_ = api.Repo.AddMember(band, mustUserID(t, api, "bob"), model.RoleViewer)
	song := createBandSongFor(t, e, alice, band, "Wonderwall")

	base := fmt.Sprintf("/api/bands/%d/songs/%d", band, song)

	// Log a rehearsal (Editor+); response is band rehearsal stats.
	rec := jsonReq(e, http.MethodPut, base+"/rehearsal", `{"date":"2026-06-10"}`, alice)
	if rec.Code != http.StatusOK {
		t.Fatalf("rehearsal: %d %s", rec.Code, rec.Body.String())
	}
	var stats struct {
		LastRehearsedAt string `json:"lastRehearsedAt"`
		RehearsalCount  int    `json:"rehearsalCount"`
	}
	_ = json.Unmarshal(rec.Body.Bytes(), &stats)
	if stats.LastRehearsedAt != "2026-06-10" || stats.RehearsalCount != 1 {
		t.Errorf("rehearsal stats = %+v", stats)
	}

	// Empty body defaults to today. Capture the reference date just before the
	// request so a UTC-midnight rollover between request and assertion can't
	// flake the comparison.
	today := time.Now().UTC().Format("2006-01-02")
	rec = jsonReq(e, http.MethodPut, base+"/rehearsal", "{}", alice)
	_ = json.Unmarshal(rec.Body.Bytes(), &stats)
	if stats.LastRehearsedAt != today {
		t.Errorf("default date = %q", stats.LastRehearsedAt)
	}

	// Viewer cannot log rehearsals.
	if rec := jsonReq(e, http.MethodPut, base+"/rehearsal", "{}", bob); rec.Code != http.StatusForbidden {
		t.Fatalf("viewer rehearsal: %d, want 403", rec.Code)
	}

	// Undo a rehearsal.
	if rec := jsonReq(e, http.MethodDelete, base+"/rehearsal/2026-06-10", "", alice); rec.Code != http.StatusOK {
		t.Fatalf("delete rehearsal: %d", rec.Code)
	}

	// Band resource lifecycle (Editor+).
	rec = jsonReq(e, http.MethodPost, base+"/resources",
		`{"url":"https://example.com/tab","label":"tab"}`, alice)
	if rec.Code != http.StatusCreated {
		t.Fatalf("create band resource: %d %s", rec.Code, rec.Body.String())
	}
	var res struct {
		ID uint `json:"id"`
	}
	_ = json.Unmarshal(rec.Body.Bytes(), &res)
	if rec := jsonReq(e, http.MethodPatch, fmt.Sprintf("%s/resources/%d", base, res.ID), `{"label":"chords"}`, alice); rec.Code != http.StatusOK {
		t.Fatalf("update band resource: %d", rec.Code)
	}
	if rec := jsonReq(e, http.MethodDelete, fmt.Sprintf("%s/resources/%d", base, res.ID), "", alice); rec.Code != http.StatusNoContent {
		t.Fatalf("delete band resource: %d", rec.Code)
	}
	// Viewer cannot create band resources.
	if rec := jsonReq(e, http.MethodPost, base+"/resources", `{"url":"https://example.com"}`, bob); rec.Code != http.StatusForbidden {
		t.Fatalf("viewer band resource: %d, want 403", rec.Code)
	}
}
