package handlers

import (
	"encoding/json"
	"fmt"
	"net/http"
	"testing"

	"github.com/labstack/echo/v5"

	appmw "github.com/jwhumphries/bandwidth/internal/middleware"
)

// newSongsAPI registers the personal-song routes on a test server.
func newSongsAPI(t *testing.T) (*echo.Echo, *API) {
	t.Helper()
	e, api := newTestAPI(t)
	g := e.Group("/api/songs", appmw.RequireAuth(api.Repo))
	g.GET("", api.Songs)
	g.POST("", api.CreateSong)
	g.GET("/:id", api.Song)
	g.PATCH("/:id", api.UpdateSong)
	g.DELETE("/:id", api.DeleteSong)
	return e, api
}

func TestSongCRUD(t *testing.T) {
	e, _ := newSongsAPI(t)
	cookie := signupAndCookie(t, e, "alice")

	// Create.
	rec := jsonReq(e, http.MethodPost, "/api/songs",
		`{"title":"Wonderwall","artist":"Oasis"}`, cookie)
	if rec.Code != http.StatusCreated {
		t.Fatalf("create: %d %s", rec.Code, rec.Body.String())
	}
	var created struct {
		ID     uint   `json:"id"`
		Title  string `json:"title"`
		Status string `json:"status"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &created); err != nil || created.ID == 0 {
		t.Fatalf("create body: %s (%v)", rec.Body.String(), err)
	}
	if created.Status != "not_learned" {
		t.Errorf("default status = %q", created.Status)
	}

	// Validation.
	if rec := jsonReq(e, http.MethodPost, "/api/songs", `{"title":"  "}`, cookie); rec.Code != http.StatusBadRequest {
		t.Fatalf("blank title: %d, want 400", rec.Code)
	}

	// List.
	rec = jsonReq(e, http.MethodGet, "/api/songs", "", cookie)
	if rec.Code != http.StatusOK {
		t.Fatalf("list: %d", rec.Code)
	}
	var list []map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &list); err != nil || len(list) != 1 {
		t.Fatalf("list body: %s (%v)", rec.Body.String(), err)
	}

	// Detail.
	rec = jsonReq(e, http.MethodGet, fmt.Sprintf("/api/songs/%d", created.ID), "", cookie)
	if rec.Code != http.StatusOK {
		t.Fatalf("detail: %d %s", rec.Code, rec.Body.String())
	}
	var detail struct {
		Title         string           `json:"title"`
		Notes         string           `json:"notes"`
		Resources     []map[string]any `json:"resources"`
		PracticeCount int              `json:"practiceCount"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &detail); err != nil {
		t.Fatal(err)
	}
	if detail.Title != "Wonderwall" || detail.Resources == nil || detail.PracticeCount != 0 {
		t.Errorf("detail = %+v", detail)
	}

	// Update identity + annotation in one PATCH.
	rec = jsonReq(e, http.MethodPatch, fmt.Sprintf("/api/songs/%d", created.ID),
		`{"artist":"Oasis (1995)","status":"learning","notes":"capo 2"}`, cookie)
	if rec.Code != http.StatusOK {
		t.Fatalf("update: %d %s", rec.Code, rec.Body.String())
	}
	rec = jsonReq(e, http.MethodGet, fmt.Sprintf("/api/songs/%d", created.ID), "", cookie)
	var after struct {
		Artist string `json:"artist"`
		Status string `json:"status"`
		Notes  string `json:"notes"`
	}
	_ = json.Unmarshal(rec.Body.Bytes(), &after)
	if after.Artist != "Oasis (1995)" || after.Status != "learning" || after.Notes != "capo 2" {
		t.Errorf("after update = %+v", after)
	}

	// Bad status value.
	rec = jsonReq(e, http.MethodPatch, fmt.Sprintf("/api/songs/%d", created.ID),
		`{"status":"shredded"}`, cookie)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("bad status: %d, want 400", rec.Code)
	}

	// Delete.
	rec = jsonReq(e, http.MethodDelete, fmt.Sprintf("/api/songs/%d", created.ID), "", cookie)
	if rec.Code != http.StatusNoContent {
		t.Fatalf("delete: %d", rec.Code)
	}
	rec = jsonReq(e, http.MethodGet, fmt.Sprintf("/api/songs/%d", created.ID), "", cookie)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("detail after delete: %d, want 404", rec.Code)
	}
}

func TestSongIsolationBetweenUsers(t *testing.T) {
	e, _ := newSongsAPI(t)
	alice := signupAndCookie(t, e, "alice")
	bob := signupAndCookie(t, e, "bob")

	rec := jsonReq(e, http.MethodPost, "/api/songs", `{"title":"Mine"}`, alice)
	var created struct {
		ID uint `json:"id"`
	}
	_ = json.Unmarshal(rec.Body.Bytes(), &created)

	for _, tc := range []struct {
		method, path, body string
	}{
		{http.MethodGet, fmt.Sprintf("/api/songs/%d", created.ID), ""},
		{http.MethodPatch, fmt.Sprintf("/api/songs/%d", created.ID), `{"title":"Stolen"}`},
		{http.MethodDelete, fmt.Sprintf("/api/songs/%d", created.ID), ""},
	} {
		if rec := jsonReq(e, tc.method, tc.path, tc.body, bob); rec.Code != http.StatusNotFound {
			t.Errorf("%s %s as bob: %d, want 404", tc.method, tc.path, rec.Code)
		}
	}
}
