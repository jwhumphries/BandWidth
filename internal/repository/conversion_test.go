package repository

import (
	"testing"

	"github.com/jwhumphries/bandwidth/internal/model"
)

func TestConversionPreservesPersonalFolderPlacement(t *testing.T) {
	repo := testRepo(t)
	aliceUser, _ := repo.CreateUser("alice", "alice@example.com", "h")
	alice := aliceUser.ID
	aliceBand, _ := repo.CreateBand(alice, "The Quietones")
	band := aliceBand.ID
	bandSong, err := repo.CreateBandSong(band, "Wonderwall", "Oasis")
	if err != nil {
		t.Fatalf("create band song: %v", err)
	}

	// Alice's ONLY personal work on the band song is foldering it.
	folder, _ := repo.CreateFolder(alice, "Faves")
	if err := repo.SetFolderEntries(folder.ID, alice, []uint{bandSong.ID}); err != nil {
		t.Fatalf("set entries: %v", err)
	}

	// Deleting the band song converts Alice's placement onto a personal copy.
	if err := repo.DeleteBandSong(bandSong.ID, band); err != nil {
		t.Fatalf("delete band song: %v", err)
	}

	folders, _ := repo.FoldersForUser(alice)
	if len(folders) != 1 || len(folders[0].SongIDs) != 1 {
		t.Fatalf("folder after conversion = %+v", folders)
	}
	newSongID := folders[0].SongIDs[0]
	if newSongID == bandSong.ID {
		t.Fatalf("folder still points at the deleted band song %d", bandSong.ID)
	}
	// The re-pointed song is Alice's personal copy.
	personal, err := repo.SongForUser(newSongID, alice)
	if err != nil {
		t.Fatalf("personal copy not found: %v", err)
	}
	if personal.OwnerUserID == nil || *personal.OwnerUserID != alice || personal.Title != "Wonderwall" {
		t.Fatalf("personal copy = %+v", personal)
	}
}

// touchPersonalLayer gives the user some personal data on a band song.
func touchPersonalLayer(t *testing.T, repo *Repo, songID, userID uint) {
	t.Helper()
	s := model.StatusLearning
	if err := repo.UpsertAnnotation(songID, userID, &s, nil); err != nil {
		t.Fatal(err)
	}
	if _, err := repo.CreateResource(songID, userID, "https://example.com/mine", "mine"); err != nil {
		t.Fatal(err)
	}
	if err := repo.LogPractice(songID, userID, "2026-06-10"); err != nil {
		t.Fatal(err)
	}
}

func TestLeaveConvertsTouchedSongs(t *testing.T) {
	repo := testRepo(t)
	alice, _ := repo.CreateUser("alice", "alice@example.com", "h")
	bob, _ := repo.CreateUser("bob", "bob@example.com", "h")
	band, _ := repo.CreateBand(alice.ID, "Band")
	_ = repo.AddMember(band.ID, bob.ID, model.RoleEditor)
	touched, _ := repo.CreateBandSong(band.ID, "Touched", "X")
	untouched, _ := repo.CreateBandSong(band.ID, "Untouched", "Y")
	touchPersonalLayer(t, repo, touched.ID, bob.ID)

	if err := repo.RemoveMember(band.ID, bob.ID); err != nil {
		t.Fatalf("RemoveMember: %v", err)
	}

	// Bob's library now has exactly one song: a personal copy of "Touched".
	items, _ := repo.SongsForUser(bob.ID)
	if len(items) != 1 || items[0].Title != "Touched" || items[0].BandID != nil {
		t.Fatalf("bob library after leave = %+v", items)
	}
	// His personal layer survived on the copy.
	if items[0].Status != model.StatusLearning || items[0].PracticeCount != 1 {
		t.Errorf("converted song lost personal layer: %+v", items[0])
	}
	// The band still has both songs, untouched by bob's departure.
	bandItems, _ := repo.SongsForBand(band.ID)
	if len(bandItems) != 2 {
		t.Errorf("band lost songs: %d", len(bandItems))
	}
	// The band song "Touched" no longer carries bob's personal rows.
	var n int64
	repo.db.Model(&model.SongAnnotation{}).
		Where("song_id = ? AND user_id = ?", touched.ID, bob.ID).Count(&n)
	if n != 0 {
		t.Errorf("bob's annotation still on the band song: %d", n)
	}
	_ = untouched
}

func TestDeleteBandSongConvertsPerMember(t *testing.T) {
	repo := testRepo(t)
	alice, _ := repo.CreateUser("alice", "alice@example.com", "h")
	bob, _ := repo.CreateUser("bob", "bob@example.com", "h")
	band, _ := repo.CreateBand(alice.ID, "Band")
	_ = repo.AddMember(band.ID, bob.ID, model.RoleEditor)
	song, _ := repo.CreateBandSong(band.ID, "Doomed", "X")
	bandStatus := model.StatusNailed
	_ = repo.UpsertBandAnnotation(song.ID, band.ID, &bandStatus, nil)
	touchPersonalLayer(t, repo, song.ID, bob.ID) // bob touched it; alice did not

	if err := repo.DeleteBandSong(song.ID, band.ID); err != nil {
		t.Fatalf("DeleteBandSong: %v", err)
	}

	// The band song and its band layer are gone.
	if _, err := repo.SongForBand(song.ID, band.ID); err == nil {
		t.Error("band song survived delete")
	}
	var bandAnns int64
	repo.db.Model(&model.SongAnnotation{}).Where("song_id = ? AND band_id = ?", song.ID, band.ID).Count(&bandAnns)
	if bandAnns != 0 {
		t.Errorf("band annotation survived: %d", bandAnns)
	}
	// Bob (touched) keeps a personal copy; alice (untouched) gets nothing.
	if items, _ := repo.SongsForUser(bob.ID); len(items) != 1 || items[0].Title != "Doomed" {
		t.Errorf("bob library = %+v, want one converted copy", items)
	}
	if items, _ := repo.SongsForUser(alice.ID); len(items) != 0 {
		t.Errorf("alice library = %+v, want empty", items)
	}
}

func TestDeleteBandSongRemovesFolderEntries(t *testing.T) {
	repo := testRepo(t)
	alice, _ := repo.CreateUser("alice", "alice@example.com", "h")
	band, _ := repo.CreateBand(alice.ID, "Band")
	song, _ := repo.CreateBandSong(band.ID, "Orphan", "X")

	// Directly insert a FolderEntry for the band song, bypassing the API
	// guard (simulating a future state where band songs can enter folders).
	entry := model.FolderEntry{SongID: song.ID, FolderID: 99, Position: 0}
	if err := repo.db.Create(&entry).Error; err != nil {
		t.Fatalf("insert FolderEntry: %v", err)
	}

	if err := repo.DeleteBandSong(song.ID, band.ID); err != nil {
		t.Fatalf("DeleteBandSong: %v", err)
	}

	var n int64
	repo.db.Model(&model.FolderEntry{}).Where("song_id = ?", song.ID).Count(&n)
	if n != 0 {
		t.Errorf("DeleteBandSong left %d orphan FolderEntry row(s) for song %d", n, song.ID)
	}
}

func TestDeleteBandConvertsForAllMembers(t *testing.T) {
	repo := testRepo(t)
	alice, _ := repo.CreateUser("alice", "alice@example.com", "h")
	bob, _ := repo.CreateUser("bob", "bob@example.com", "h")
	band, _ := repo.CreateBand(alice.ID, "Band")
	_ = repo.AddMember(band.ID, bob.ID, model.RoleEditor)
	song, _ := repo.CreateBandSong(band.ID, "Shared", "X")
	touchPersonalLayer(t, repo, song.ID, bob.ID)

	if err := repo.DeleteBand(band.ID); err != nil {
		t.Fatalf("DeleteBand: %v", err)
	}
	if _, err := repo.BandByID(band.ID); err == nil {
		t.Error("band survived delete")
	}
	// Bob's touched song converted; the band song row is gone.
	items, _ := repo.SongsForUser(bob.ID)
	if len(items) != 1 || items[0].Title != "Shared" || items[0].BandID != nil {
		t.Errorf("bob library after band delete = %+v", items)
	}
	if items, _ := repo.SongsForUser(alice.ID); len(items) != 0 {
		t.Errorf("alice library after band delete = %+v", items)
	}
}
