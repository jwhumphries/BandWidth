package repository

import "testing"

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
