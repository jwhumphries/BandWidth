package repository

import (
	"testing"

	"github.com/jwhumphries/bandwidth/internal/model"
)

func TestCreateAndFindUser(t *testing.T) {
	repo := testRepo(t)

	user, err := repo.CreateUser("alice", "alice@example.com", "hash")
	if err != nil {
		t.Fatalf("CreateUser: %v", err)
	}
	if user.ID == 0 {
		t.Fatal("user ID not assigned")
	}

	byName, err := repo.UserByLogin("alice")
	if err != nil || byName.ID != user.ID {
		t.Errorf("UserByLogin(username) = %v, %v", byName, err)
	}
	byEmail, err := repo.UserByLogin("alice@example.com")
	if err != nil || byEmail.ID != user.ID {
		t.Errorf("UserByLogin(email) = %v, %v", byEmail, err)
	}
	byID, err := repo.UserByID(user.ID)
	if err != nil || byID.Username != "alice" {
		t.Errorf("UserByID = %v, %v", byID, err)
	}
	if _, err := repo.UserByLogin("nobody"); err == nil {
		t.Error("UserByLogin(nobody) should fail")
	}
}

func TestCreateUserDuplicate(t *testing.T) {
	repo := testRepo(t)
	if _, err := repo.CreateUser("alice", "alice@example.com", "h"); err != nil {
		t.Fatal(err)
	}

	_, err := repo.CreateUser("alice", "other@example.com", "h")
	if !IsDuplicate(err) {
		t.Errorf("duplicate username: IsDuplicate = false, err = %v", err)
	}
	_, err = repo.CreateUser("bob", "alice@example.com", "h")
	if !IsDuplicate(err) {
		t.Errorf("duplicate email: IsDuplicate = false, err = %v", err)
	}
	if IsDuplicate(nil) {
		t.Error("IsDuplicate(nil) = true")
	}
}

func TestSaveUser(t *testing.T) {
	repo := testRepo(t)
	user, _ := repo.CreateUser("alice", "alice@example.com", "h")

	user.Email = "new@example.com"
	if err := repo.SaveUser(user); err != nil {
		t.Fatalf("SaveUser: %v", err)
	}
	again, _ := repo.UserByID(user.ID)
	if again.Email != "new@example.com" {
		t.Errorf("email = %q after save", again.Email)
	}
}

func TestDeleteUserRemovesPersonalData(t *testing.T) {
	repo := testRepo(t)
	alice, _ := repo.CreateUser("alice", "alice@example.com", "h")
	song, _ := repo.CreateSong(alice.ID, "Solo", "X")
	folder, _ := repo.CreateFolder(alice.ID, "Faves")
	if err := repo.SetFolderEntries(folder.ID, alice.ID, []uint{song.ID}); err != nil {
		t.Fatal(err)
	}
	token, _ := repo.CreateSession(alice.ID)

	if err := repo.DeleteUser(alice.ID); err != nil {
		t.Fatalf("DeleteUser: %v", err)
	}

	if _, err := repo.UserByID(alice.ID); err == nil {
		t.Error("user survived delete")
	}
	if _, err := repo.SongForUser(song.ID, alice.ID); err == nil {
		t.Error("personal song survived delete")
	}
	if items, _ := repo.FoldersForUser(alice.ID); len(items) != 0 {
		t.Errorf("folders survived delete: %+v", items)
	}
	if _, err := repo.SessionUser(token); err == nil {
		t.Error("session survived delete")
	}
}

func TestDeleteUserCascadesCreatedBands(t *testing.T) {
	repo := testRepo(t)
	alice, _ := repo.CreateUser("alice", "alice@example.com", "h")
	bob, _ := repo.CreateUser("bob", "bob@example.com", "h")
	band, _ := repo.CreateBand(alice.ID, "Band")
	if err := repo.AddMember(band.ID, bob.ID, model.RoleEditor); err != nil {
		t.Fatal(err)
	}
	song, _ := repo.CreateBandSong(band.ID, "Shared", "X")
	touchPersonalLayer(t, repo, song.ID, bob.ID)

	if err := repo.DeleteUser(alice.ID); err != nil {
		t.Fatalf("DeleteUser: %v", err)
	}

	if _, err := repo.BandByID(band.ID); err == nil {
		t.Error("band created by the deleted user survived")
	}
	items, _ := repo.SongsForUser(bob.ID)
	if len(items) != 1 || items[0].Title != "Shared" {
		t.Errorf("bob library after creator deleted = %+v, want one converted copy", items)
	}
}

func TestDeleteUserClearsPersonalLayerOnOtherBands(t *testing.T) {
	repo := testRepo(t)
	alice, _ := repo.CreateUser("alice", "alice@example.com", "h")
	bob, _ := repo.CreateUser("bob", "bob@example.com", "h")
	band, _ := repo.CreateBand(alice.ID, "Band")
	if err := repo.AddMember(band.ID, bob.ID, model.RoleEditor); err != nil {
		t.Fatal(err)
	}
	song, _ := repo.CreateBandSong(band.ID, "Shared", "X")
	touchPersonalLayer(t, repo, song.ID, bob.ID)

	if err := repo.DeleteUser(bob.ID); err != nil {
		t.Fatalf("DeleteUser: %v", err)
	}

	if _, err := repo.SongForBand(song.ID, band.ID); err != nil {
		t.Errorf("band song did not survive non-creator delete: %v", err)
	}
	var n int64
	repo.db.Model(&model.SongAnnotation{}).
		Where("song_id = ? AND user_id = ?", song.ID, bob.ID).Count(&n)
	if n != 0 {
		t.Errorf("bob's personal layer survived his own deletion: %d rows", n)
	}
}
