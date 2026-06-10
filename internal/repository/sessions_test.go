package repository

import (
	"testing"
	"time"

	"github.com/jwhumphries/bandwidth/internal/model"
)

func TestSessionLifecycle(t *testing.T) {
	repo := testRepo(t)
	user, _ := repo.CreateUser("alice", "alice@example.com", "h")

	token, err := repo.CreateSession(user.ID)
	if err != nil {
		t.Fatalf("CreateSession: %v", err)
	}

	got, err := repo.SessionUser(token)
	if err != nil || got.ID != user.ID {
		t.Fatalf("SessionUser = %v, %v", got, err)
	}
	if _, err := repo.SessionUser("bogus-token"); err == nil {
		t.Error("bogus token accepted")
	}

	if err := repo.DeleteSession(token); err != nil {
		t.Fatalf("DeleteSession: %v", err)
	}
	if _, err := repo.SessionUser(token); err == nil {
		t.Error("deleted session still valid")
	}
}

func TestExpiredSessionRejected(t *testing.T) {
	repo := testRepo(t)
	user, _ := repo.CreateUser("alice", "alice@example.com", "h")
	token, _ := repo.CreateSession(user.ID)

	// Force the session into the past.
	repo.db.Model(&model.Session{}).
		Where("user_id = ?", user.ID).
		Update("expires_at", time.Now().Add(-time.Minute))

	if _, err := repo.SessionUser(token); err == nil {
		t.Error("expired session accepted")
	}
}

func TestDeleteUserSessions(t *testing.T) {
	repo := testRepo(t)
	user, _ := repo.CreateUser("alice", "alice@example.com", "h")
	t1, _ := repo.CreateSession(user.ID)
	t2, _ := repo.CreateSession(user.ID)

	if err := repo.DeleteUserSessions(user.ID); err != nil {
		t.Fatalf("DeleteUserSessions: %v", err)
	}
	if _, err := repo.SessionUser(t1); err == nil {
		t.Error("session 1 survived")
	}
	if _, err := repo.SessionUser(t2); err == nil {
		t.Error("session 2 survived")
	}
}
