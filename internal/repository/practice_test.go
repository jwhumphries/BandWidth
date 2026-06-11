package repository

import "testing"

func TestLogPracticeIdempotentPerDay(t *testing.T) {
	repo := testRepo(t)
	user, _ := repo.CreateUser("alice", "alice@example.com", "h")
	song, _ := repo.CreateSong(user.ID, "Wonderwall", "Oasis")

	if err := repo.LogPractice(song.ID, user.ID, "2026-06-10"); err != nil {
		t.Fatalf("LogPractice: %v", err)
	}
	// Same day again: no error, no duplicate.
	if err := repo.LogPractice(song.ID, user.ID, "2026-06-10"); err != nil {
		t.Fatalf("LogPractice(dup): %v", err)
	}
	if err := repo.LogPractice(song.ID, user.ID, "2026-06-11"); err != nil {
		t.Fatalf("LogPractice(day 2): %v", err)
	}

	last, count, err := repo.PracticeStats(song.ID, user.ID)
	if err != nil {
		t.Fatalf("PracticeStats: %v", err)
	}
	if last != "2026-06-11" || count != 2 {
		t.Fatalf("stats = %q/%d, want 2026-06-11/2", last, count)
	}
}

func TestDeletePractice(t *testing.T) {
	repo := testRepo(t)
	user, _ := repo.CreateUser("alice", "alice@example.com", "h")
	song, _ := repo.CreateSong(user.ID, "Wonderwall", "Oasis")
	_ = repo.LogPractice(song.ID, user.ID, "2026-06-10")
	_ = repo.LogPractice(song.ID, user.ID, "2026-06-11")

	if err := repo.DeletePractice(song.ID, user.ID, "2026-06-11"); err != nil {
		t.Fatalf("DeletePractice: %v", err)
	}
	last, count, _ := repo.PracticeStats(song.ID, user.ID)
	if last != "2026-06-10" || count != 1 {
		t.Fatalf("stats after delete = %q/%d", last, count)
	}
	// Deleting a date that isn't logged is a no-op.
	if err := repo.DeletePractice(song.ID, user.ID, "2026-01-01"); err != nil {
		t.Fatalf("DeletePractice(absent): %v", err)
	}
}

func TestPracticeStatsEmpty(t *testing.T) {
	repo := testRepo(t)
	user, _ := repo.CreateUser("alice", "alice@example.com", "h")
	song, _ := repo.CreateSong(user.ID, "Wonderwall", "Oasis")

	last, count, err := repo.PracticeStats(song.ID, user.ID)
	if err != nil || last != "" || count != 0 {
		t.Fatalf("empty stats = %q/%d (%v)", last, count, err)
	}
}
