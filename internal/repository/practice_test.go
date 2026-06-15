package repository

import (
	"testing"

	"github.com/jwhumphries/bandwidth/internal/model"
)

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

func TestBandPracticeIsolatedFromUser(t *testing.T) {
	repo := testRepo(t)
	user, _ := repo.CreateUser("alice", "alice@example.com", "h")
	band, _ := repo.CreateBand(user.ID, "Band")
	song := &model.Song{Title: "Wonderwall", Artist: "Oasis", OwnerBandID: &band.ID}
	if err := repo.db.Create(song).Error; err != nil {
		t.Fatal(err)
	}

	// A band rehearsal and a personal practice on the SAME song/day are
	// distinct rows in distinct layers.
	if err := repo.LogBandPractice(song.ID, band.ID, "2026-06-10"); err != nil {
		t.Fatalf("LogBandPractice: %v", err)
	}
	if err := repo.LogPractice(song.ID, user.ID, "2026-06-10"); err != nil {
		t.Fatalf("LogPractice: %v", err)
	}
	// Logging the same band day twice is idempotent.
	if err := repo.LogBandPractice(song.ID, band.ID, "2026-06-10"); err != nil {
		t.Fatal(err)
	}

	bandLast, bandCount, _ := repo.BandPracticeStats(song.ID, band.ID)
	if bandLast != "2026-06-10" || bandCount != 1 {
		t.Errorf("band stats = %q/%d, want 2026-06-10/1", bandLast, bandCount)
	}
	userLast, userCount, _ := repo.PracticeStats(song.ID, user.ID)
	if userLast != "2026-06-10" || userCount != 1 {
		t.Errorf("user stats = %q/%d (band rows must not leak into user layer)", userLast, userCount)
	}

	if err := repo.DeleteBandPractice(song.ID, band.ID, "2026-06-10"); err != nil {
		t.Fatalf("DeleteBandPractice: %v", err)
	}
	_, bandCount, _ = repo.BandPracticeStats(song.ID, band.ID)
	if bandCount != 0 {
		t.Errorf("band count after delete = %d", bandCount)
	}
	// The user's row is untouched by deleting the band's.
	_, userCount, _ = repo.PracticeStats(song.ID, user.ID)
	if userCount != 1 {
		t.Errorf("user count after band delete = %d, want 1", userCount)
	}
}
