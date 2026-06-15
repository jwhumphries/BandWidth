package repository

import (
	"errors"
	"testing"
	"time"

	"github.com/jwhumphries/bandwidth/internal/model"
)

func inviteFixture(t *testing.T) (*Repo, *model.User, *model.User, *model.Band) {
	t.Helper()
	repo := testRepo(t)
	alice, _ := repo.CreateUser("alice", "alice@example.com", "h")
	bob, _ := repo.CreateUser("bob", "bob@example.com", "h")
	band, _ := repo.CreateBand(alice.ID, "Band")
	return repo, alice, bob, band
}

func TestDirectInviteLifecycle(t *testing.T) {
	repo, alice, bob, band := inviteFixture(t)

	invite, err := repo.CreateDirectInvite(band.ID, bob.ID, model.RoleEditor, alice.ID)
	if err != nil {
		t.Fatalf("CreateDirectInvite: %v", err)
	}

	// Member sees their pending invite with the band name.
	pending, err := repo.PendingInvitesForUser(bob.ID)
	if err != nil || len(pending) != 1 {
		t.Fatalf("PendingInvitesForUser: %v (%v)", pending, err)
	}
	if pending[0].BandName != "Band" || pending[0].Role != model.RoleEditor {
		t.Errorf("pending = %+v", pending[0])
	}

	// Duplicate pending invite rejected; inviting an existing member rejected.
	if _, err := repo.CreateDirectInvite(band.ID, bob.ID, model.RoleEditor, alice.ID); !errors.Is(err, ErrInvitePending) {
		t.Errorf("duplicate invite: %v", err)
	}
	if _, err := repo.CreateDirectInvite(band.ID, alice.ID, model.RoleEditor, alice.ID); !errors.Is(err, ErrAlreadyMember) {
		t.Errorf("invite existing member: %v", err)
	}

	// Accept joins with the invite's role and is single-use.
	bandID, err := repo.AcceptInvite(invite.ID, bob.ID)
	if err != nil || bandID != band.ID {
		t.Fatalf("AcceptInvite: %d, %v", bandID, err)
	}
	role, _ := repo.MemberRole(band.ID, bob.ID)
	if role != model.RoleEditor {
		t.Errorf("role after accept = %q", role)
	}
	if _, err := repo.AcceptInvite(invite.ID, bob.ID); err == nil {
		t.Error("invite accepted twice")
	}
	if pending, _ := repo.PendingInvitesForUser(bob.ID); len(pending) != 0 {
		t.Errorf("pending after accept = %v", pending)
	}
}

func TestAcceptGuards(t *testing.T) {
	repo, alice, bob, band := inviteFixture(t)
	carol, _ := repo.CreateUser("carol", "carol@example.com", "h")
	invite, _ := repo.CreateDirectInvite(band.ID, bob.ID, model.RoleViewer, alice.ID)

	// Only the invited user can accept.
	if _, err := repo.AcceptInvite(invite.ID, carol.ID); err == nil {
		t.Error("wrong user accepted invite")
	}

	// Declined invites cannot be accepted.
	if err := repo.DeclineInvite(invite.ID, bob.ID); err != nil {
		t.Fatalf("DeclineInvite: %v", err)
	}
	if _, err := repo.AcceptInvite(invite.ID, bob.ID); err == nil {
		t.Error("declined invite accepted")
	}

	// Expired invites cannot be accepted.
	invite2, _ := repo.CreateDirectInvite(band.ID, bob.ID, model.RoleViewer, alice.ID)
	repo.db.Model(&model.BandInvite{}).Where("id = ?", invite2.ID).
		Update("expires_at", time.Now().Add(-time.Minute))
	if _, err := repo.AcceptInvite(invite2.ID, bob.ID); err == nil {
		t.Error("expired invite accepted")
	}
}

func TestLinkInviteLifecycle(t *testing.T) {
	repo, alice, bob, band := inviteFixture(t)
	carol, _ := repo.CreateUser("carol", "carol@example.com", "h")

	_, token, err := repo.CreateLinkInvite(band.ID, model.RoleViewer, alice.ID)
	if err != nil || token == "" {
		t.Fatalf("CreateLinkInvite: %q, %v", token, err)
	}

	// Multi-use: two different users join via the same link.
	if bandID, err := repo.JoinByLink(token, bob.ID); err != nil || bandID != band.ID {
		t.Fatalf("JoinByLink(bob): %d, %v", bandID, err)
	}
	if bandID, err := repo.JoinByLink(token, carol.ID); err != nil || bandID != band.ID {
		t.Fatalf("JoinByLink(carol): %d, %v", bandID, err)
	}
	role, _ := repo.MemberRole(band.ID, carol.ID)
	if role != model.RoleViewer {
		t.Errorf("link role = %q", role)
	}

	// Joining again is idempotent.
	if bandID, err := repo.JoinByLink(token, bob.ID); err != nil || bandID != band.ID {
		t.Errorf("re-join: %d, %v", bandID, err)
	}

	// Bogus tokens rejected.
	if _, err := repo.JoinByLink("bogus", bob.ID); err == nil {
		t.Error("bogus token joined")
	}

	// Revoked links stop working; revoke is band-scoped.
	invites, err := repo.InvitesForBand(band.ID)
	if err != nil || len(invites) != 1 {
		t.Fatalf("InvitesForBand: %v (%v)", invites, err)
	}
	if !invites[0].IsLink {
		t.Errorf("invite not a link: %+v", invites[0])
	}
	if err := repo.RevokeInvite(invites[0].ID, 9999); err == nil {
		t.Error("revoke with wrong band succeeded")
	}
	if err := repo.RevokeInvite(invites[0].ID, band.ID); err != nil {
		t.Fatalf("RevokeInvite: %v", err)
	}
	dave, _ := repo.CreateUser("dave", "dave@example.com", "h")
	if _, err := repo.JoinByLink(token, dave.ID); err == nil {
		t.Error("revoked link joined")
	}
}
