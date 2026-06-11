package repository

import (
	"testing"
	"time"

	"github.com/jwhumphries/bandwidth/internal/model"
)

func TestBackupCodes(t *testing.T) {
	repo := testRepo(t)
	user, _ := repo.CreateUser("alice", "alice@example.com", "h")

	codes := []string{"AAAA-BBBB", "CCCC-DDDD"}
	if err := repo.ReplaceBackupCodes(user.ID, codes); err != nil {
		t.Fatalf("ReplaceBackupCodes: %v", err)
	}

	if !repo.ConsumeBackupCode(user.ID, "AAAA-BBBB") {
		t.Error("valid code rejected")
	}
	if repo.ConsumeBackupCode(user.ID, "AAAA-BBBB") {
		t.Error("code consumed twice")
	}
	if repo.ConsumeBackupCode(user.ID, "XXXX-YYYY") {
		t.Error("unknown code accepted")
	}

	// Replacing wipes old codes.
	if err := repo.ReplaceBackupCodes(user.ID, []string{"EEEE-FFFF"}); err != nil {
		t.Fatal(err)
	}
	if repo.ConsumeBackupCode(user.ID, "CCCC-DDDD") {
		t.Error("old code survived replacement")
	}

	if err := repo.DeleteBackupCodes(user.ID); err != nil {
		t.Fatalf("DeleteBackupCodes: %v", err)
	}
	if repo.ConsumeBackupCode(user.ID, "EEEE-FFFF") {
		t.Error("code survived deletion")
	}
}

func TestPasswordResetLifecycle(t *testing.T) {
	repo := testRepo(t)
	user, _ := repo.CreateUser("alice", "alice@example.com", "h")

	token, err := repo.CreatePasswordReset(user.ID)
	if err != nil {
		t.Fatalf("CreatePasswordReset: %v", err)
	}

	gotID, err := repo.ConsumePasswordReset(token)
	if err != nil || gotID != user.ID {
		t.Fatalf("ConsumePasswordReset = %d, %v", gotID, err)
	}
	// Single use.
	if _, err := repo.ConsumePasswordReset(token); err == nil {
		t.Error("reset token consumed twice")
	}
	if _, err := repo.ConsumePasswordReset("bogus"); err == nil {
		t.Error("bogus reset token accepted")
	}
}

func TestExpiredPasswordResetRejected(t *testing.T) {
	repo := testRepo(t)
	user, _ := repo.CreateUser("alice", "alice@example.com", "h")
	token, _ := repo.CreatePasswordReset(user.ID)

	repo.db.Model(&model.PasswordReset{}).
		Where("user_id = ?", user.ID).
		Update("expires_at", time.Now().Add(-time.Minute))

	if _, err := repo.ConsumePasswordReset(token); err == nil {
		t.Error("expired reset token accepted")
	}
}
