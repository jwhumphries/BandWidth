package repository

import "testing"

func TestFolderLifecycle(t *testing.T) {
	repo := testRepo(t)
	user, _ := repo.CreateUser("alice", "alice@example.com", "h")
	s1, _ := repo.CreateSong(user.ID, "Wonderwall", "Oasis")
	s2, _ := repo.CreateSong(user.ID, "Creep", "Radiohead")

	f1, err := repo.CreateFolder(user.ID, "Setlist")
	if err != nil {
		t.Fatalf("CreateFolder: %v", err)
	}
	f2, _ := repo.CreateFolder(user.ID, "Practice queue")
	if f2.Position <= f1.Position {
		t.Errorf("folder positions not appended: %d then %d", f1.Position, f2.Position)
	}

	// Membership + order (playlist-style: same song can be in many folders).
	if err := repo.SetFolderEntries(f1.ID, user.ID, []uint{s2.ID, s1.ID}); err != nil {
		t.Fatalf("SetFolderEntries: %v", err)
	}
	if err := repo.SetFolderEntries(f2.ID, user.ID, []uint{s1.ID}); err != nil {
		t.Fatal(err)
	}

	folders, err := repo.FoldersForUser(user.ID)
	if err != nil || len(folders) != 2 {
		t.Fatalf("FoldersForUser: %d (%v)", len(folders), err)
	}
	if folders[0].ID != f1.ID || folders[1].ID != f2.ID {
		t.Error("folders not in position order")
	}
	if len(folders[0].SongIDs) != 2 || folders[0].SongIDs[0] != s2.ID || folders[0].SongIDs[1] != s1.ID {
		t.Errorf("f1 song order = %v, want [s2 s1]", folders[0].SongIDs)
	}

	// Replacing entries replaces order and membership.
	if err := repo.SetFolderEntries(f1.ID, user.ID, []uint{s1.ID}); err != nil {
		t.Fatal(err)
	}
	folders, _ = repo.FoldersForUser(user.ID)
	if len(folders[0].SongIDs) != 1 || folders[0].SongIDs[0] != s1.ID {
		t.Errorf("f1 after replace = %v", folders[0].SongIDs)
	}

	if err := repo.RenameFolder(f1.ID, user.ID, "Gig 6/20"); err != nil {
		t.Fatalf("RenameFolder: %v", err)
	}
	folders, _ = repo.FoldersForUser(user.ID)
	if folders[0].Name != "Gig 6/20" {
		t.Errorf("name after rename = %q", folders[0].Name)
	}

	// Reorder folders.
	if err := repo.ReorderFolders(user.ID, []uint{f2.ID, f1.ID}); err != nil {
		t.Fatalf("ReorderFolders: %v", err)
	}
	folders, _ = repo.FoldersForUser(user.ID)
	if folders[0].ID != f2.ID {
		t.Error("folder reorder not applied")
	}

	// Deleting a folder removes entries but never songs.
	if err := repo.DeleteFolder(f1.ID, user.ID); err != nil {
		t.Fatalf("DeleteFolder: %v", err)
	}
	folders, _ = repo.FoldersForUser(user.ID)
	if len(folders) != 1 {
		t.Fatalf("folders after delete = %d", len(folders))
	}
	if items, _ := repo.SongsForUser(user.ID); len(items) != 2 {
		t.Errorf("songs after folder delete = %d, want 2", len(items))
	}
}

func TestFolderOwnershipChecks(t *testing.T) {
	repo := testRepo(t)
	alice, _ := repo.CreateUser("alice", "alice@example.com", "h")
	bob, _ := repo.CreateUser("bob", "bob@example.com", "h")
	folder, _ := repo.CreateFolder(alice.ID, "Mine")
	bobSong, _ := repo.CreateSong(bob.ID, "Creep", "Radiohead")

	if err := repo.RenameFolder(folder.ID, bob.ID, "x"); err == nil {
		t.Error("non-owner renamed folder")
	}
	if err := repo.DeleteFolder(folder.ID, bob.ID); err == nil {
		t.Error("non-owner deleted folder")
	}
	if err := repo.SetFolderEntries(folder.ID, bob.ID, nil); err == nil {
		t.Error("non-owner set entries")
	}
	// Songs not visible to the owner are rejected from entries.
	if err := repo.SetFolderEntries(folder.ID, alice.ID, []uint{bobSong.ID}); err == nil {
		t.Error("invisible song accepted into folder")
	}
}
