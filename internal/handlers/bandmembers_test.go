package handlers

import (
	"fmt"
	"net/http"
	"testing"

	"github.com/labstack/echo/v5"

	appmw "github.com/jwhumphries/bandwidth/internal/middleware"
	"github.com/jwhumphries/bandwidth/internal/model"
)

func newMembersAPI(t *testing.T) (*echo.Echo, *API) {
	t.Helper()
	e, api := newBandsAPI(t)
	g := e.Group("/api/bands", appmw.RequireAuth(api.Repo))
	g.PATCH("/:id/members/:userId", api.SetMemberRole)
	g.DELETE("/:id/members/:userId", api.RemoveMember)
	return e, api
}

func TestMemberManagement(t *testing.T) {
	e, api := newMembersAPI(t)
	alice := signupAndCookie(t, e, "alice") // creator/admin
	bob := signupAndCookie(t, e, "bob")     // member
	_ = signupAndCookie(t, e, "carol")      // member

	band := createBandFor(t, e, alice, "Band")
	bobID := mustUserID(t, api, "bob")
	carolID := mustUserID(t, api, "carol")
	aliceID := mustUserID(t, api, "alice")
	_ = api.Repo.AddMember(band, bobID, model.RoleEditor)
	_ = api.Repo.AddMember(band, carolID, model.RoleEditor)

	memberPath := func(uid uint) string {
		return fmt.Sprintf("/api/bands/%d/members/%d", band, uid)
	}

	// Admin changes a role.
	rec := jsonReq(e, http.MethodPatch, memberPath(bobID), `{"role":"viewer"}`, alice)
	if rec.Code != http.StatusNoContent {
		t.Fatalf("set role: %d %s", rec.Code, rec.Body.String())
	}
	role, _ := api.Repo.MemberRole(band, bobID)
	if role != model.RoleViewer {
		t.Errorf("role = %q", role)
	}

	// Invalid role rejected.
	if rec := jsonReq(e, http.MethodPatch, memberPath(bobID), `{"role":"roadie"}`, alice); rec.Code != http.StatusBadRequest {
		t.Fatalf("bad role: %d, want 400", rec.Code)
	}

	// Creator's role is immutable; creator cannot be removed.
	if rec := jsonReq(e, http.MethodPatch, memberPath(aliceID), `{"role":"viewer"}`, alice); rec.Code != http.StatusBadRequest {
		t.Fatalf("demote creator: %d, want 400", rec.Code)
	}
	if rec := jsonReq(e, http.MethodDelete, memberPath(aliceID), "", alice); rec.Code != http.StatusBadRequest {
		t.Fatalf("remove creator: %d, want 400", rec.Code)
	}

	// Non-admins cannot manage others...
	if rec := jsonReq(e, http.MethodPatch, memberPath(carolID), `{"role":"viewer"}`, bob); rec.Code != http.StatusForbidden {
		t.Fatalf("member sets role: %d, want 403", rec.Code)
	}
	if rec := jsonReq(e, http.MethodDelete, memberPath(carolID), "", bob); rec.Code != http.StatusForbidden {
		t.Fatalf("member removes other: %d, want 403", rec.Code)
	}
	// ...but can remove THEMSELVES (leave).
	if rec := jsonReq(e, http.MethodDelete, memberPath(bobID), "", bob); rec.Code != http.StatusNoContent {
		t.Fatalf("leave: %d", rec.Code)
	}
	if _, err := api.Repo.MemberRole(band, bobID); err == nil {
		t.Error("bob still a member after leaving")
	}

	// Admin removes a member.
	if rec := jsonReq(e, http.MethodDelete, memberPath(carolID), "", alice); rec.Code != http.StatusNoContent {
		t.Fatalf("admin remove: %d", rec.Code)
	}
}
