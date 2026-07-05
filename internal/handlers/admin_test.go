package handlers

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"testing"

	"github.com/labstack/echo/v5"

	appmw "github.com/jwhumphries/bandwidth/internal/middleware"
)

func newAdminAPI(t *testing.T) (*echo.Echo, *API) {
	t.Helper()
	e, api := newTestAPI(t)
	g := e.Group("/api/admin", appmw.RequireAuth(api.Repo), appmw.RequireAdmin(api.IsAdminEmail))
	g.GET("/users", api.AdminUsers)
	g.DELETE("/users/:id", api.AdminDeleteUser)
	g.GET("/bands", api.AdminBands)
	g.DELETE("/bands/:id", api.AdminDeleteBand)
	g.GET("/access-policy", api.AdminGetAccessPolicy)
	g.PUT("/access-policy", api.AdminSetAccessPolicy)
	g.POST("/access-policy/emails", api.AdminAddAllowedEmail)
	g.DELETE("/access-policy/emails/:id", api.AdminRemoveAllowedEmail)
	return e, api
}

func mustAdminUserID(t *testing.T, api *API, username string) uint {
	t.Helper()
	user, err := api.Repo.UserByLogin(username)
	if err != nil {
		t.Fatalf("UserByLogin(%s): %v", username, err)
	}
	return user.ID
}

func TestAdminUsersRequiresAdmin(t *testing.T) {
	e, api := newAdminAPI(t)
	bob := signupAndCookie(t, e, "bob")
	rec := jsonReq(e, http.MethodGet, "/api/admin/users", "", bob)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("non-admin: %d, want 403", rec.Code)
	}

	api.AdminEmails = map[string]bool{"admin@example.com": true}
	admin := signupAndCookie(t, e, "admin")
	rec = jsonReq(e, http.MethodGet, "/api/admin/users", "", admin)
	if rec.Code != http.StatusOK {
		t.Fatalf("admin: %d %s", rec.Code, rec.Body.String())
	}
	var users []struct {
		Username string `json:"username"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &users); err != nil {
		t.Fatal(err)
	}
	if len(users) != 2 {
		t.Fatalf("users = %+v, want 2", users)
	}
}

func TestAdminDeleteUser(t *testing.T) {
	e, api := newAdminAPI(t)
	api.AdminEmails = map[string]bool{"admin@example.com": true}
	admin := signupAndCookie(t, e, "admin")
	signupAndCookie(t, e, "bob")
	bobID := mustAdminUserID(t, api, "bob")
	adminID := mustAdminUserID(t, api, "admin")

	rec := jsonReq(e, http.MethodDelete, fmt.Sprintf("/api/admin/users/%d", adminID), "", admin)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("self-delete: %d, want 400", rec.Code)
	}

	rec = jsonReq(e, http.MethodDelete, "/api/admin/users/abc", "", admin)
	if rec.Code != http.StatusNotFound || !strings.Contains(rec.Body.String(), "user not found") {
		t.Fatalf("delete malformed user id: %d %s, want 404 user not found", rec.Code, rec.Body.String())
	}

	rec = jsonReq(e, http.MethodDelete, "/api/admin/users/9999", "", admin)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("delete missing user: %d, want 404", rec.Code)
	}

	rec = jsonReq(e, http.MethodDelete, fmt.Sprintf("/api/admin/users/%d", bobID), "", admin)
	if rec.Code != http.StatusNoContent {
		t.Fatalf("delete bob: %d %s", rec.Code, rec.Body.String())
	}
	if _, err := api.Repo.UserByID(bobID); err == nil {
		t.Error("bob survived admin delete")
	}
}

func TestAdminBandsAndDelete(t *testing.T) {
	e, api := newAdminAPI(t)
	api.AdminEmails = map[string]bool{"admin@example.com": true}
	admin := signupAndCookie(t, e, "admin")
	adminID := mustAdminUserID(t, api, "admin")
	band, err := api.Repo.CreateBand(adminID, "The Quietones")
	if err != nil {
		t.Fatal(err)
	}

	rec := jsonReq(e, http.MethodGet, "/api/admin/bands", "", admin)
	if rec.Code != http.StatusOK {
		t.Fatalf("list bands: %d %s", rec.Code, rec.Body.String())
	}
	var bands []struct {
		Name string `json:"name"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &bands); err != nil {
		t.Fatal(err)
	}
	if len(bands) != 1 || bands[0].Name != "The Quietones" {
		t.Fatalf("bands = %+v", bands)
	}

	rec = jsonReq(e, http.MethodDelete, "/api/admin/bands/9999", "", admin)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("delete missing band: %d, want 404", rec.Code)
	}

	rec = jsonReq(e, http.MethodDelete, fmt.Sprintf("/api/admin/bands/%d", band.ID), "", admin)
	if rec.Code != http.StatusNoContent {
		t.Fatalf("delete band: %d %s", rec.Code, rec.Body.String())
	}
	if _, err := api.Repo.BandByID(band.ID); err == nil {
		t.Error("band survived admin delete")
	}
}

func TestAdminAccessPolicy(t *testing.T) {
	e, api := newAdminAPI(t)
	api.AdminEmails = map[string]bool{"admin@example.com": true}
	admin := signupAndCookie(t, e, "admin")

	rec := jsonReq(e, http.MethodGet, "/api/admin/access-policy", "", admin)
	var policy struct {
		Enabled       bool `json:"enabled"`
		AllowedEmails []struct {
			ID    uint   `json:"id"`
			Email string `json:"email"`
		} `json:"allowedEmails"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &policy); err != nil {
		t.Fatal(err)
	}
	if policy.Enabled || len(policy.AllowedEmails) != 0 {
		t.Fatalf("initial policy = %+v", policy)
	}

	rec = jsonReq(e, http.MethodPut, "/api/admin/access-policy", `{"enabled":true}`, admin)
	if rec.Code != http.StatusNoContent {
		t.Fatalf("enable: %d %s", rec.Code, rec.Body.String())
	}

	rec = jsonReq(e, http.MethodPost, "/api/admin/access-policy/emails",
		`{"email":"Friend@Example.com"}`, admin)
	if rec.Code != http.StatusCreated {
		t.Fatalf("add email: %d %s", rec.Code, rec.Body.String())
	}
	var created struct {
		ID    uint   `json:"id"`
		Email string `json:"email"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &created); err != nil {
		t.Fatal(err)
	}
	if created.Email != "friend@example.com" {
		t.Errorf("email not normalized: %q", created.Email)
	}

	rec = jsonReq(e, http.MethodPost, "/api/admin/access-policy/emails",
		`{"email":"friend@example.com"}`, admin)
	if rec.Code != http.StatusConflict {
		t.Fatalf("duplicate email: %d, want 409", rec.Code)
	}

	rec = jsonReq(e, http.MethodDelete,
		fmt.Sprintf("/api/admin/access-policy/emails/%d", created.ID), "", admin)
	if rec.Code != http.StatusNoContent {
		t.Fatalf("remove email: %d %s", rec.Code, rec.Body.String())
	}
}
