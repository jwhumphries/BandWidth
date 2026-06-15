package repository

import (
	"errors"
	"testing"

	"gorm.io/gorm"
)

func TestBandFolderCRUDAndEntries(t *testing.T) {
	repo := testRepo(t)
	aliceUser, _ := repo.CreateUser("alice", "alice@example.com", "h")
	alice := aliceUser.ID
	aliceBand, _ := repo.CreateBand(alice, "The Quietones")
	band := aliceBand.ID

	// Create two band folders; positions auto-increment.
	f1, err := repo.CreateBandFolder(band, "Set 1")
	if err != nil {
		t.Fatalf("create band folder: %v", err)
	}
	if _, err := repo.CreateBandFolder(band, "Set 2"); err != nil {
		t.Fatalf("create band folder 2: %v", err)
	}

	song, err := repo.CreateBandSong(band, "Wonderwall", "Oasis")
	if err != nil {
		t.Fatalf("create band song: %v", err)
	}

	// A band song can go into a band folder.
	if err := repo.SetBandFolderEntries(f1.ID, band, []uint{song.ID}); err != nil {
		t.Fatalf("set band folder entries: %v", err)
	}
	folders, err := repo.FoldersForBand(band)
	if err != nil {
		t.Fatalf("folders for band: %v", err)
	}
	if len(folders) != 2 || folders[0].Name != "Set 1" {
		t.Fatalf("band folders = %+v", folders)
	}
	if len(folders[0].SongIDs) != 1 || folders[0].SongIDs[0] != song.ID {
		t.Fatalf("band folder entries = %+v", folders[0])
	}

	// A song from a DIFFERENT band cannot be placed in this band's folder.
	otherBandObj, _ := repo.CreateBand(alice, "Other")
	otherBand := otherBandObj.ID
	otherSong, _ := repo.CreateBandSong(otherBand, "Creep", "Radiohead")
	err = repo.SetBandFolderEntries(f1.ID, band, []uint{otherSong.ID})
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		t.Fatalf("cross-band entry = %v, want ErrRecordNotFound", err)
	}

	// Rename, reorder, delete operate only on this band's folders.
	if err := repo.RenameBandFolder(f1.ID, band, "Opener"); err != nil {
		t.Fatalf("rename: %v", err)
	}
	if err := repo.ReorderBandFolders(band, []uint{folders[1].ID, f1.ID}); err != nil {
		t.Fatalf("reorder: %v", err)
	}
	if err := repo.DeleteBandFolder(f1.ID, band); err != nil {
		t.Fatalf("delete: %v", err)
	}
	remaining, _ := repo.FoldersForBand(band)
	if len(remaining) != 1 || remaining[0].Name != "Set 2" {
		t.Fatalf("after delete = %+v", remaining)
	}
}
