package repository

import (
	"testing"

	"github.com/jwhumphries/bandwidth/internal/model"
)

func TestAccessPolicyDefaultsDisabled(t *testing.T) {
	repo := testRepo(t)
	enabled, err := repo.AccessPolicyEnabled()
	if err != nil {
		t.Fatalf("AccessPolicyEnabled: %v", err)
	}
	if enabled {
		t.Error("access policy enabled by default, want disabled")
	}
}

func TestSetAccessPolicyEnabled(t *testing.T) {
	repo := testRepo(t)
	if err := repo.SetAccessPolicyEnabled(true); err != nil {
		t.Fatalf("SetAccessPolicyEnabled(true): %v", err)
	}
	enabled, err := repo.AccessPolicyEnabled()
	if err != nil || !enabled {
		t.Fatalf("AccessPolicyEnabled = %v, %v; want true, nil", enabled, err)
	}
	if err := repo.SetAccessPolicyEnabled(false); err != nil {
		t.Fatalf("SetAccessPolicyEnabled(false): %v", err)
	}
	enabled, _ = repo.AccessPolicyEnabled()
	if enabled {
		t.Error("access policy still enabled after Set(false)")
	}
}

func TestAllowedEmailCRUD(t *testing.T) {
	repo := testRepo(t)
	admin, _ := repo.CreateUser("admin", "admin@example.com", "h")

	allowed, err := repo.EmailAllowed("friend@example.com")
	if err != nil || allowed {
		t.Fatalf("EmailAllowed before add = %v, %v; want false, nil", allowed, err)
	}

	entry, err := repo.AddAllowedEmail("friend@example.com", admin.ID)
	if err != nil {
		t.Fatalf("AddAllowedEmail: %v", err)
	}

	allowed, err = repo.EmailAllowed("friend@example.com")
	if err != nil || !allowed {
		t.Fatalf("EmailAllowed after add = %v, %v; want true, nil", allowed, err)
	}

	list, err := repo.AllowedEmails()
	if err != nil {
		t.Fatalf("AllowedEmails: %v", err)
	}
	if len(list) != 1 || list[0].Email != "friend@example.com" {
		t.Fatalf("AllowedEmails = %+v", list)
	}

	if _, err := repo.AddAllowedEmail("friend@example.com", admin.ID); !IsDuplicate(err) {
		t.Errorf("duplicate add err = %v, want a duplicate error", err)
	}

	if err := repo.RemoveAllowedEmail(entry.ID); err != nil {
		t.Fatalf("RemoveAllowedEmail: %v", err)
	}
	allowed, _ = repo.EmailAllowed("friend@example.com")
	if allowed {
		t.Error("email still allowed after removal")
	}
	if err := repo.RemoveAllowedEmail(entry.ID); err == nil {
		t.Error("removing an already-removed entry should error")
	}
}

func TestAllUsersAndAllBands(t *testing.T) {
	repo := testRepo(t)
	alice, _ := repo.CreateUser("alice", "alice@example.com", "h")
	bob, _ := repo.CreateUser("bob", "bob@example.com", "h")
	band, _ := repo.CreateBand(alice.ID, "The Quietones")
	if err := repo.AddMember(band.ID, bob.ID, model.RoleEditor); err != nil {
		t.Fatal(err)
	}

	users, err := repo.AllUsers()
	if err != nil {
		t.Fatalf("AllUsers: %v", err)
	}
	if len(users) != 2 {
		t.Fatalf("AllUsers = %+v, want 2 rows", users)
	}

	bands, err := repo.AllBands()
	if err != nil {
		t.Fatalf("AllBands: %v", err)
	}
	if len(bands) != 1 || bands[0].CreatorUsername != "alice" || bands[0].MemberCount != 2 {
		t.Fatalf("AllBands = %+v", bands)
	}
}
