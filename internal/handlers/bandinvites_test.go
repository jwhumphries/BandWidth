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

func newInvitesAPI(t *testing.T) (*echo.Echo, *API) {
	t.Helper()
	e, api := newBandsAPI(t)
	g := e.Group("/api/bands", appmw.RequireAuth(api.Repo))
	g.GET("/:id/invites", api.BandInvites)
	g.POST("/:id/invites", api.CreateInvite)
	g.DELETE("/:id/invites/:inviteId", api.RevokeInvite)
	inv := e.Group("/api/invites", appmw.RequireAuth(api.Repo))
	inv.GET("", api.MyInvites)
	inv.POST("/:id/accept", api.AcceptInvite)
	inv.POST("/:id/decline", api.DeclineInvite)
	inv.POST("/link", api.JoinByLink)
	return e, api
}

func TestDirectInviteFlow(t *testing.T) {
	e, api := newInvitesAPI(t)
	alice := signupAndCookie(t, e, "alice")
	bob := signupAndCookie(t, e, "bob")
	band := createBandFor(t, e, alice, "Band")

	// Invite bob by username, default role editor.
	rec := jsonReq(e, http.MethodPost, fmt.Sprintf("/api/bands/%d/invites", band),
		`{"username":"bob"}`, alice)
	if rec.Code != http.StatusCreated {
		t.Fatalf("invite: %d %s", rec.Code, rec.Body.String())
	}

	// Unknown user → 404; existing member → 409; duplicate pending → 409.
	if rec := jsonReq(e, http.MethodPost, fmt.Sprintf("/api/bands/%d/invites", band),
		`{"username":"nobody"}`, alice); rec.Code != http.StatusNotFound {
		t.Fatalf("unknown user: %d, want 404", rec.Code)
	}
	if rec := jsonReq(e, http.MethodPost, fmt.Sprintf("/api/bands/%d/invites", band),
		`{"username":"alice"}`, alice); rec.Code != http.StatusConflict {
		t.Fatalf("invite member: %d, want 409", rec.Code)
	}
	if rec := jsonReq(e, http.MethodPost, fmt.Sprintf("/api/bands/%d/invites", band),
		`{"username":"bob"}`, alice); rec.Code != http.StatusConflict {
		t.Fatalf("duplicate invite: %d, want 409", rec.Code)
	}

	// Non-admins cannot invite.
	if rec := jsonReq(e, http.MethodPost, fmt.Sprintf("/api/bands/%d/invites", band),
		`{"username":"carol"}`, bob); rec.Code != http.StatusNotFound {
		t.Fatalf("non-member invites: %d, want 404", rec.Code)
	}

	// Bob sees and accepts the invite.
	rec = jsonReq(e, http.MethodGet, "/api/invites", "", bob)
	var pending []struct {
		ID       uint   `json:"id"`
		BandName string `json:"bandName"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &pending); err != nil || len(pending) != 1 {
		t.Fatalf("my invites: %s (%v)", rec.Body.String(), err)
	}
	rec = jsonReq(e, http.MethodPost, fmt.Sprintf("/api/invites/%d/accept", pending[0].ID), "", bob)
	if rec.Code != http.StatusOK {
		t.Fatalf("accept: %d %s", rec.Code, rec.Body.String())
	}
	role, err := api.Repo.MemberRole(band, mustUserID(t, api, "bob"))
	if err != nil || role != model.RoleEditor {
		t.Fatalf("bob role = %q, %v", role, err)
	}

	// Accepting someone else's invite 404s.
	carol := signupAndCookie(t, e, "carol")
	rec = jsonReq(e, http.MethodPost, fmt.Sprintf("/api/bands/%d/invites", band),
		`{"username":"carol","role":"viewer"}`, alice)
	if rec.Code != http.StatusCreated {
		t.Fatal(rec.Code)
	}
	var carolInvite struct {
		ID uint `json:"id"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &carolInvite); err != nil {
		t.Fatalf("carol invite body: %s (%v)", rec.Body.String(), err)
	}
	if rec := jsonReq(e, http.MethodPost, fmt.Sprintf("/api/invites/%d/accept", carolInvite.ID), "", bob); rec.Code != http.StatusNotFound {
		t.Fatalf("accept other's invite: %d, want 404", rec.Code)
	}
	// Decline.
	if rec := jsonReq(e, http.MethodPost, fmt.Sprintf("/api/invites/%d/decline", carolInvite.ID), "", carol); rec.Code != http.StatusNoContent {
		t.Fatalf("decline: %d", rec.Code)
	}
}

func TestLinkInviteFlow(t *testing.T) {
	e, api := newInvitesAPI(t)
	alice := signupAndCookie(t, e, "alice")
	bob := signupAndCookie(t, e, "bob")
	band := createBandFor(t, e, alice, "Band")

	// Create a link (admin only) with a role.
	rec := jsonReq(e, http.MethodPost, fmt.Sprintf("/api/bands/%d/invites", band),
		`{"link":true,"role":"viewer"}`, alice)
	if rec.Code != http.StatusCreated {
		t.Fatalf("create link: %d %s", rec.Code, rec.Body.String())
	}
	var link struct {
		ID    uint   `json:"id"`
		Token string `json:"token"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &link); err != nil || link.Token == "" {
		t.Fatalf("link body: %s (%v)", rec.Body.String(), err)
	}

	// Join via the link.
	rec = jsonReq(e, http.MethodPost, "/api/invites/link", fmt.Sprintf(`{"token":%q}`, link.Token), bob)
	if rec.Code != http.StatusOK {
		t.Fatalf("join: %d %s", rec.Code, rec.Body.String())
	}
	role, _ := api.Repo.MemberRole(band, mustUserID(t, api, "bob"))
	if role != model.RoleViewer {
		t.Errorf("joined role = %q", role)
	}

	// Bad token 404s.
	if rec := jsonReq(e, http.MethodPost, "/api/invites/link", `{"token":"bogus"}`, bob); rec.Code != http.StatusNotFound {
		t.Fatalf("bogus token: %d, want 404", rec.Code)
	}

	// Admin lists and revokes; revoked link stops working.
	rec = jsonReq(e, http.MethodGet, fmt.Sprintf("/api/bands/%d/invites", band), "", alice)
	var invites []struct {
		ID     uint `json:"id"`
		IsLink bool `json:"isLink"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &invites); err != nil || len(invites) != 1 || !invites[0].IsLink {
		t.Fatalf("band invites: %s (%v)", rec.Body.String(), err)
	}
	rec = jsonReq(e, http.MethodDelete,
		fmt.Sprintf("/api/bands/%d/invites/%d", band, invites[0].ID), "", alice)
	if rec.Code != http.StatusNoContent {
		t.Fatalf("revoke: %d", rec.Code)
	}
	carol := signupAndCookie(t, e, "carol")
	if rec := jsonReq(e, http.MethodPost, "/api/invites/link", fmt.Sprintf(`{"token":%q}`, link.Token), carol); rec.Code != http.StatusNotFound {
		t.Fatalf("revoked link join: %d, want 404", rec.Code)
	}
}
