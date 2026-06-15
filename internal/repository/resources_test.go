package repository

import (
	"testing"

	"github.com/jwhumphries/bandwidth/internal/model"
)

func TestResourceLifecycle(t *testing.T) {
	repo := testRepo(t)
	user, _ := repo.CreateUser("alice", "alice@example.com", "h")
	song, _ := repo.CreateSong(user.ID, "Wonderwall", "Oasis")

	first, err := repo.CreateResource(song.ID, user.ID, "https://example.com/tab", "tab")
	if err != nil {
		t.Fatalf("CreateResource: %v", err)
	}
	second, _ := repo.CreateResource(song.ID, user.ID, "https://example.com/video", "video")
	if second.Position <= first.Position {
		t.Errorf("positions not appended: %d then %d", first.Position, second.Position)
	}

	list, err := repo.ResourcesForSongUser(song.ID, user.ID)
	if err != nil || len(list) != 2 {
		t.Fatalf("ResourcesForSongUser: %d (%v)", len(list), err)
	}
	if list[0].ID != first.ID {
		t.Error("resources not ordered by position")
	}

	url := "https://example.com/tab2"
	updated, err := repo.UpdateResource(first.ID, song.ID, user.ID, &url, nil)
	if err != nil || updated.URL != url || updated.Label != "tab" {
		t.Fatalf("UpdateResource: %+v (%v)", updated, err)
	}

	if err := repo.DeleteResource(first.ID, song.ID, user.ID); err != nil {
		t.Fatalf("DeleteResource: %v", err)
	}
	list, _ = repo.ResourcesForSongUser(song.ID, user.ID)
	if len(list) != 1 {
		t.Fatalf("resources after delete = %d", len(list))
	}
}

func TestResourceSubjectIsolation(t *testing.T) {
	repo := testRepo(t)
	alice, _ := repo.CreateUser("alice", "alice@example.com", "h")
	bob, _ := repo.CreateUser("bob", "bob@example.com", "h")
	song, _ := repo.CreateSong(alice.ID, "Wonderwall", "Oasis")
	res, _ := repo.CreateResource(song.ID, alice.ID, "https://example.com", "x")

	if _, err := repo.UpdateResource(res.ID, song.ID, bob.ID, nil, nil); err == nil {
		t.Error("non-owner updated resource")
	}
	if err := repo.DeleteResource(res.ID, song.ID, bob.ID); err == nil {
		t.Error("non-owner deleted resource")
	}

	// Mismatched songID should also reject.
	if _, err := repo.UpdateResource(res.ID, 9999, alice.ID, nil, nil); err == nil {
		t.Error("mismatched songID updated resource")
	}
}

func TestBandResourcesIsolatedFromUser(t *testing.T) {
	repo := testRepo(t)
	user, _ := repo.CreateUser("alice", "alice@example.com", "h")
	band, _ := repo.CreateBand(user.ID, "Band")
	song := &model.Song{Title: "Wonderwall", Artist: "Oasis", OwnerBandID: &band.ID}
	if err := repo.db.Create(song).Error; err != nil {
		t.Fatal(err)
	}

	bandRes, err := repo.CreateBandResource(song.ID, band.ID, "https://example.com/band", "band tab")
	if err != nil {
		t.Fatalf("CreateBandResource: %v", err)
	}
	if _, err := repo.CreateResource(song.ID, user.ID, "https://example.com/mine", "my tab"); err != nil {
		t.Fatal(err)
	}

	// Each layer sees only its own resources.
	bandList, _ := repo.ResourcesForSongBand(song.ID, band.ID)
	if len(bandList) != 1 || bandList[0].Label != "band tab" {
		t.Errorf("band resources = %+v", bandList)
	}
	userList, _ := repo.ResourcesForSongUser(song.ID, user.ID)
	if len(userList) != 1 || userList[0].Label != "my tab" {
		t.Errorf("user resources = %+v", userList)
	}

	url := "https://example.com/band2"
	if _, err := repo.UpdateBandResource(bandRes.ID, song.ID, band.ID, &url, nil); err != nil {
		t.Fatalf("UpdateBandResource: %v", err)
	}
	if err := repo.DeleteBandResource(bandRes.ID, song.ID, band.ID); err != nil {
		t.Fatalf("DeleteBandResource: %v", err)
	}
	if bandList, _ = repo.ResourcesForSongBand(song.ID, band.ID); len(bandList) != 0 {
		t.Errorf("band resources after delete = %d", len(bandList))
	}
	if userList, _ = repo.ResourcesForSongUser(song.ID, user.ID); len(userList) != 1 {
		t.Errorf("user resources after band delete = %d, want 1", len(userList))
	}
}
