package repository

import (
	"testing"

	"github.com/jwhumphries/bandwidth/internal/model"
)

func TestCreateBandMakesCreatorAdmin(t *testing.T) {
	repo := testRepo(t)
	user, _ := repo.CreateUser("alice", "alice@example.com", "h")

	band, err := repo.CreateBand(user.ID, "The Quietones")
	if err != nil {
		t.Fatalf("CreateBand: %v", err)
	}
	if band.CreatorID != user.ID {
		t.Errorf("creator = %d", band.CreatorID)
	}
	role, err := repo.MemberRole(band.ID, user.ID)
	if err != nil || role != model.RoleAdmin {
		t.Fatalf("creator role = %q, %v", role, err)
	}
}

func TestMemberRoleNonMember(t *testing.T) {
	repo := testRepo(t)
	alice, _ := repo.CreateUser("alice", "alice@example.com", "h")
	bob, _ := repo.CreateUser("bob", "bob@example.com", "h")
	band, _ := repo.CreateBand(alice.ID, "Band")

	if _, err := repo.MemberRole(band.ID, bob.ID); err == nil {
		t.Error("non-member has a role")
	}
}

func TestBandsForUserAndMembers(t *testing.T) {
	repo := testRepo(t)
	alice, _ := repo.CreateUser("alice", "alice@example.com", "h")
	bob, _ := repo.CreateUser("bob", "bob@example.com", "h")
	band, _ := repo.CreateBand(alice.ID, "Band")
	if err := repo.AddMember(band.ID, bob.ID, model.RoleEditor); err != nil {
		t.Fatalf("AddMember: %v", err)
	}

	summaries, err := repo.BandsForUser(bob.ID)
	if err != nil || len(summaries) != 1 {
		t.Fatalf("BandsForUser: %v (%v)", summaries, err)
	}
	if summaries[0].Role != model.RoleEditor || summaries[0].MemberCount != 2 {
		t.Errorf("summary = %+v", summaries[0])
	}
	if summaries[0].CreatorID != alice.ID {
		t.Errorf("summary creator = %d, want %d", summaries[0].CreatorID, alice.ID)
	}

	members, err := repo.MembersForBand(band.ID)
	if err != nil || len(members) != 2 {
		t.Fatalf("MembersForBand: %v (%v)", members, err)
	}
	// Ordered by join time: creator first.
	if members[0].Username != "alice" || members[0].Role != model.RoleAdmin {
		t.Errorf("first member = %+v", members[0])
	}
	if members[1].Username != "bob" {
		t.Errorf("second member = %+v", members[1])
	}

	// Duplicate membership rejected.
	if err := repo.AddMember(band.ID, bob.ID, model.RoleViewer); err == nil {
		t.Error("duplicate membership allowed")
	}
}

func TestSetMemberRoleAndRemove(t *testing.T) {
	repo := testRepo(t)
	alice, _ := repo.CreateUser("alice", "alice@example.com", "h")
	bob, _ := repo.CreateUser("bob", "bob@example.com", "h")
	band, _ := repo.CreateBand(alice.ID, "Band")
	_ = repo.AddMember(band.ID, bob.ID, model.RoleEditor)

	if err := repo.SetMemberRole(band.ID, bob.ID, model.RoleViewer); err != nil {
		t.Fatalf("SetMemberRole: %v", err)
	}
	role, _ := repo.MemberRole(band.ID, bob.ID)
	if role != model.RoleViewer {
		t.Errorf("role after set = %q", role)
	}
	// Unknown member errors.
	if err := repo.SetMemberRole(band.ID, 9999, model.RoleViewer); err == nil {
		t.Error("set role on non-member succeeded")
	}

	if err := repo.RemoveMember(band.ID, bob.ID); err != nil {
		t.Fatalf("RemoveMember: %v", err)
	}
	if _, err := repo.MemberRole(band.ID, bob.ID); err == nil {
		t.Error("removed member still has role")
	}
}

func TestRenameAndDeleteBand(t *testing.T) {
	repo := testRepo(t)
	alice, _ := repo.CreateUser("alice", "alice@example.com", "h")
	band, _ := repo.CreateBand(alice.ID, "Old")

	if err := repo.RenameBand(band.ID, "New"); err != nil {
		t.Fatalf("RenameBand: %v", err)
	}
	got, _ := repo.BandByID(band.ID)
	if got.Name != "New" {
		t.Errorf("name = %q", got.Name)
	}

	if err := repo.DeleteBand(band.ID); err != nil {
		t.Fatalf("DeleteBand: %v", err)
	}
	if _, err := repo.BandByID(band.ID); err == nil {
		t.Error("band survived delete")
	}
	if _, err := repo.MemberRole(band.ID, alice.ID); err == nil {
		t.Error("membership survived delete")
	}
}
