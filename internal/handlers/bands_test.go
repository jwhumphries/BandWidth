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

func newBandsAPI(t *testing.T) (*echo.Echo, *API) {
	t.Helper()
	e, api := newTestAPI(t)
	g := e.Group("/api/bands", appmw.RequireAuth(api.Repo))
	g.GET("", api.Bands)
	g.POST("", api.CreateBand)
	g.GET("/:id", api.Band)
	g.PATCH("/:id", api.RenameBand)
	g.DELETE("/:id", api.DeleteBand)
	return e, api
}

// createBandFor creates a band via the API and returns its id.
func createBandFor(t *testing.T, e *echo.Echo, cookie *http.Cookie, name string) uint {
	t.Helper()
	rec := jsonReq(e, http.MethodPost, "/api/bands",
		fmt.Sprintf(`{"name":%q}`, name), cookie)
	if rec.Code != http.StatusCreated {
		t.Fatalf("create band: %d %s", rec.Code, rec.Body.String())
	}
	var created struct {
		ID uint `json:"id"`
	}
	_ = json.Unmarshal(rec.Body.Bytes(), &created)
	return created.ID
}

func TestBandCRUD(t *testing.T) {
	e, api := newBandsAPI(t)
	alice := signupAndCookie(t, e, "alice")
	bob := signupAndCookie(t, e, "bob")

	id := createBandFor(t, e, alice, "The Quietones")

	// Blank name rejected.
	if rec := jsonReq(e, http.MethodPost, "/api/bands", `{"name":"  "}`, alice); rec.Code != http.StatusBadRequest {
		t.Fatalf("blank name: %d, want 400", rec.Code)
	}

	// List shows role + member count.
	rec := jsonReq(e, http.MethodGet, "/api/bands", "", alice)
	var list []struct {
		Name        string `json:"name"`
		Role        string `json:"role"`
		MemberCount int    `json:"memberCount"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &list); err != nil || len(list) != 1 {
		t.Fatalf("bands list: %s (%v)", rec.Body.String(), err)
	}
	if list[0].Role != "admin" || list[0].MemberCount != 1 {
		t.Errorf("summary = %+v", list[0])
	}

	// Detail shows members and my role.
	rec = jsonReq(e, http.MethodGet, fmt.Sprintf("/api/bands/%d", id), "", alice)
	if rec.Code != http.StatusOK {
		t.Fatalf("detail: %d %s", rec.Code, rec.Body.String())
	}
	var detail struct {
		Name      string `json:"name"`
		MyRole    string `json:"myRole"`
		CreatorID uint   `json:"creatorId"`
		Members   []struct {
			Username string `json:"username"`
			Role     string `json:"role"`
		} `json:"members"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &detail); err != nil {
		t.Fatal(err)
	}
	if detail.MyRole != "admin" || len(detail.Members) != 1 || detail.Members[0].Username != "alice" {
		t.Errorf("detail = %+v", detail)
	}

	// Non-members get 404s everywhere.
	for _, tc := range []struct{ method, path, body string }{
		{http.MethodGet, fmt.Sprintf("/api/bands/%d", id), ""},
		{http.MethodPatch, fmt.Sprintf("/api/bands/%d", id), `{"name":"X"}`},
		{http.MethodDelete, fmt.Sprintf("/api/bands/%d", id), ""},
	} {
		if rec := jsonReq(e, tc.method, tc.path, tc.body, bob); rec.Code != http.StatusNotFound {
			t.Errorf("%s %s as bob: %d, want 404", tc.method, tc.path, rec.Code)
		}
	}

	// Rename (admin) works.
	rec = jsonReq(e, http.MethodPatch, fmt.Sprintf("/api/bands/%d", id), `{"name":"Loudones"}`, alice)
	if rec.Code != http.StatusNoContent {
		t.Fatalf("rename: %d %s", rec.Code, rec.Body.String())
	}

	// Non-admin members get 403 on admin actions.
	if err := api.Repo.AddMember(id, mustUserID(t, api, "bob"), model.RoleEditor); err != nil {
		t.Fatal(err)
	}
	rec = jsonReq(e, http.MethodPatch, fmt.Sprintf("/api/bands/%d", id), `{"name":"Nope"}`, bob)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("editor rename: %d, want 403", rec.Code)
	}
	// Non-creator cannot delete, even an admin.
	_ = api.Repo.SetMemberRole(id, mustUserID(t, api, "bob"), model.RoleAdmin)
	rec = jsonReq(e, http.MethodDelete, fmt.Sprintf("/api/bands/%d", id), "", bob)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("non-creator delete: %d, want 403", rec.Code)
	}

	// Creator deletes.
	rec = jsonReq(e, http.MethodDelete, fmt.Sprintf("/api/bands/%d", id), "", alice)
	if rec.Code != http.StatusNoContent {
		t.Fatalf("delete: %d", rec.Code)
	}
}

// mustUserID looks a user up by username through the repo.
func mustUserID(t *testing.T, api *API, username string) uint {
	t.Helper()
	user, err := api.Repo.UserByLogin(username)
	if err != nil {
		t.Fatalf("UserByLogin(%s): %v", username, err)
	}
	return user.ID
}
