package handlers

import (
	"encoding/json"
	"fmt"
	"net/http"
	"testing"
	"time"

	"github.com/labstack/echo/v5"

	appmw "github.com/jwhumphries/bandwidth/internal/middleware"
)

func newPracticeAPI(t *testing.T) (*echo.Echo, *API) {
	t.Helper()
	e, api := newSongsAPI(t)
	g := e.Group("/api/songs", appmw.RequireAuth(api.Repo))
	g.PUT("/:id/practice", api.LogPractice)
	g.DELETE("/:id/practice/:date", api.DeletePractice)
	g.POST("/:id/resources", api.CreateResource)
	g.PATCH("/:id/resources/:resourceId", api.UpdateResource)
	g.DELETE("/:id/resources/:resourceId", api.DeleteResource)
	return e, api
}

func createSongFor(t *testing.T, e *echo.Echo, cookie *http.Cookie) uint {
	t.Helper()
	rec := jsonReq(e, http.MethodPost, "/api/songs", `{"title":"Wonderwall"}`, cookie)
	if rec.Code != http.StatusCreated {
		t.Fatalf("create song: %d %s", rec.Code, rec.Body.String())
	}
	var created struct {
		ID uint `json:"id"`
	}
	_ = json.Unmarshal(rec.Body.Bytes(), &created)
	return created.ID
}

func TestPracticeEndpoints(t *testing.T) {
	e, _ := newPracticeAPI(t)
	cookie := signupAndCookie(t, e, "alice")
	id := createSongFor(t, e, cookie)

	// Explicit date.
	rec := jsonReq(e, http.MethodPut, fmt.Sprintf("/api/songs/%d/practice", id),
		`{"date":"2026-06-10"}`, cookie)
	if rec.Code != http.StatusOK {
		t.Fatalf("practice: %d %s", rec.Code, rec.Body.String())
	}
	var stats struct {
		LastPracticedAt string `json:"lastPracticedAt"`
		PracticeCount   int    `json:"practiceCount"`
	}
	_ = json.Unmarshal(rec.Body.Bytes(), &stats)
	if stats.LastPracticedAt != "2026-06-10" || stats.PracticeCount != 1 {
		t.Errorf("stats = %+v", stats)
	}

	// Same day again: idempotent.
	rec = jsonReq(e, http.MethodPut, fmt.Sprintf("/api/songs/%d/practice", id),
		`{"date":"2026-06-10"}`, cookie)
	_ = json.Unmarshal(rec.Body.Bytes(), &stats)
	if stats.PracticeCount != 1 {
		t.Errorf("idempotency: count = %d", stats.PracticeCount)
	}

	// Empty body defaults to today.
	rec = jsonReq(e, http.MethodPut, fmt.Sprintf("/api/songs/%d/practice", id), "{}", cookie)
	_ = json.Unmarshal(rec.Body.Bytes(), &stats)
	if stats.LastPracticedAt != time.Now().UTC().Format("2006-01-02") {
		t.Errorf("default date = %q", stats.LastPracticedAt)
	}

	// Bad dates.
	for _, body := range []string{`{"date":"junk"}`, `{"date":"2126-01-01"}`} {
		rec = jsonReq(e, http.MethodPut, fmt.Sprintf("/api/songs/%d/practice", id), body, cookie)
		if rec.Code != http.StatusBadRequest {
			t.Errorf("bad date %s: %d, want 400", body, rec.Code)
		}
	}

	// Undo.
	rec = jsonReq(e, http.MethodDelete,
		fmt.Sprintf("/api/songs/%d/practice/2026-06-10", id), "", cookie)
	if rec.Code != http.StatusOK {
		t.Fatalf("delete practice: %d", rec.Code)
	}
	_ = json.Unmarshal(rec.Body.Bytes(), &stats)
	if stats.PracticeCount != 1 {
		t.Errorf("count after undo = %d, want 1 (today remains)", stats.PracticeCount)
	}
}

func TestResourceEndpoints(t *testing.T) {
	e, _ := newPracticeAPI(t)
	cookie := signupAndCookie(t, e, "alice")
	id := createSongFor(t, e, cookie)

	rec := jsonReq(e, http.MethodPost, fmt.Sprintf("/api/songs/%d/resources", id),
		`{"url":"https://example.com/tab","label":"tab"}`, cookie)
	if rec.Code != http.StatusCreated {
		t.Fatalf("create resource: %d %s", rec.Code, rec.Body.String())
	}
	var res struct {
		ID uint `json:"id"`
	}
	_ = json.Unmarshal(rec.Body.Bytes(), &res)

	// Validation: URL must be http(s).
	rec = jsonReq(e, http.MethodPost, fmt.Sprintf("/api/songs/%d/resources", id),
		`{"url":"javascript:alert(1)"}`, cookie)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("bad url: %d, want 400", rec.Code)
	}

	rec = jsonReq(e, http.MethodPatch,
		fmt.Sprintf("/api/songs/%d/resources/%d", id, res.ID),
		`{"label":"chords"}`, cookie)
	if rec.Code != http.StatusOK {
		t.Fatalf("update resource: %d %s", rec.Code, rec.Body.String())
	}

	// The resource must belong to the song in the URL.
	other := createSongFor(t, e, cookie)
	rec = jsonReq(e, http.MethodPatch,
		fmt.Sprintf("/api/songs/%d/resources/%d", other, res.ID),
		`{"label":"nope"}`, cookie)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("mismatched song patch: %d, want 404", rec.Code)
	}

	rec = jsonReq(e, http.MethodDelete,
		fmt.Sprintf("/api/songs/%d/resources/%d", id, res.ID), "", cookie)
	if rec.Code != http.StatusNoContent {
		t.Fatalf("delete resource: %d", rec.Code)
	}

	// Other users get 404s.
	bob := signupAndCookie(t, e, "bob")
	rec = jsonReq(e, http.MethodPost, fmt.Sprintf("/api/songs/%d/resources", id),
		`{"url":"https://example.com"}`, bob)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("bob create on alice song: %d, want 404", rec.Code)
	}
}
