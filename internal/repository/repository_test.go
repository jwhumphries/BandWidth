package repository

import (
	"path/filepath"
	"testing"
)

// testRepo returns a Repo backed by a fresh in-memory database.
func testRepo(t *testing.T) *Repo {
	t.Helper()
	repo, err := Open(":memory:")
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	return repo
}

func TestOpenMigratesSchema(t *testing.T) {
	repo := testRepo(t)

	for _, table := range []string{"users", "sessions", "backup_codes", "password_resets"} {
		var n int64
		if err := repo.db.Table(table).Count(&n).Error; err != nil {
			t.Errorf("table %s not migrated: %v", table, err)
		}
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
