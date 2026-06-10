package repository

import "testing"

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
