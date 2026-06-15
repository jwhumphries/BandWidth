package repository

import (
	"testing"

	"github.com/jwhumphries/bandwidth/internal/model"
)

func TestSongsForUserIncludesBandSongs(t *testing.T) {
	repo := testRepo(t)
	alice, _ := repo.CreateUser("alice", "alice@example.com", "h")
	bob, _ := repo.CreateUser("bob", "bob@example.com", "h")
	band, _ := repo.CreateBand(alice.ID, "The Quietones")
	_ = repo.AddMember(band.ID, bob.ID, model.RoleEditor)

	_, _ = repo.CreateSong(bob.ID, "My Own Song", "Me")
	bandSong, _ := repo.CreateBandSong(band.ID, "Band Song", "Them")

	// Bob's library shows both his own song and the band song.
	items, err := repo.SongsForUser(bob.ID)
	if err != nil || len(items) != 2 {
		t.Fatalf("SongsForUser: %d items (%v)", len(items), err)
	}
	var own, shared *SongListItem
	for i := range items {
		switch items[i].Title {
		case "My Own Song":
			own = &items[i]
		case "Band Song":
			shared = &items[i]
		}
	}
	if own == nil || own.BandID != nil {
		t.Errorf("own song tagged with band: %+v", own)
	}
	if shared == nil || shared.BandID == nil || *shared.BandID != band.ID || shared.BandName != "The Quietones" {
		t.Errorf("band song not tagged: %+v", shared)
	}

	// The status shown for the band song is BOB'S personal layer, not the
	// band's. Set a band status and a different personal status.
	bandStatus := model.StatusNailed
	_ = repo.UpsertBandAnnotation(bandSong.ID, band.ID, &bandStatus, nil)
	personal := model.StatusLearning
	_ = repo.UpsertAnnotation(bandSong.ID, bob.ID, &personal, nil)
	items, _ = repo.SongsForUser(bob.ID)
	for i := range items {
		if items[i].Title == "Band Song" && items[i].Status != model.StatusLearning {
			t.Errorf("band song list status = %q, want bob's personal 'learning'", items[i].Status)
		}
	}

	// A non-member's library never shows the band song.
	carol, _ := repo.CreateUser("carol", "carol@example.com", "h")
	if items, _ := repo.SongsForUser(carol.ID); len(items) != 0 {
		t.Errorf("non-member library = %d, want 0", len(items))
	}
}

func TestCreateAndListSongs(t *testing.T) {
	repo := testRepo(t)
	user, _ := repo.CreateUser("alice", "alice@example.com", "h")
	other, _ := repo.CreateUser("bob", "bob@example.com", "h")

	song, err := repo.CreateSong(user.ID, "Wonderwall", "Oasis")
	if err != nil {
		t.Fatalf("CreateSong: %v", err)
	}
	if song.ID == 0 || *song.OwnerUserID != user.ID {
		t.Fatalf("bad song: %+v", song)
	}
	if _, err := repo.CreateSong(other.ID, "Creep", "Radiohead"); err != nil {
		t.Fatal(err)
	}

	items, err := repo.SongsForUser(user.ID)
	if err != nil {
		t.Fatalf("SongsForUser: %v", err)
	}
	if len(items) != 1 {
		t.Fatalf("got %d songs, want 1 (no leakage between users)", len(items))
	}
	got := items[0]
	if got.Title != "Wonderwall" || got.Artist != "Oasis" {
		t.Errorf("item identity: %+v", got)
	}
	if got.Status != model.StatusNotLearned {
		t.Errorf("default status = %q, want not_learned", got.Status)
	}
	if got.PracticeCount != 0 || got.LastPracticedAt != "" {
		t.Errorf("practice defaults: %+v", got)
	}
}

func TestSongForUserVisibility(t *testing.T) {
	repo := testRepo(t)
	alice, _ := repo.CreateUser("alice", "alice@example.com", "h")
	bob, _ := repo.CreateUser("bob", "bob@example.com", "h")
	song, _ := repo.CreateSong(alice.ID, "Wonderwall", "Oasis")

	if _, err := repo.SongForUser(song.ID, alice.ID); err != nil {
		t.Errorf("owner cannot see own song: %v", err)
	}
	if _, err := repo.SongForUser(song.ID, bob.ID); err == nil {
		t.Error("non-owner can see song")
	}
	if _, err := repo.SongForUser(9999, alice.ID); err == nil {
		t.Error("missing song found")
	}
}

func TestUpsertAnnotation(t *testing.T) {
	repo := testRepo(t)
	user, _ := repo.CreateUser("alice", "alice@example.com", "h")
	song, _ := repo.CreateSong(user.ID, "Wonderwall", "Oasis")

	status := model.StatusLearning
	if err := repo.UpsertAnnotation(song.ID, user.ID, &status, nil); err != nil {
		t.Fatalf("UpsertAnnotation(create): %v", err)
	}
	items, _ := repo.SongsForUser(user.ID)
	if items[0].Status != model.StatusLearning {
		t.Fatalf("status after upsert = %q", items[0].Status)
	}

	// Update notes only; status must survive.
	notes := "capo on 2"
	if err := repo.UpsertAnnotation(song.ID, user.ID, nil, &notes); err != nil {
		t.Fatalf("UpsertAnnotation(update): %v", err)
	}
	ann, err := repo.AnnotationForSongUser(song.ID, user.ID)
	if err != nil {
		t.Fatalf("AnnotationForSongUser: %v", err)
	}
	if ann.Status != model.StatusLearning || ann.Notes != "capo on 2" {
		t.Errorf("annotation = %+v", ann)
	}

	// Exactly one row exists.
	var n int64
	repo.db.Model(&model.SongAnnotation{}).Where("song_id = ?", song.ID).Count(&n)
	if n != 1 {
		t.Errorf("annotation rows = %d, want 1", n)
	}
}

func TestDeleteSongCascades(t *testing.T) {
	repo := testRepo(t)
	user, _ := repo.CreateUser("alice", "alice@example.com", "h")
	song, _ := repo.CreateSong(user.ID, "Wonderwall", "Oasis")

	status := model.StatusLearned
	_ = repo.UpsertAnnotation(song.ID, user.ID, &status, nil)
	_, _ = repo.CreateResource(song.ID, user.ID, "https://example.com/tab", "tab")
	_ = repo.LogPractice(song.ID, user.ID, "2026-06-10")
	folder, _ := repo.CreateFolder(user.ID, "Setlist")
	_ = repo.SetFolderEntries(folder.ID, user.ID, []uint{song.ID})

	if err := repo.DeleteSong(song.ID, user.ID); err != nil {
		t.Fatalf("DeleteSong: %v", err)
	}

	for table, m := range map[string]any{
		"songs":            &model.Song{},
		"song_annotations": &model.SongAnnotation{},
		"resources":        &model.Resource{},
		"practice_events":  &model.PracticeEvent{},
		"folder_entries":   &model.FolderEntry{},
	} {
		var n int64
		repo.db.Model(m).Count(&n)
		if n != 0 {
			t.Errorf("%s rows after delete = %d, want 0", table, n)
		}
	}

	// Deleting a song you don't own fails.
	song2, _ := repo.CreateSong(user.ID, "Creep", "Radiohead")
	bob, _ := repo.CreateUser("bob", "bob@example.com", "h")
	if err := repo.DeleteSong(song2.ID, bob.ID); err == nil {
		t.Error("non-owner deleted song")
	}
}
