package repository

import "testing"

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
	updated, err := repo.UpdateResource(first.ID, user.ID, &url, nil)
	if err != nil || updated.URL != url || updated.Label != "tab" {
		t.Fatalf("UpdateResource: %+v (%v)", updated, err)
	}

	if err := repo.DeleteResource(first.ID, user.ID); err != nil {
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

	if _, err := repo.UpdateResource(res.ID, bob.ID, nil, nil); err == nil {
		t.Error("non-owner updated resource")
	}
	if err := repo.DeleteResource(res.ID, bob.ID); err == nil {
		t.Error("non-owner deleted resource")
	}
}
