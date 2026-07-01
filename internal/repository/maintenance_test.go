package repository

import (
	"testing"
	"time"

	"github.com/jwhumphries/bandwidth/internal/auth"
	"github.com/jwhumphries/bandwidth/internal/model"
)

func TestPurgeExpired(t *testing.T) {
	repo := testRepo(t)
	alice, _ := repo.CreateUser("alice", "alice@example.com", "h")
	bob, _ := repo.CreateUser("bob", "bob@example.com", "h")
	carol, _ := repo.CreateUser("carol", "carol@example.com", "h")
	band, _ := repo.CreateBand(alice.ID, "The Quietones")

	// Sessions: one live, one expired.
	liveSession, err := repo.CreateSession(alice.ID)
	if err != nil {
		t.Fatal(err)
	}
	expiredSession := &model.Session{
		TokenHash: auth.HashToken(auth.NewToken()),
		UserID:    alice.ID,
		ExpiresAt: time.Now().Add(-time.Hour),
	}
	if err := repo.db.Create(expiredSession).Error; err != nil {
		t.Fatal(err)
	}

	// Password resets: one live, one used.
	liveReset, err := repo.CreatePasswordReset(alice.ID)
	if err != nil {
		t.Fatal(err)
	}
	usedReset, err := repo.CreatePasswordReset(alice.ID)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := repo.ConsumePasswordReset(usedReset); err != nil {
		t.Fatal(err)
	}

	// Invites: pending direct (bob) and live link survive; declined direct
	// (carol) and expired link are purged.
	pending, err := repo.CreateDirectInvite(band.ID, bob.ID, model.RoleEditor, alice.ID)
	if err != nil {
		t.Fatal(err)
	}
	declined, err := repo.CreateDirectInvite(band.ID, carol.ID, model.RoleViewer, alice.ID)
	if err != nil {
		t.Fatal(err)
	}
	if err := repo.DeclineInvite(declined.ID, carol.ID); err != nil {
		t.Fatal(err)
	}
	liveLink, linkToken, err := repo.CreateLinkInvite(band.ID, model.RoleViewer, alice.ID)
	if err != nil {
		t.Fatal(err)
	}
	expiredLink, _, err := repo.CreateLinkInvite(band.ID, model.RoleViewer, alice.ID)
	if err != nil {
		t.Fatal(err)
	}
	err = repo.db.Model(&model.BandInvite{}).Where("id = ?", expiredLink.ID).
		Update("expires_at", time.Now().Add(-time.Minute)).Error
	if err != nil {
		t.Fatal(err)
	}

	if err := repo.PurgeExpired(); err != nil {
		t.Fatalf("PurgeExpired: %v", err)
	}

	if _, err := repo.SessionUser(liveSession); err != nil {
		t.Errorf("live session purged: %v", err)
	}
	var sessions int64
	repo.db.Model(&model.Session{}).Count(&sessions)
	if sessions != 1 {
		t.Errorf("sessions after purge = %d, want 1", sessions)
	}

	var resets int64
	repo.db.Model(&model.PasswordReset{}).Count(&resets)
	if resets != 1 {
		t.Errorf("password resets after purge = %d, want 1", resets)
	}
	if _, err := repo.ConsumePasswordReset(liveReset); err != nil {
		t.Errorf("live reset purged: %v", err)
	}

	var inviteIDs []uint
	repo.db.Model(&model.BandInvite{}).Order("id").Pluck("id", &inviteIDs)
	if len(inviteIDs) != 2 || inviteIDs[0] != pending.ID || inviteIDs[1] != liveLink.ID {
		t.Errorf("invites after purge = %v, want [%d %d]", inviteIDs, pending.ID, liveLink.ID)
	}
	if _, err := repo.BandNameByLinkToken(linkToken); err != nil {
		t.Errorf("live link token purged: %v", err)
	}
}
