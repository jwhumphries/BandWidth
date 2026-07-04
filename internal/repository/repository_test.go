package repository

import (
	"path/filepath"
	"testing"

	"github.com/jwhumphries/bandwidth/internal/model"
)

// testRepo returns a Repo backed by a fresh in-memory database.
func testRepo(t *testing.T) *Repo {
	t.Helper()
	repo, err := Open(":memory:")
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	t.Cleanup(func() {
		if err := repo.Close(); err != nil {
			t.Errorf("closing test repo: %v", err)
		}
	})
	return repo
}

func TestOpenMigratesSchema(t *testing.T) {
	repo := testRepo(t)

	for _, table := range []string{
		"users", "sessions", "backup_codes", "password_resets",
		"songs", "song_annotations", "resources", "practice_events",
		"folders", "folder_entries",
		"bands", "band_members", "band_invites",
		"access_policies", "allowed_emails",
	} {
		var n int64
		if err := repo.db.Table(table).Count(&n).Error; err != nil {
			t.Errorf("table %s not migrated: %v", table, err)
		}
	}
}

func TestSubjectUniquenessEnforced(t *testing.T) {
	repo := testRepo(t)
	uid := uint(1)

	a1 := model.SongAnnotation{SongID: 1, UserID: &uid, Status: model.StatusLearning}
	a2 := model.SongAnnotation{SongID: 1, UserID: &uid, Status: model.StatusLearned}
	if err := repo.db.Create(&a1).Error; err != nil {
		t.Fatalf("first annotation: %v", err)
	}
	if err := repo.db.Create(&a2).Error; err == nil {
		t.Error("duplicate (song, user) annotation allowed")
	}

	p1 := model.PracticeEvent{SongID: 1, UserID: &uid, Date: "2026-06-10"}
	p2 := model.PracticeEvent{SongID: 1, UserID: &uid, Date: "2026-06-10"}
	if err := repo.db.Create(&p1).Error; err != nil {
		t.Fatalf("first practice event: %v", err)
	}
	if err := repo.db.Create(&p2).Error; err == nil {
		t.Error("duplicate (song, user, date) practice event allowed")
	}
	// A different date is fine.
	p3 := model.PracticeEvent{SongID: 1, UserID: &uid, Date: "2026-06-11"}
	if err := repo.db.Create(&p3).Error; err != nil {
		t.Errorf("distinct date rejected: %v", err)
	}
}

func TestOpenAppliesPragmas(t *testing.T) {
	repo := testRepo(t)

	var fk int
	if err := repo.db.Raw("PRAGMA foreign_keys").Scan(&fk).Error; err != nil || fk != 1 {
		t.Errorf("foreign_keys = %d (err %v), want 1", fk, err)
	}

	fileRepo, err := Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatalf("Open(file): %v", err)
	}
	defer func() {
		if err := fileRepo.Close(); err != nil {
			t.Errorf("Close: %v", err)
		}
	}()
	var mode string
	if err := fileRepo.db.Raw("PRAGMA journal_mode").Scan(&mode).Error; err != nil || mode != "wal" {
		t.Errorf("journal_mode = %q (err %v), want wal", mode, err)
	}
}
