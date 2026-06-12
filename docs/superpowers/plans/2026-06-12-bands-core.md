# BandWidth Bands Core Implementation Plan (Plan 4a of 6)

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Bands without songs: create bands, member roster with Admin/Editor/Viewer roles, direct invites (by username/email) and revocable share links, accept/decline, leave/remove, rename/delete — backend and frontend.

**Architecture:** Three new models (Band, BandMember, BandInvite) with the role enum. Membership is the authorization primitive: `MemberRole(bandID, userID)` feeds a `bandAccess` handler helper that enforces role thresholds (non-members get 404s). Invites reuse the existing token primitives (raw token to the client once, SHA-256 at rest); direct invites are single-use, share links are multi-use until revoked/expired. Band SONGS, the band metadata layer, personal-view interleaving, and the leave/delete conversion engine are Plan 4b — `RemoveMember`/`DeleteBand` here are written so 4b can extend them with conversion.

**Tech Stack:** Existing Go/Echo/GORM and React/TanStack Query stacks. No new dependencies.

---

## Conventions for the executor

- Repo root `/Users/john/code/git/BandWidth`, branch off `main` (e.g. `bands-core`).
- All verification through `just` recipes (Dagger, Bash timeout 600000 ms): `just test`, `just lint-go`, `just test-frontend`, `just typecheck`, `just lint-js`, `just format-check` (`just format` to fix), full gate `just check` (Tasks 7 and 12). Host commands only for dependency management and `go doc`.
- Echo v5 pointer contexts; existing helpers: handlers `songID`/`notFoundOr` (uint :id parsing / 404 mapping), test helpers `newTestAPI`, `postJSON`, `sessionCookie`, `signupAndCookie`, `jsonReq`; repository `testRepo`. Handlers nil-guard `appmw.CurrentUser(c)`.
- Role semantics fixed by the spec: creator is a permanent Admin (cannot be demoted, removed, or leave — they delete the band instead); new members default to Editor; only Admins manage members/invites/band settings; band deletion is creator-only.
- Invite semantics fixed in this plan: direct invites expire in 14 days and are single-use; share links expire in 7 days and are multi-use until revoked. Tokens stored hashed (`auth.HashToken`). Decline marks `revoked_at`. Accepting when already a member is idempotent (returns the band, marks the invite accepted).

## File structure being built

```
internal/model/bands.go              # Band, BandMember, BandInvite, BandRole
internal/repository/repository.go    # AutoMigrate gains the three models
internal/repository/bands.go         # CreateBand, BandsForUser, BandByID, MemberRole,
                                     # MembersForBand, RenameBand, SetMemberRole,
                                     # RemoveMember, DeleteBand
internal/repository/bandinvites.go   # direct/link create, pending lists, revoke,
                                     # accept/decline, join-by-link
internal/handlers/bands.go           # bandAccess helper + Bands/CreateBand/Band/
                                     # RenameBand/DeleteBand
internal/handlers/bandmembers.go     # SetMemberRole, RemoveMember (admin or self-leave)
internal/handlers/bandinvites.go     # CreateInvite, BandInvites, RevokeInvite,
                                     # MyInvites, AcceptInvite, DeclineInvite, JoinByLink
cmd/bandwidth/server.go              # /api/bands + /api/invites groups
frontend/src/lib/types.ts            # BandRole, BandSummary, BandDetail, MyInvite, ...
frontend/src/hooks/bands.ts          # band + member + band-invite hooks
frontend/src/hooks/invites.ts        # my-invites + accept/decline/join hooks
frontend/src/pages/BandsPage.tsx     # list + create + pending invites
frontend/src/pages/BandPage.tsx      # roster, role management, invites, settings
frontend/src/pages/JoinPage.tsx      # /join/:token
frontend/src/components/bands/*     # MemberList, InviteManager, BandSettings, BandsCard
frontend/src/components/Layout.tsx  # Bands nav link + invite badge
frontend/src/pages/ProfilePage.tsx  # BandsCard (list + leave)
frontend/src/App.tsx                # /bands, /bands/:id, /join/:token routes
```

---

### Task 1: Band models + migration

**Files:**
- Create: `internal/model/bands.go`
- Modify: `internal/repository/repository.go` (AutoMigrate), `internal/repository/repository_test.go` (table list)

- [ ] **Step 1: Write the failing test**

In `internal/repository/repository_test.go`, extend the table list in `TestOpenMigratesSchema` with `"bands", "band_members", "band_invites"` (after `"folder_entries"`).

Run: `just test`. Expected: FAIL — tables missing.

- [ ] **Step 2: Write `internal/model/bands.go`**

```go
package model

import "time"

// BandRole is a member's permission level within a band.
type BandRole string

// Band roles, in ascending privilege order.
const (
	RoleViewer BandRole = "viewer"
	RoleEditor BandRole = "editor"
	RoleAdmin  BandRole = "admin"
)

// Valid reports whether r is a known role.
func (r BandRole) Valid() bool {
	switch r {
	case RoleViewer, RoleEditor, RoleAdmin:
		return true
	}
	return false
}

func (r BandRole) rank() int {
	switch r {
	case RoleAdmin:
		return 3
	case RoleEditor:
		return 2
	case RoleViewer:
		return 1
	}
	return 0
}

// AtLeast reports whether r grants at least min's privileges.
func (r BandRole) AtLeast(min BandRole) bool {
	return r.rank() >= min.rank()
}

// Band is a group of users sharing songs. The creator is a permanent Admin.
type Band struct {
	ID        uint   `gorm:"primarykey"`
	Name      string `gorm:"not null"`
	CreatorID uint   `gorm:"not null"`
	CreatedAt time.Time
	UpdatedAt time.Time
}

// BandMember links a user to a band with a role.
type BandMember struct {
	ID        uint     `gorm:"primarykey"`
	BandID    uint     `gorm:"not null;uniqueIndex:idx_band_member"`
	UserID    uint     `gorm:"not null;uniqueIndex:idx_band_member"`
	Role      BandRole `gorm:"not null"`
	CreatedAt time.Time
}

// BandInvite is either a direct invite (InvitedUserID set, single-use) or a
// share link (TokenHash set, multi-use until revoked/expired). Declining a
// direct invite sets RevokedAt.
type BandInvite struct {
	ID            uint     `gorm:"primarykey"`
	BandID        uint     `gorm:"index;not null"`
	Role          BandRole `gorm:"not null"`
	InvitedUserID *uint    `gorm:"index"`
	TokenHash     *string  `gorm:"uniqueIndex"`
	ExpiresAt     time.Time `gorm:"not null"`
	RevokedAt     *time.Time
	AcceptedAt    *time.Time
	CreatedBy     uint `gorm:"not null"`
	CreatedAt     time.Time
}
```

- [ ] **Step 3: Extend AutoMigrate** in `internal/repository/repository.go` — append `&model.Band{}, &model.BandMember{}, &model.BandInvite{},` to the AutoMigrate list (keep the close-on-error path).

- [ ] **Step 4: Run `just test` + `just lint-go` (green/clean), commit**

```bash
git add internal/model/ internal/repository/
git commit -m "feat: band domain models and migration"
```

---

### Task 2: Band + membership repository

**Files:**
- Create: `internal/repository/bands.go`
- Test: `internal/repository/bands_test.go`

- [ ] **Step 1: Write the failing tests** — `internal/repository/bands_test.go`:

```go
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
```

- [ ] **Step 2: Run `just test` — FAIL (undefined methods).**

- [ ] **Step 3: Write `internal/repository/bands.go`**

```go
package repository

import (
	"gorm.io/gorm"

	"github.com/jwhumphries/bandwidth/internal/model"
)

// BandSummary is one row of a user's band list.
type BandSummary struct {
	ID          uint           `json:"id"`
	Name        string         `json:"name"`
	Role        model.BandRole `json:"role"`
	MemberCount int            `json:"memberCount"`
}

// BandMemberInfo is one roster row.
type BandMemberInfo struct {
	UserID   uint           `json:"userId"`
	Username string         `json:"username"`
	Role     model.BandRole `json:"role"`
}

// CreateBand creates a band with the creator as permanent Admin.
func (r *Repo) CreateBand(creatorID uint, name string) (*model.Band, error) {
	band := &model.Band{Name: name, CreatorID: creatorID}
	err := r.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(band).Error; err != nil {
			return err
		}
		member := &model.BandMember{
			BandID: band.ID, UserID: creatorID, Role: model.RoleAdmin,
		}
		return tx.Create(member).Error
	})
	if err != nil {
		return nil, err
	}
	return band, nil
}

// AddMember adds a user to a band with a role.
func (r *Repo) AddMember(bandID, userID uint, role model.BandRole) error {
	return r.db.Create(&model.BandMember{
		BandID: bandID, UserID: userID, Role: role,
	}).Error
}

// BandsForUser lists the user's bands with their role and the member count.
func (r *Repo) BandsForUser(userID uint) ([]BandSummary, error) {
	summaries := []BandSummary{}
	err := r.db.Table("band_members").
		Select(`bands.id, bands.name, band_members.role,
			(SELECT COUNT(*) FROM band_members bm WHERE bm.band_id = bands.id) AS member_count`).
		Joins("JOIN bands ON bands.id = band_members.band_id").
		Where("band_members.user_id = ?", userID).
		Order("bands.name COLLATE NOCASE, bands.id").
		Scan(&summaries).Error
	if err != nil {
		return nil, err
	}
	return summaries, nil
}

// BandByID loads a band.
func (r *Repo) BandByID(bandID uint) (*model.Band, error) {
	var band model.Band
	if err := r.db.First(&band, bandID).Error; err != nil {
		return nil, err
	}
	return &band, nil
}

// MemberRole returns the user's role in the band, or
// gorm.ErrRecordNotFound for non-members.
func (r *Repo) MemberRole(bandID, userID uint) (model.BandRole, error) {
	var member model.BandMember
	err := r.db.Where("band_id = ? AND user_id = ?", bandID, userID).
		First(&member).Error
	if err != nil {
		return "", err
	}
	return member.Role, nil
}

// MembersForBand returns the roster with usernames, oldest member first.
func (r *Repo) MembersForBand(bandID uint) ([]BandMemberInfo, error) {
	members := []BandMemberInfo{}
	err := r.db.Table("band_members").
		Select("band_members.user_id, users.username, band_members.role").
		Joins("JOIN users ON users.id = band_members.user_id").
		Where("band_members.band_id = ?", bandID).
		Order("band_members.created_at, band_members.id").
		Scan(&members).Error
	if err != nil {
		return nil, err
	}
	return members, nil
}

// RenameBand updates the band's name.
func (r *Repo) RenameBand(bandID uint, name string) error {
	return r.db.Model(&model.Band{}).Where("id = ?", bandID).
		Update("name", name).Error
}

// SetMemberRole changes a member's role. Unknown members error.
func (r *Repo) SetMemberRole(bandID, userID uint, role model.BandRole) error {
	res := r.db.Model(&model.BandMember{}).
		Where("band_id = ? AND user_id = ?", bandID, userID).
		Update("role", role)
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected != 1 {
		return gorm.ErrRecordNotFound
	}
	return nil
}

// RemoveMember removes a user from a band. The bands-songs plan extends
// this with the personal-copy conversion for the member's song data.
func (r *Repo) RemoveMember(bandID, userID uint) error {
	res := r.db.Where("band_id = ? AND user_id = ?", bandID, userID).
		Delete(&model.BandMember{})
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected != 1 {
		return gorm.ErrRecordNotFound
	}
	return nil
}

// DeleteBand removes the band, its memberships, and its invites. The
// bands-songs plan extends this with song conversion/deletion.
func (r *Repo) DeleteBand(bandID uint) error {
	return r.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("band_id = ?", bandID).
			Delete(&model.BandMember{}).Error; err != nil {
			return err
		}
		if err := tx.Where("band_id = ?", bandID).
			Delete(&model.BandInvite{}).Error; err != nil {
			return err
		}
		return tx.Delete(&model.Band{}, bandID).Error
	})
}
```

- [ ] **Step 4: Run `just test` + `just lint-go` (green/clean), commit**

```bash
git add internal/repository/
git commit -m "feat: band and membership repository"
```

---

### Task 3: Invite repository

**Files:**
- Create: `internal/repository/bandinvites.go`
- Test: `internal/repository/bandinvites_test.go`

- [ ] **Step 1: Write the failing tests** — `internal/repository/bandinvites_test.go`:

```go
package repository

import (
	"errors"
	"testing"
	"time"

	"github.com/jwhumphries/bandwidth/internal/model"
)

func inviteFixture(t *testing.T) (*Repo, *model.User, *model.User, *model.Band) {
	t.Helper()
	repo := testRepo(t)
	alice, _ := repo.CreateUser("alice", "alice@example.com", "h")
	bob, _ := repo.CreateUser("bob", "bob@example.com", "h")
	band, _ := repo.CreateBand(alice.ID, "Band")
	return repo, alice, bob, band
}

func TestDirectInviteLifecycle(t *testing.T) {
	repo, alice, bob, band := inviteFixture(t)

	invite, err := repo.CreateDirectInvite(band.ID, bob.ID, model.RoleEditor, alice.ID)
	if err != nil {
		t.Fatalf("CreateDirectInvite: %v", err)
	}

	// Member sees their pending invite with the band name.
	pending, err := repo.PendingInvitesForUser(bob.ID)
	if err != nil || len(pending) != 1 {
		t.Fatalf("PendingInvitesForUser: %v (%v)", pending, err)
	}
	if pending[0].BandName != "Band" || pending[0].Role != model.RoleEditor {
		t.Errorf("pending = %+v", pending[0])
	}

	// Duplicate pending invite rejected; inviting an existing member rejected.
	if _, err := repo.CreateDirectInvite(band.ID, bob.ID, model.RoleEditor, alice.ID); !errors.Is(err, ErrInvitePending) {
		t.Errorf("duplicate invite: %v", err)
	}
	if _, err := repo.CreateDirectInvite(band.ID, alice.ID, model.RoleEditor, alice.ID); !errors.Is(err, ErrAlreadyMember) {
		t.Errorf("invite existing member: %v", err)
	}

	// Accept joins with the invite's role and is single-use.
	bandID, err := repo.AcceptInvite(invite.ID, bob.ID)
	if err != nil || bandID != band.ID {
		t.Fatalf("AcceptInvite: %d, %v", bandID, err)
	}
	role, _ := repo.MemberRole(band.ID, bob.ID)
	if role != model.RoleEditor {
		t.Errorf("role after accept = %q", role)
	}
	if _, err := repo.AcceptInvite(invite.ID, bob.ID); err == nil {
		t.Error("invite accepted twice")
	}
	if pending, _ := repo.PendingInvitesForUser(bob.ID); len(pending) != 0 {
		t.Errorf("pending after accept = %v", pending)
	}
}

func TestAcceptGuards(t *testing.T) {
	repo, alice, bob, band := inviteFixture(t)
	carol, _ := repo.CreateUser("carol", "carol@example.com", "h")
	invite, _ := repo.CreateDirectInvite(band.ID, bob.ID, model.RoleViewer, alice.ID)

	// Only the invited user can accept.
	if _, err := repo.AcceptInvite(invite.ID, carol.ID); err == nil {
		t.Error("wrong user accepted invite")
	}

	// Declined invites cannot be accepted.
	if err := repo.DeclineInvite(invite.ID, bob.ID); err != nil {
		t.Fatalf("DeclineInvite: %v", err)
	}
	if _, err := repo.AcceptInvite(invite.ID, bob.ID); err == nil {
		t.Error("declined invite accepted")
	}

	// Expired invites cannot be accepted.
	invite2, _ := repo.CreateDirectInvite(band.ID, bob.ID, model.RoleViewer, alice.ID)
	repo.db.Model(&model.BandInvite{}).Where("id = ?", invite2.ID).
		Update("expires_at", time.Now().Add(-time.Minute))
	if _, err := repo.AcceptInvite(invite2.ID, bob.ID); err == nil {
		t.Error("expired invite accepted")
	}
}

func TestLinkInviteLifecycle(t *testing.T) {
	repo, alice, bob, band := inviteFixture(t)
	carol, _ := repo.CreateUser("carol", "carol@example.com", "h")

	token, err := repo.CreateLinkInvite(band.ID, model.RoleViewer, alice.ID)
	if err != nil || token == "" {
		t.Fatalf("CreateLinkInvite: %q, %v", token, err)
	}

	// Multi-use: two different users join via the same link.
	if bandID, err := repo.JoinByLink(token, bob.ID); err != nil || bandID != band.ID {
		t.Fatalf("JoinByLink(bob): %d, %v", bandID, err)
	}
	if bandID, err := repo.JoinByLink(token, carol.ID); err != nil || bandID != band.ID {
		t.Fatalf("JoinByLink(carol): %d, %v", bandID, err)
	}
	role, _ := repo.MemberRole(band.ID, carol.ID)
	if role != model.RoleViewer {
		t.Errorf("link role = %q", role)
	}

	// Joining again is idempotent.
	if bandID, err := repo.JoinByLink(token, bob.ID); err != nil || bandID != band.ID {
		t.Errorf("re-join: %d, %v", bandID, err)
	}

	// Bogus tokens rejected.
	if _, err := repo.JoinByLink("bogus", bob.ID); err == nil {
		t.Error("bogus token joined")
	}

	// Revoked links stop working; revoke is band-scoped.
	invites, err := repo.InvitesForBand(band.ID)
	if err != nil || len(invites) != 1 {
		t.Fatalf("InvitesForBand: %v (%v)", invites, err)
	}
	if !invites[0].IsLink {
		t.Errorf("invite not a link: %+v", invites[0])
	}
	if err := repo.RevokeInvite(invites[0].ID, 9999); err == nil {
		t.Error("revoke with wrong band succeeded")
	}
	if err := repo.RevokeInvite(invites[0].ID, band.ID); err != nil {
		t.Fatalf("RevokeInvite: %v", err)
	}
	dave, _ := repo.CreateUser("dave", "dave@example.com", "h")
	if _, err := repo.JoinByLink(token, dave.ID); err == nil {
		t.Error("revoked link joined")
	}
}
```

- [ ] **Step 2: Run `just test` — FAIL.**

- [ ] **Step 3: Write `internal/repository/bandinvites.go`**

```go
package repository

import (
	"errors"
	"time"

	"gorm.io/gorm"

	"github.com/jwhumphries/bandwidth/internal/auth"
	"github.com/jwhumphries/bandwidth/internal/model"
)

// Invite lifetimes.
const (
	directInviteDuration = 14 * 24 * time.Hour
	linkInviteDuration   = 7 * 24 * time.Hour
)

// Invite errors the handler layer maps to HTTP statuses.
var (
	ErrAlreadyMember = errors.New("user is already a member")
	ErrInvitePending = errors.New("an invite for this user is already pending")
)

// PendingInvite is one row of a user's incoming-invite list.
type PendingInvite struct {
	ID       uint           `json:"id"`
	BandID   uint           `json:"bandId"`
	BandName string         `json:"bandName"`
	Role     model.BandRole `json:"role"`
}

// BandInviteInfo is one row of a band's outgoing-invite list (admin view).
type BandInviteInfo struct {
	ID              uint           `json:"id"`
	Role            model.BandRole `json:"role"`
	InvitedUsername *string        `json:"invitedUsername"`
	IsLink          bool           `json:"isLink"`
	ExpiresAt       time.Time      `json:"expiresAt"`
}

// pendingInviteScope filters to usable invites.
func pendingInviteScope(db *gorm.DB) *gorm.DB {
	return db.Where("accepted_at IS NULL AND revoked_at IS NULL AND expires_at > ?", time.Now())
}

// CreateDirectInvite invites an existing user to a band.
func (r *Repo) CreateDirectInvite(bandID, invitedUserID uint, role model.BandRole, createdBy uint) (*model.BandInvite, error) {
	if _, err := r.MemberRole(bandID, invitedUserID); err == nil {
		return nil, ErrAlreadyMember
	}
	var n int64
	err := pendingInviteScope(r.db.Model(&model.BandInvite{})).
		Where("band_id = ? AND invited_user_id = ?", bandID, invitedUserID).
		Count(&n).Error
	if err != nil {
		return nil, err
	}
	if n > 0 {
		return nil, ErrInvitePending
	}
	invite := &model.BandInvite{
		BandID:        bandID,
		Role:          role,
		InvitedUserID: &invitedUserID,
		ExpiresAt:     time.Now().Add(directInviteDuration),
		CreatedBy:     createdBy,
	}
	if err := r.db.Create(invite).Error; err != nil {
		return nil, err
	}
	return invite, nil
}

// CreateLinkInvite creates a multi-use share link and returns its raw token.
func (r *Repo) CreateLinkInvite(bandID uint, role model.BandRole, createdBy uint) (string, error) {
	token := auth.NewToken()
	hash := auth.HashToken(token)
	invite := &model.BandInvite{
		BandID:    bandID,
		Role:      role,
		TokenHash: &hash,
		ExpiresAt: time.Now().Add(linkInviteDuration),
		CreatedBy: createdBy,
	}
	if err := r.db.Create(invite).Error; err != nil {
		return "", err
	}
	return token, nil
}

// PendingInvitesForUser lists a user's incoming pending invites.
func (r *Repo) PendingInvitesForUser(userID uint) ([]PendingInvite, error) {
	pending := []PendingInvite{}
	err := pendingInviteScope(r.db.Table("band_invites")).
		Select("band_invites.id, band_invites.band_id, bands.name AS band_name, band_invites.role").
		Joins("JOIN bands ON bands.id = band_invites.band_id").
		Where("band_invites.invited_user_id = ?", userID).
		Order("band_invites.id").
		Scan(&pending).Error
	if err != nil {
		return nil, err
	}
	return pending, nil
}

// InvitesForBand lists a band's pending invites (admin view).
func (r *Repo) InvitesForBand(bandID uint) ([]BandInviteInfo, error) {
	invites := []BandInviteInfo{}
	err := pendingInviteScope(r.db.Table("band_invites")).
		Select(`band_invites.id, band_invites.role, users.username AS invited_username,
			band_invites.token_hash IS NOT NULL AS is_link, band_invites.expires_at`).
		Joins("LEFT JOIN users ON users.id = band_invites.invited_user_id").
		Where("band_invites.band_id = ?", bandID).
		Order("band_invites.id").
		Scan(&invites).Error
	if err != nil {
		return nil, err
	}
	return invites, nil
}

// RevokeInvite revokes a band's invite (band-scoped).
func (r *Repo) RevokeInvite(inviteID, bandID uint) error {
	res := r.db.Model(&model.BandInvite{}).
		Where("id = ? AND band_id = ? AND revoked_at IS NULL", inviteID, bandID).
		Update("revoked_at", time.Now())
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected != 1 {
		return gorm.ErrRecordNotFound
	}
	return nil
}

// AcceptInvite joins the invited user to the band with the invite's role.
// Single-use; idempotent if the user is somehow already a member.
func (r *Repo) AcceptInvite(inviteID, userID uint) (uint, error) {
	var invite model.BandInvite
	err := pendingInviteScope(r.db).
		Where("id = ? AND invited_user_id = ?", inviteID, userID).
		First(&invite).Error
	if err != nil {
		return 0, err
	}
	err = r.db.Transaction(func(tx *gorm.DB) error {
		now := time.Now()
		if err := tx.Model(&model.BandInvite{}).Where("id = ?", invite.ID).
			Update("accepted_at", now).Error; err != nil {
			return err
		}
		if _, err := r.MemberRole(invite.BandID, userID); err == nil {
			return nil // already a member; just consume the invite
		}
		return tx.Create(&model.BandMember{
			BandID: invite.BandID, UserID: userID, Role: invite.Role,
		}).Error
	})
	if err != nil {
		return 0, err
	}
	return invite.BandID, nil
}

// DeclineInvite marks a user's incoming invite revoked.
func (r *Repo) DeclineInvite(inviteID, userID uint) error {
	res := pendingInviteScope(r.db.Model(&model.BandInvite{})).
		Where("id = ? AND invited_user_id = ?", inviteID, userID).
		Update("revoked_at", time.Now())
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected != 1 {
		return gorm.ErrRecordNotFound
	}
	return nil
}

// JoinByLink adds the user to the link's band (multi-use, idempotent).
func (r *Repo) JoinByLink(token string, userID uint) (uint, error) {
	hash := auth.HashToken(token)
	var invite model.BandInvite
	err := pendingInviteScope(r.db).
		Where("token_hash = ?", hash).
		First(&invite).Error
	if err != nil {
		return 0, err
	}
	if _, err := r.MemberRole(invite.BandID, userID); err == nil {
		return invite.BandID, nil
	}
	if err := r.AddMember(invite.BandID, userID, invite.Role); err != nil {
		return 0, err
	}
	return invite.BandID, nil
}
```

NOTE: `token_hash IS NOT NULL AS is_link` scans into a Go bool via SQLite's 0/1 — if GORM's Scan misreads it, change the struct field to `IsLink bool` backed by a `CASE WHEN token_hash IS NOT NULL THEN 1 ELSE 0 END AS is_link` select; the test is the arbiter. Report any such adaptation.

- [ ] **Step 4: Run `just test` + `just lint-go` (green/clean), commit**

```bash
git add internal/repository/
git commit -m "feat: band invite repository"
```

---

### Task 4: Band handlers (list/create/detail/rename/delete) + access helper

**Files:**
- Create: `internal/handlers/bands.go`
- Test: `internal/handlers/bands_test.go`

- [ ] **Step 1: Write the failing tests** — `internal/handlers/bands_test.go`:

```go
package handlers

import (
	"encoding/json"
	"fmt"
	"net/http"
	"testing"

	"github.com/labstack/echo/v5"

	appmw "github.com/jwhumphries/bandwidth/internal/middleware"
	"github.com/jwhumphries/bandwidth/internal/model"
)

func newBandsAPI(t *testing.T) (*echo.Echo, *API) {
	t.Helper()
	e, api := newTestAPI(t)
	g := e.Group("/api/bands", appmw.RequireAuth(api.Repo))
	g.GET("", api.Bands)
	g.POST("", api.CreateBand)
	g.GET("/:id", api.Band)
	g.PATCH("/:id", api.RenameBand)
	g.DELETE("/:id", api.DeleteBand)
	return e, api
}

// createBandFor creates a band via the API and returns its id.
func createBandFor(t *testing.T, e *echo.Echo, cookie *http.Cookie, name string) uint {
	t.Helper()
	rec := jsonReq(e, http.MethodPost, "/api/bands",
		fmt.Sprintf(`{"name":%q}`, name), cookie)
	if rec.Code != http.StatusCreated {
		t.Fatalf("create band: %d %s", rec.Code, rec.Body.String())
	}
	var created struct {
		ID uint `json:"id"`
	}
	_ = json.Unmarshal(rec.Body.Bytes(), &created)
	return created.ID
}

func TestBandCRUD(t *testing.T) {
	e, api := newBandsAPI(t)
	alice := signupAndCookie(t, e, "alice")
	bob := signupAndCookie(t, e, "bob")

	id := createBandFor(t, e, alice, "The Quietones")

	// Blank name rejected.
	if rec := jsonReq(e, http.MethodPost, "/api/bands", `{"name":"  "}`, alice); rec.Code != http.StatusBadRequest {
		t.Fatalf("blank name: %d, want 400", rec.Code)
	}

	// List shows role + member count.
	rec := jsonReq(e, http.MethodGet, "/api/bands", "", alice)
	var list []struct {
		Name        string `json:"name"`
		Role        string `json:"role"`
		MemberCount int    `json:"memberCount"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &list); err != nil || len(list) != 1 {
		t.Fatalf("bands list: %s (%v)", rec.Body.String(), err)
	}
	if list[0].Role != "admin" || list[0].MemberCount != 1 {
		t.Errorf("summary = %+v", list[0])
	}

	// Detail shows members and my role.
	rec = jsonReq(e, http.MethodGet, fmt.Sprintf("/api/bands/%d", id), "", alice)
	if rec.Code != http.StatusOK {
		t.Fatalf("detail: %d %s", rec.Code, rec.Body.String())
	}
	var detail struct {
		Name      string `json:"name"`
		MyRole    string `json:"myRole"`
		CreatorID uint   `json:"creatorId"`
		Members   []struct {
			Username string `json:"username"`
			Role     string `json:"role"`
		} `json:"members"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &detail); err != nil {
		t.Fatal(err)
	}
	if detail.MyRole != "admin" || len(detail.Members) != 1 || detail.Members[0].Username != "alice" {
		t.Errorf("detail = %+v", detail)
	}

	// Non-members get 404s everywhere.
	for _, tc := range []struct{ method, path, body string }{
		{http.MethodGet, fmt.Sprintf("/api/bands/%d", id), ""},
		{http.MethodPatch, fmt.Sprintf("/api/bands/%d", id), `{"name":"X"}`},
		{http.MethodDelete, fmt.Sprintf("/api/bands/%d", id), ""},
	} {
		if rec := jsonReq(e, tc.method, tc.path, tc.body, bob); rec.Code != http.StatusNotFound {
			t.Errorf("%s %s as bob: %d, want 404", tc.method, tc.path, rec.Code)
		}
	}

	// Rename (admin) works.
	rec = jsonReq(e, http.MethodPatch, fmt.Sprintf("/api/bands/%d", id), `{"name":"Loudones"}`, alice)
	if rec.Code != http.StatusNoContent {
		t.Fatalf("rename: %d %s", rec.Code, rec.Body.String())
	}

	// Non-admin members get 403 on admin actions.
	if err := api.Repo.AddMember(id, mustUserID(t, api, "bob"), model.RoleEditor); err != nil {
		t.Fatal(err)
	}
	rec = jsonReq(e, http.MethodPatch, fmt.Sprintf("/api/bands/%d", id), `{"name":"Nope"}`, bob)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("editor rename: %d, want 403", rec.Code)
	}
	// Non-creator cannot delete, even an admin.
	_ = api.Repo.SetMemberRole(id, mustUserID(t, api, "bob"), model.RoleAdmin)
	rec = jsonReq(e, http.MethodDelete, fmt.Sprintf("/api/bands/%d", id), "", bob)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("non-creator delete: %d, want 403", rec.Code)
	}

	// Creator deletes.
	rec = jsonReq(e, http.MethodDelete, fmt.Sprintf("/api/bands/%d", id), "", alice)
	if rec.Code != http.StatusNoContent {
		t.Fatalf("delete: %d", rec.Code)
	}
}

// mustUserID looks a user up by username through the repo.
func mustUserID(t *testing.T, api *API, username string) uint {
	t.Helper()
	user, err := api.Repo.UserByLogin(username)
	if err != nil {
		t.Fatalf("UserByLogin(%s): %v", username, err)
	}
	return user.ID
}
```

- [ ] **Step 2: Run `just test` — FAIL.**

- [ ] **Step 3: Write `internal/handlers/bands.go`**

```go
package handlers

import (
	"net/http"
	"strings"

	"github.com/labstack/echo/v5"

	appmw "github.com/jwhumphries/bandwidth/internal/middleware"
	"github.com/jwhumphries/bandwidth/internal/model"
)

// bandAccess parses the :id band, loads the caller's role, and enforces a
// minimum. Non-members get 404 (bands are invisible to outsiders);
// insufficient role gets 403.
func (a *API) bandAccess(c *echo.Context, min model.BandRole) (uint, model.BandRole, error) {
	user := appmw.CurrentUser(c)
	if user == nil {
		return 0, "", echo.NewHTTPError(http.StatusInternalServerError, "user not in context")
	}
	id, err := songID(c) // shared uint :id parser
	if err != nil {
		return 0, "", echo.NewHTTPError(http.StatusNotFound, "band not found")
	}
	role, err := a.Repo.MemberRole(id, user.ID)
	if err != nil {
		return 0, "", echo.NewHTTPError(http.StatusNotFound, "band not found")
	}
	if !role.AtLeast(min) {
		return 0, "", echo.NewHTTPError(http.StatusForbidden, "insufficient role")
	}
	return id, role, nil
}

// Bands lists the caller's bands.
func (a *API) Bands(c *echo.Context) error {
	user := appmw.CurrentUser(c)
	if user == nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "user not in context")
	}
	summaries, err := a.Repo.BandsForUser(user.ID)
	if err != nil {
		return err
	}
	return c.JSON(http.StatusOK, summaries)
}

type bandNameRequest struct {
	Name string `json:"name"`
}

// CreateBand creates a band with the caller as permanent Admin.
func (a *API) CreateBand(c *echo.Context) error {
	user := appmw.CurrentUser(c)
	if user == nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "user not in context")
	}
	var req bandNameRequest
	if err := c.Bind(&req); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid request body")
	}
	req.Name = strings.TrimSpace(req.Name)
	if req.Name == "" || len(req.Name) > maxTitleLen {
		return echo.NewHTTPError(http.StatusBadRequest, "a band name is required")
	}
	band, err := a.Repo.CreateBand(user.ID, req.Name)
	if err != nil {
		return err
	}
	return c.JSON(http.StatusCreated, map[string]any{
		"id": band.ID, "name": band.Name, "creatorId": band.CreatorID,
	})
}

// Band returns band detail with the roster (any member).
func (a *API) Band(c *echo.Context) error {
	id, role, err := a.bandAccess(c, model.RoleViewer)
	if err != nil {
		return err
	}
	band, err := a.Repo.BandByID(id)
	if err != nil {
		return notFoundOr(err, "band")
	}
	members, err := a.Repo.MembersForBand(id)
	if err != nil {
		return err
	}
	return c.JSON(http.StatusOK, map[string]any{
		"id":        band.ID,
		"name":      band.Name,
		"creatorId": band.CreatorID,
		"myRole":    role,
		"members":   members,
	})
}

// RenameBand renames the band (admin).
func (a *API) RenameBand(c *echo.Context) error {
	id, _, err := a.bandAccess(c, model.RoleAdmin)
	if err != nil {
		return err
	}
	var req bandNameRequest
	if err := c.Bind(&req); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid request body")
	}
	req.Name = strings.TrimSpace(req.Name)
	if req.Name == "" || len(req.Name) > maxTitleLen {
		return echo.NewHTTPError(http.StatusBadRequest, "a band name is required")
	}
	if err := a.Repo.RenameBand(id, req.Name); err != nil {
		return err
	}
	return c.NoContent(http.StatusNoContent)
}

// DeleteBand deletes the band (creator only).
func (a *API) DeleteBand(c *echo.Context) error {
	id, _, err := a.bandAccess(c, model.RoleAdmin)
	if err != nil {
		return err
	}
	user := appmw.CurrentUser(c)
	band, err := a.Repo.BandByID(id)
	if err != nil {
		return notFoundOr(err, "band")
	}
	if band.CreatorID != user.ID {
		return echo.NewHTTPError(http.StatusForbidden, "only the band creator can delete the band")
	}
	if err := a.Repo.DeleteBand(id); err != nil {
		return err
	}
	return c.NoContent(http.StatusNoContent)
}
```

- [ ] **Step 4: Run `just test` + `just lint-go` (green/clean), commit**

```bash
git add internal/handlers/
git commit -m "feat: band handlers and role access helper"
```

---

### Task 5: Member handlers (role change, remove/leave)

**Files:**
- Create: `internal/handlers/bandmembers.go`
- Test: `internal/handlers/bandmembers_test.go`

- [ ] **Step 1: Write the failing tests** — `internal/handlers/bandmembers_test.go`:

```go
package handlers

import (
	"fmt"
	"net/http"
	"testing"

	"github.com/labstack/echo/v5"

	appmw "github.com/jwhumphries/bandwidth/internal/middleware"
	"github.com/jwhumphries/bandwidth/internal/model"
)

func newMembersAPI(t *testing.T) (*echo.Echo, *API) {
	t.Helper()
	e, api := newBandsAPI(t)
	g := e.Group("/api/bands", appmw.RequireAuth(api.Repo))
	g.PATCH("/:id/members/:userId", api.SetMemberRole)
	g.DELETE("/:id/members/:userId", api.RemoveMember)
	return e, api
}

func TestMemberManagement(t *testing.T) {
	e, api := newMembersAPI(t)
	alice := signupAndCookie(t, e, "alice") // creator/admin
	bob := signupAndCookie(t, e, "bob")     // member
	_ = signupAndCookie(t, e, "carol")      // member

	band := createBandFor(t, e, alice, "Band")
	bobID := mustUserID(t, api, "bob")
	carolID := mustUserID(t, api, "carol")
	aliceID := mustUserID(t, api, "alice")
	_ = api.Repo.AddMember(band, bobID, model.RoleEditor)
	_ = api.Repo.AddMember(band, carolID, model.RoleEditor)

	memberPath := func(uid uint) string {
		return fmt.Sprintf("/api/bands/%d/members/%d", band, uid)
	}

	// Admin changes a role.
	rec := jsonReq(e, http.MethodPatch, memberPath(bobID), `{"role":"viewer"}`, alice)
	if rec.Code != http.StatusNoContent {
		t.Fatalf("set role: %d %s", rec.Code, rec.Body.String())
	}
	role, _ := api.Repo.MemberRole(band, bobID)
	if role != model.RoleViewer {
		t.Errorf("role = %q", role)
	}

	// Invalid role rejected.
	if rec := jsonReq(e, http.MethodPatch, memberPath(bobID), `{"role":"roadie"}`, alice); rec.Code != http.StatusBadRequest {
		t.Fatalf("bad role: %d, want 400", rec.Code)
	}

	// Creator's role is immutable; creator cannot be removed.
	if rec := jsonReq(e, http.MethodPatch, memberPath(aliceID), `{"role":"viewer"}`, alice); rec.Code != http.StatusBadRequest {
		t.Fatalf("demote creator: %d, want 400", rec.Code)
	}
	if rec := jsonReq(e, http.MethodDelete, memberPath(aliceID), "", alice); rec.Code != http.StatusBadRequest {
		t.Fatalf("remove creator: %d, want 400", rec.Code)
	}

	// Non-admins cannot manage others...
	if rec := jsonReq(e, http.MethodPatch, memberPath(carolID), `{"role":"viewer"}`, bob); rec.Code != http.StatusForbidden {
		t.Fatalf("member sets role: %d, want 403", rec.Code)
	}
	if rec := jsonReq(e, http.MethodDelete, memberPath(carolID), "", bob); rec.Code != http.StatusForbidden {
		t.Fatalf("member removes other: %d, want 403", rec.Code)
	}
	// ...but can remove THEMSELVES (leave).
	if rec := jsonReq(e, http.MethodDelete, memberPath(bobID), "", bob); rec.Code != http.StatusNoContent {
		t.Fatalf("leave: %d", rec.Code)
	}
	if _, err := api.Repo.MemberRole(band, bobID); err == nil {
		t.Error("bob still a member after leaving")
	}

	// Admin removes a member.
	if rec := jsonReq(e, http.MethodDelete, memberPath(carolID), "", alice); rec.Code != http.StatusNoContent {
		t.Fatalf("admin remove: %d", rec.Code)
	}
}
```

- [ ] **Step 2: Run `just test` — FAIL.**

- [ ] **Step 3: Write `internal/handlers/bandmembers.go`**

```go
package handlers

import (
	"net/http"
	"strconv"

	"github.com/labstack/echo/v5"

	appmw "github.com/jwhumphries/bandwidth/internal/middleware"
	"github.com/jwhumphries/bandwidth/internal/model"
)

func memberID(c *echo.Context) (uint, error) {
	id, err := strconv.ParseUint(c.Param("userId"), 10, 32)
	if err != nil || id == 0 {
		return 0, echo.NewHTTPError(http.StatusNotFound, "member not found")
	}
	return uint(id), nil
}

type setRoleRequest struct {
	Role string `json:"role"`
}

// SetMemberRole changes a member's role (admin). The creator is immutable.
func (a *API) SetMemberRole(c *echo.Context) error {
	bandID, _, err := a.bandAccess(c, model.RoleAdmin)
	if err != nil {
		return err
	}
	targetID, err := memberID(c)
	if err != nil {
		return err
	}
	var req setRoleRequest
	if err := c.Bind(&req); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid request body")
	}
	role := model.BandRole(req.Role)
	if !role.Valid() {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid role")
	}
	band, err := a.Repo.BandByID(bandID)
	if err != nil {
		return notFoundOr(err, "band")
	}
	if band.CreatorID == targetID {
		return echo.NewHTTPError(http.StatusBadRequest, "the band creator is always an admin")
	}
	if err := a.Repo.SetMemberRole(bandID, targetID, role); err != nil {
		return notFoundOr(err, "member")
	}
	return c.NoContent(http.StatusNoContent)
}

// RemoveMember removes a member (admin), or lets a member remove
// themselves (leave). The creator can do neither — they delete the band.
func (a *API) RemoveMember(c *echo.Context) error {
	bandID, role, err := a.bandAccess(c, model.RoleViewer)
	if err != nil {
		return err
	}
	user := appmw.CurrentUser(c)
	targetID, err := memberID(c)
	if err != nil {
		return err
	}
	if targetID != user.ID && !role.AtLeast(model.RoleAdmin) {
		return echo.NewHTTPError(http.StatusForbidden, "insufficient role")
	}
	band, err := a.Repo.BandByID(bandID)
	if err != nil {
		return notFoundOr(err, "band")
	}
	if band.CreatorID == targetID {
		return echo.NewHTTPError(http.StatusBadRequest,
			"the creator cannot leave or be removed; delete the band instead")
	}
	if err := a.Repo.RemoveMember(bandID, targetID); err != nil {
		return notFoundOr(err, "member")
	}
	return c.NoContent(http.StatusNoContent)
}
```

- [ ] **Step 4: Run `just test` + `just lint-go` (green/clean), commit**

```bash
git add internal/handlers/
git commit -m "feat: member role and removal handlers"
```

---

### Task 6: Invite handlers

**Files:**
- Create: `internal/handlers/bandinvites.go`
- Test: `internal/handlers/bandinvites_test.go`

- [ ] **Step 1: Write the failing tests** — `internal/handlers/bandinvites_test.go`:

```go
package handlers

import (
	"encoding/json"
	"fmt"
	"net/http"
	"testing"

	"github.com/labstack/echo/v5"

	appmw "github.com/jwhumphries/bandwidth/internal/middleware"
	"github.com/jwhumphries/bandwidth/internal/model"
)

func newInvitesAPI(t *testing.T) (*echo.Echo, *API) {
	t.Helper()
	e, api := newBandsAPI(t)
	g := e.Group("/api/bands", appmw.RequireAuth(api.Repo))
	g.GET("/:id/invites", api.BandInvites)
	g.POST("/:id/invites", api.CreateInvite)
	g.DELETE("/:id/invites/:inviteId", api.RevokeInvite)
	inv := e.Group("/api/invites", appmw.RequireAuth(api.Repo))
	inv.GET("", api.MyInvites)
	inv.POST("/:id/accept", api.AcceptInvite)
	inv.POST("/:id/decline", api.DeclineInvite)
	inv.POST("/link/:token", api.JoinByLink)
	return e, api
}

func TestDirectInviteFlow(t *testing.T) {
	e, api := newInvitesAPI(t)
	alice := signupAndCookie(t, e, "alice")
	bob := signupAndCookie(t, e, "bob")
	band := createBandFor(t, e, alice, "Band")

	// Invite bob by username, default role editor.
	rec := jsonReq(e, http.MethodPost, fmt.Sprintf("/api/bands/%d/invites", band),
		`{"username":"bob"}`, alice)
	if rec.Code != http.StatusCreated {
		t.Fatalf("invite: %d %s", rec.Code, rec.Body.String())
	}

	// Unknown user → 404; existing member → 409; duplicate pending → 409.
	if rec := jsonReq(e, http.MethodPost, fmt.Sprintf("/api/bands/%d/invites", band),
		`{"username":"nobody"}`, alice); rec.Code != http.StatusNotFound {
		t.Fatalf("unknown user: %d, want 404", rec.Code)
	}
	if rec := jsonReq(e, http.MethodPost, fmt.Sprintf("/api/bands/%d/invites", band),
		`{"username":"alice"}`, alice); rec.Code != http.StatusConflict {
		t.Fatalf("invite member: %d, want 409", rec.Code)
	}
	if rec := jsonReq(e, http.MethodPost, fmt.Sprintf("/api/bands/%d/invites", band),
		`{"username":"bob"}`, alice); rec.Code != http.StatusConflict {
		t.Fatalf("duplicate invite: %d, want 409", rec.Code)
	}

	// Non-admins cannot invite.
	if rec := jsonReq(e, http.MethodPost, fmt.Sprintf("/api/bands/%d/invites", band),
		`{"username":"carol"}`, bob); rec.Code != http.StatusNotFound {
		t.Fatalf("non-member invites: %d, want 404", rec.Code)
	}

	// Bob sees and accepts the invite.
	rec = jsonReq(e, http.MethodGet, "/api/invites", "", bob)
	var pending []struct {
		ID       uint   `json:"id"`
		BandName string `json:"bandName"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &pending); err != nil || len(pending) != 1 {
		t.Fatalf("my invites: %s (%v)", rec.Body.String(), err)
	}
	rec = jsonReq(e, http.MethodPost, fmt.Sprintf("/api/invites/%d/accept", pending[0].ID), "", bob)
	if rec.Code != http.StatusOK {
		t.Fatalf("accept: %d %s", rec.Code, rec.Body.String())
	}
	role, err := api.Repo.MemberRole(band, mustUserID(t, api, "bob"))
	if err != nil || role != model.RoleEditor {
		t.Fatalf("bob role = %q, %v", role, err)
	}

	// Accepting someone else's invite 404s.
	carol := signupAndCookie(t, e, "carol")
	rec = jsonReq(e, http.MethodPost, fmt.Sprintf("/api/bands/%d/invites", band),
		`{"username":"carol","role":"viewer"}`, alice)
	if rec.Code != http.StatusCreated {
		t.Fatal(rec.Code)
	}
	var carolInvite struct {
		ID uint `json:"id"`
	}
	_ = json.Unmarshal(rec.Body.Bytes(), &carolInvite)
	if rec := jsonReq(e, http.MethodPost, fmt.Sprintf("/api/invites/%d/accept", carolInvite.ID), "", bob); rec.Code != http.StatusNotFound {
		t.Fatalf("accept other's invite: %d, want 404", rec.Code)
	}
	// Decline.
	if rec := jsonReq(e, http.MethodPost, fmt.Sprintf("/api/invites/%d/decline", carolInvite.ID), "", carol); rec.Code != http.StatusNoContent {
		t.Fatalf("decline: %d", rec.Code)
	}
}

func TestLinkInviteFlow(t *testing.T) {
	e, api := newInvitesAPI(t)
	alice := signupAndCookie(t, e, "alice")
	bob := signupAndCookie(t, e, "bob")
	band := createBandFor(t, e, alice, "Band")

	// Create a link (admin only) with a role.
	rec := jsonReq(e, http.MethodPost, fmt.Sprintf("/api/bands/%d/invites", band),
		`{"link":true,"role":"viewer"}`, alice)
	if rec.Code != http.StatusCreated {
		t.Fatalf("create link: %d %s", rec.Code, rec.Body.String())
	}
	var link struct {
		ID    uint   `json:"id"`
		Token string `json:"token"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &link); err != nil || link.Token == "" {
		t.Fatalf("link body: %s (%v)", rec.Body.String(), err)
	}

	// Join via the link.
	rec = jsonReq(e, http.MethodPost, "/api/invites/link/"+link.Token, "", bob)
	if rec.Code != http.StatusOK {
		t.Fatalf("join: %d %s", rec.Code, rec.Body.String())
	}
	role, _ := api.Repo.MemberRole(band, mustUserID(t, api, "bob"))
	if role != model.RoleViewer {
		t.Errorf("joined role = %q", role)
	}

	// Bad token 404s.
	if rec := jsonReq(e, http.MethodPost, "/api/invites/link/bogus", "", bob); rec.Code != http.StatusNotFound {
		t.Fatalf("bogus token: %d, want 404", rec.Code)
	}

	// Admin lists and revokes; revoked link stops working.
	rec = jsonReq(e, http.MethodGet, fmt.Sprintf("/api/bands/%d/invites", band), "", alice)
	var invites []struct {
		ID     uint `json:"id"`
		IsLink bool `json:"isLink"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &invites); err != nil || len(invites) != 1 || !invites[0].IsLink {
		t.Fatalf("band invites: %s (%v)", rec.Body.String(), err)
	}
	rec = jsonReq(e, http.MethodDelete,
		fmt.Sprintf("/api/bands/%d/invites/%d", band, invites[0].ID), "", alice)
	if rec.Code != http.StatusNoContent {
		t.Fatalf("revoke: %d", rec.Code)
	}
	carol := signupAndCookie(t, e, "carol")
	if rec := jsonReq(e, http.MethodPost, "/api/invites/link/"+link.Token, "", carol); rec.Code != http.StatusNotFound {
		t.Fatalf("revoked link join: %d, want 404", rec.Code)
	}
}
```

- [ ] **Step 2: Run `just test` — FAIL.**

- [ ] **Step 3: Write `internal/handlers/bandinvites.go`**

```go
package handlers

import (
	"errors"
	"net/http"
	"strconv"
	"strings"

	"github.com/labstack/echo/v5"

	appmw "github.com/jwhumphries/bandwidth/internal/middleware"
	"github.com/jwhumphries/bandwidth/internal/model"
	"github.com/jwhumphries/bandwidth/internal/repository"
)

func inviteID(c *echo.Context) (uint, error) {
	param := c.Param("inviteId")
	if param == "" {
		param = c.Param("id")
	}
	id, err := strconv.ParseUint(param, 10, 32)
	if err != nil || id == 0 {
		return 0, echo.NewHTTPError(http.StatusNotFound, "invite not found")
	}
	return uint(id), nil
}

type createInviteRequest struct {
	Username string `json:"username"`
	Role     string `json:"role"`
	Link     bool   `json:"link"`
}

// CreateInvite creates a direct invite (by username/email) or a share link
// (admin). The raw link token is returned exactly once.
func (a *API) CreateInvite(c *echo.Context) error {
	bandID, _, err := a.bandAccess(c, model.RoleAdmin)
	if err != nil {
		return err
	}
	user := appmw.CurrentUser(c)
	var req createInviteRequest
	if err := c.Bind(&req); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid request body")
	}
	role := model.RoleEditor
	if req.Role != "" {
		role = model.BandRole(req.Role)
		if !role.Valid() {
			return echo.NewHTTPError(http.StatusBadRequest, "invalid role")
		}
	}

	if req.Link {
		token, err := a.Repo.CreateLinkInvite(bandID, role, user.ID)
		if err != nil {
			return err
		}
		return c.JSON(http.StatusCreated, map[string]any{
			"role": role, "token": token, "isLink": true,
		})
	}

	username := strings.TrimSpace(req.Username)
	if username == "" {
		return echo.NewHTTPError(http.StatusBadRequest, "a username or email is required")
	}
	invitee, err := a.Repo.UserByLogin(username)
	if err != nil {
		return echo.NewHTTPError(http.StatusNotFound, "no such user")
	}
	invite, err := a.Repo.CreateDirectInvite(bandID, invitee.ID, role, user.ID)
	switch {
	case errors.Is(err, repository.ErrAlreadyMember):
		return echo.NewHTTPError(http.StatusConflict, "already a member")
	case errors.Is(err, repository.ErrInvitePending):
		return echo.NewHTTPError(http.StatusConflict, "an invite is already pending")
	case err != nil:
		return err
	}
	return c.JSON(http.StatusCreated, map[string]any{
		"id": invite.ID, "role": invite.Role, "invitedUsername": invitee.Username,
	})
}

// BandInvites lists a band's pending invites (admin).
func (a *API) BandInvites(c *echo.Context) error {
	bandID, _, err := a.bandAccess(c, model.RoleAdmin)
	if err != nil {
		return err
	}
	invites, err := a.Repo.InvitesForBand(bandID)
	if err != nil {
		return err
	}
	return c.JSON(http.StatusOK, invites)
}

// RevokeInvite revokes a pending invite (admin).
func (a *API) RevokeInvite(c *echo.Context) error {
	bandID, _, err := a.bandAccess(c, model.RoleAdmin)
	if err != nil {
		return err
	}
	id, err := inviteID(c)
	if err != nil {
		return err
	}
	if err := a.Repo.RevokeInvite(id, bandID); err != nil {
		return notFoundOr(err, "invite")
	}
	return c.NoContent(http.StatusNoContent)
}

// MyInvites lists the caller's pending invites.
func (a *API) MyInvites(c *echo.Context) error {
	user := appmw.CurrentUser(c)
	if user == nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "user not in context")
	}
	pending, err := a.Repo.PendingInvitesForUser(user.ID)
	if err != nil {
		return err
	}
	return c.JSON(http.StatusOK, pending)
}

// AcceptInvite accepts a direct invite and returns the joined band's id.
func (a *API) AcceptInvite(c *echo.Context) error {
	user := appmw.CurrentUser(c)
	if user == nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "user not in context")
	}
	id, err := inviteID(c)
	if err != nil {
		return err
	}
	bandID, err := a.Repo.AcceptInvite(id, user.ID)
	if err != nil {
		return notFoundOr(err, "invite")
	}
	return c.JSON(http.StatusOK, map[string]any{"bandId": bandID})
}

// DeclineInvite declines a direct invite.
func (a *API) DeclineInvite(c *echo.Context) error {
	user := appmw.CurrentUser(c)
	if user == nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "user not in context")
	}
	id, err := inviteID(c)
	if err != nil {
		return err
	}
	if err := a.Repo.DeclineInvite(id, user.ID); err != nil {
		return notFoundOr(err, "invite")
	}
	return c.NoContent(http.StatusNoContent)
}

// JoinByLink joins a band via a share-link token.
func (a *API) JoinByLink(c *echo.Context) error {
	user := appmw.CurrentUser(c)
	if user == nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "user not in context")
	}
	token := c.Param("token")
	if token == "" {
		return echo.NewHTTPError(http.StatusNotFound, "invite not found")
	}
	bandID, err := a.Repo.JoinByLink(token, user.ID)
	if err != nil {
		return notFoundOr(err, "invite")
	}
	return c.JSON(http.StatusOK, map[string]any{"bandId": bandID})
}
```

- [ ] **Step 4: Run `just test` + `just lint-go` (green/clean), commit**

```bash
git add internal/handlers/
git commit -m "feat: invite handlers"
```

---

### Task 7: Route wiring + integration test

**Files:**
- Modify: `cmd/bandwidth/server.go`, `cmd/bandwidth/server_test.go`

- [ ] **Step 1: Failing integration test** — append to `cmd/bandwidth/server_test.go`:

```go
func TestBandLifecycleFlow(t *testing.T) {
	e := testServer(t)

	rec := do(e, http.MethodPost, "/api/auth/signup",
		`{"username":"alice","email":"alice@example.com","password":"hunter2hunter2"}`, nil)
	alice := rec.Result().Cookies()

	rec = do(e, http.MethodPost, "/api/bands", `{"name":"The Quietones"}`, alice)
	if rec.Code != http.StatusCreated {
		t.Fatalf("create band: %d %s", rec.Code, rec.Body.String())
	}

	rec = do(e, http.MethodGet, "/api/bands", "", alice)
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), "Quietones") {
		t.Fatalf("bands list: %d %s", rec.Code, rec.Body.String())
	}

	rec = do(e, http.MethodGet, "/api/invites", "", alice)
	if rec.Code != http.StatusOK {
		t.Fatalf("my invites: %d", rec.Code)
	}
}
```

Run: `just test`. Expected: FAIL — routes not wired.

- [ ] **Step 2: Wire routes** in `cmd/bandwidth/server.go` `newEcho`, after the folders group:

```go
	bands := apiGroup.Group("/bands", appmw.RequireAuth(api.Repo))
	bands.GET("", api.Bands)
	bands.POST("", api.CreateBand)
	bands.GET("/:id", api.Band)
	bands.PATCH("/:id", api.RenameBand)
	bands.DELETE("/:id", api.DeleteBand)
	bands.PATCH("/:id/members/:userId", api.SetMemberRole)
	bands.DELETE("/:id/members/:userId", api.RemoveMember)
	bands.GET("/:id/invites", api.BandInvites)
	bands.POST("/:id/invites", api.CreateInvite)
	bands.DELETE("/:id/invites/:inviteId", api.RevokeInvite)

	invites := apiGroup.Group("/invites", appmw.RequireAuth(api.Repo))
	invites.GET("", api.MyInvites)
	invites.POST("/:id/accept", api.AcceptInvite)
	invites.POST("/:id/decline", api.DeclineInvite)
	invites.POST("/link/:token", api.JoinByLink)
```

- [ ] **Step 3: `just test` green, then full gate `just check` → "all checks passed". Commit:**

```bash
git add cmd/
git commit -m "feat: wire band and invite routes"
```

---

### Task 8: Frontend types + hooks

**Files:**
- Modify: `frontend/src/lib/types.ts` (append)
- Create: `frontend/src/hooks/bands.ts`, `frontend/src/hooks/invites.ts`

- [ ] **Step 1: Append to `frontend/src/lib/types.ts`**

```ts
export type BandRole = 'viewer' | 'editor' | 'admin';

export interface BandSummary {
  id: number;
  name: string;
  role: BandRole;
  memberCount: number;
}

export interface BandMemberInfo {
  userId: number;
  username: string;
  role: BandRole;
}

export interface BandDetail {
  id: number;
  name: string;
  creatorId: number;
  myRole: BandRole;
  members: BandMemberInfo[];
}

export interface MyInvite {
  id: number;
  bandId: number;
  bandName: string;
  role: BandRole;
}

export interface BandInviteInfo {
  id: number;
  role: BandRole;
  invitedUsername: string | null;
  isLink: boolean;
  expiresAt: string;
}

export interface CreatedInvite {
  id?: number;
  role: BandRole;
  invitedUsername?: string;
  token?: string;
  isLink?: boolean;
}
```

- [ ] **Step 2: Write `frontend/src/hooks/bands.ts`**

```ts
import {useMutation, useQuery, useQueryClient} from '@tanstack/react-query';
import {api, ApiError} from '../lib/api';
import type {
  BandDetail,
  BandInviteInfo,
  BandRole,
  BandSummary,
  CreatedInvite,
} from '../lib/types';

export function useBands() {
  return useQuery<BandSummary[], ApiError>({
    queryKey: ['bands'],
    queryFn: () => api.get<BandSummary[]>('/api/bands'),
  });
}

export function useBand(id: number) {
  return useQuery<BandDetail, ApiError>({
    queryKey: ['bands', id],
    queryFn: () => api.get<BandDetail>(`/api/bands/${id}`),
  });
}

export function useCreateBand() {
  const queryClient = useQueryClient();
  return useMutation<{id: number}, ApiError, {name: string}>({
    mutationFn: data => api.post<{id: number}>('/api/bands', data),
    onSuccess: () => void queryClient.invalidateQueries({queryKey: ['bands']}),
  });
}

export function useRenameBand(id: number) {
  const queryClient = useQueryClient();
  return useMutation<void, ApiError, {name: string}>({
    mutationFn: data => api.patch<void>(`/api/bands/${id}`, data),
    onSuccess: () => void queryClient.invalidateQueries({queryKey: ['bands']}),
  });
}

export function useDeleteBand() {
  const queryClient = useQueryClient();
  return useMutation<void, ApiError, number>({
    mutationFn: id => api.delete(`/api/bands/${id}`),
    onSuccess: () => void queryClient.invalidateQueries({queryKey: ['bands']}),
  });
}

export function useSetMemberRole(bandId: number) {
  const queryClient = useQueryClient();
  return useMutation<void, ApiError, {userId: number; role: BandRole}>({
    mutationFn: ({userId, role}) =>
      api.patch<void>(`/api/bands/${bandId}/members/${userId}`, {role}),
    onSuccess: () =>
      void queryClient.invalidateQueries({queryKey: ['bands', bandId]}),
  });
}

export function useRemoveMember(bandId: number) {
  const queryClient = useQueryClient();
  return useMutation<void, ApiError, number>({
    mutationFn: userId => api.delete(`/api/bands/${bandId}/members/${userId}`),
    onSuccess: () => void queryClient.invalidateQueries({queryKey: ['bands']}),
  });
}

export function useBandInvites(bandId: number, enabled: boolean) {
  return useQuery<BandInviteInfo[], ApiError>({
    queryKey: ['bands', bandId, 'invites'],
    queryFn: () => api.get<BandInviteInfo[]>(`/api/bands/${bandId}/invites`),
    enabled,
  });
}

export function useCreateInvite(bandId: number) {
  const queryClient = useQueryClient();
  return useMutation<
    CreatedInvite,
    ApiError,
    {username?: string; role?: BandRole; link?: boolean}
  >({
    mutationFn: data => api.post<CreatedInvite>(`/api/bands/${bandId}/invites`, data),
    onSuccess: () =>
      void queryClient.invalidateQueries({queryKey: ['bands', bandId, 'invites']}),
  });
}

export function useRevokeInvite(bandId: number) {
  const queryClient = useQueryClient();
  return useMutation<void, ApiError, number>({
    mutationFn: inviteId => api.delete(`/api/bands/${bandId}/invites/${inviteId}`),
    onSuccess: () =>
      void queryClient.invalidateQueries({queryKey: ['bands', bandId, 'invites']}),
  });
}
```

- [ ] **Step 3: Write `frontend/src/hooks/invites.ts`**

```ts
import {useMutation, useQuery, useQueryClient} from '@tanstack/react-query';
import {api, ApiError} from '../lib/api';
import type {MyInvite} from '../lib/types';

export function useMyInvites() {
  return useQuery<MyInvite[], ApiError>({
    queryKey: ['my-invites'],
    queryFn: () => api.get<MyInvite[]>('/api/invites'),
    staleTime: 60 * 1000,
  });
}

function useInviteResolution() {
  const queryClient = useQueryClient();
  return () => {
    void queryClient.invalidateQueries({queryKey: ['my-invites']});
    void queryClient.invalidateQueries({queryKey: ['bands']});
    void queryClient.invalidateQueries({queryKey: ['songs']});
  };
}

export function useAcceptInvite() {
  const onResolved = useInviteResolution();
  return useMutation<{bandId: number}, ApiError, number>({
    mutationFn: id => api.post<{bandId: number}>(`/api/invites/${id}/accept`),
    onSuccess: onResolved,
  });
}

export function useDeclineInvite() {
  const onResolved = useInviteResolution();
  return useMutation<void, ApiError, number>({
    mutationFn: id => api.post<void>(`/api/invites/${id}/decline`),
    onSuccess: onResolved,
  });
}

export function useJoinByLink() {
  const onResolved = useInviteResolution();
  return useMutation<{bandId: number}, ApiError, string>({
    mutationFn: token => api.post<{bandId: number}>(`/api/invites/link/${token}`),
    onSuccess: onResolved,
  });
}
```

(The `['songs']` invalidation is forward-compatible with Plan 4b, when joining a band changes the library; harmless now.)

- [ ] **Step 4: Checks (just test-frontend / typecheck / lint-js / format-check) green, commit**

```bash
git add frontend
git commit -m "feat: band and invite types and hooks"
```

---

### Task 9: Bands page, navbar link + badge, join page

**Files:**
- Create: `frontend/src/pages/BandsPage.tsx`, `frontend/src/pages/BandsPage.test.tsx`, `frontend/src/pages/JoinPage.tsx`
- Modify: `frontend/src/components/Layout.tsx` (Bands link + invite badge), `frontend/src/App.tsx` (routes)

- [ ] **Step 1: Failing tests** — `frontend/src/pages/BandsPage.test.tsx`:

```tsx
import {screen, waitFor} from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import {beforeEach, describe, expect, it, vi} from 'vitest';
import {renderWithProviders} from '../test/utils';
import BandsPage from './BandsPage';

const bands = [{id: 1, name: 'The Quietones', role: 'admin', memberCount: 3}];
const invites = [{id: 9, bandId: 2, bandName: 'Loud Ones', role: 'editor'}];

function jsonResponse(status: number, body: unknown) {
  return new Response(JSON.stringify(body), {status});
}

describe('BandsPage', () => {
  beforeEach(() => {
    vi.stubGlobal(
      'fetch',
      vi.fn().mockImplementation((input: RequestInfo | URL, init?: RequestInit) => {
        const url = String(input);
        if (url.includes('/api/invites') && init?.method === 'POST') {
          return Promise.resolve(jsonResponse(200, {bandId: 2}));
        }
        if (url.includes('/api/invites')) {
          return Promise.resolve(jsonResponse(200, invites));
        }
        if (url.includes('/api/bands') && init?.method === 'POST') {
          return Promise.resolve(jsonResponse(201, {id: 5}));
        }
        return Promise.resolve(jsonResponse(200, bands));
      }),
    );
  });

  it('lists bands with role and member count', async () => {
    renderWithProviders(<BandsPage />);
    expect(await screen.findByText('The Quietones')).toBeInTheDocument();
    expect(screen.getByText(/admin/i)).toBeInTheDocument();
    expect(screen.getByText(/3 members/i)).toBeInTheDocument();
  });

  it('shows pending invites with accept/decline', async () => {
    renderWithProviders(<BandsPage />);
    expect(await screen.findByText('Loud Ones')).toBeInTheDocument();
    await userEvent.click(screen.getByRole('button', {name: /accept/i}));
    await waitFor(() => {
      const calls = vi.mocked(fetch).mock.calls;
      expect(
        calls.some(
          ([input, init]) =>
            String(input).includes('/accept') && init?.method === 'POST',
        ),
      ).toBe(true);
    });
  });

  it('creates a band', async () => {
    renderWithProviders(<BandsPage />);
    await screen.findByText('The Quietones');
    await userEvent.type(screen.getByPlaceholderText(/new band/i), 'Fresh Band');
    await userEvent.click(screen.getByRole('button', {name: /create/i}));
    await waitFor(() => {
      const calls = vi.mocked(fetch).mock.calls;
      expect(
        calls.some(
          ([input, init]) =>
            String(input).endsWith('/api/bands') &&
            init?.method === 'POST' &&
            String(init.body).includes('Fresh Band'),
        ),
      ).toBe(true);
    });
  });
});
```

Run: `just test-frontend` — FAIL.

- [ ] **Step 2: Write `frontend/src/pages/BandsPage.tsx`**

```tsx
import {useState} from 'react';
import type {FormEvent} from 'react';
import {Link} from 'react-router';
import {useBands, useCreateBand} from '../hooks/bands';
import {useAcceptInvite, useDeclineInvite, useMyInvites} from '../hooks/invites';

export default function BandsPage() {
  const {data: bands = []} = useBands();
  const {data: invites = []} = useMyInvites();
  const createBand = useCreateBand();
  const acceptInvite = useAcceptInvite();
  const declineInvite = useDeclineInvite();
  const [name, setName] = useState('');

  const create = (e: FormEvent) => {
    e.preventDefault();
    if (!name.trim()) return;
    createBand.mutate({name: name.trim()}, {onSuccess: () => setName('')});
  };

  return (
    <div className="flex flex-col gap-6">
      <h1 className="text-3xl font-bold">Bands</h1>

      {invites.length > 0 && (
        <section className="card bg-base-100 shadow">
          <div className="card-body">
            <h2 className="card-title">Invitations</h2>
            <ul className="flex flex-col gap-2">
              {invites.map(invite => (
                <li key={invite.id} className="flex items-center gap-3">
                  <span className="min-w-0 flex-1 truncate">
                    {invite.bandName}{' '}
                    <span className="badge badge-ghost">{invite.role}</span>
                  </span>
                  <button
                    className="btn btn-primary btn-sm"
                    onClick={() => acceptInvite.mutate(invite.id)}
                  >
                    Accept
                  </button>
                  <button
                    className="btn btn-ghost btn-sm"
                    onClick={() => declineInvite.mutate(invite.id)}
                  >
                    Decline
                  </button>
                </li>
              ))}
            </ul>
            {(acceptInvite.error ?? declineInvite.error) && (
              <div role="alert" className="alert alert-error">
                {(acceptInvite.error ?? declineInvite.error)?.message}
              </div>
            )}
          </div>
        </section>
      )}

      {bands.length === 0 ? (
        <p className="text-base-content/60 py-6 text-center">
          No bands yet — create one or ask for an invite.
        </p>
      ) : (
        <ul className="flex flex-col gap-2">
          {bands.map(band => (
            <li key={band.id}>
              <Link
                to={`/bands/${band.id}`}
                className="bg-base-100 flex items-center gap-3 rounded-box p-4 shadow-sm"
              >
                <span className="min-w-0 flex-1 truncate font-semibold">
                  {band.name}
                </span>
                <span className="badge badge-ghost">{band.role}</span>
                <span className="text-base-content/60 text-sm">
                  {band.memberCount} members
                </span>
              </Link>
            </li>
          ))}
        </ul>
      )}

      <form className="flex gap-2" onSubmit={create}>
        <input
          className="input min-w-0 flex-1"
          placeholder="New band name…"
          value={name}
          onChange={e => setName(e.target.value)}
        />
        <button className="btn btn-primary" disabled={createBand.isPending}>
          Create
        </button>
      </form>
      {createBand.error && (
        <div role="alert" className="alert alert-error">
          {createBand.error.message}
        </div>
      )}
    </div>
  );
}
```

- [ ] **Step 3: Write `frontend/src/pages/JoinPage.tsx`**

```tsx
import {useNavigate, useParams} from 'react-router';
import {useJoinByLink} from '../hooks/invites';

export default function JoinPage() {
  const {token} = useParams();
  const navigate = useNavigate();
  const join = useJoinByLink();

  if (!token) {
    return <p className="py-12 text-center">Invalid invite link.</p>;
  }

  return (
    <div className="flex flex-col items-center gap-4 py-12">
      <h1 className="text-2xl font-bold">Join band</h1>
      <p>You have been invited to join a band.</p>
      {join.error && (
        <div role="alert" className="alert alert-error">
          {join.error.message}
        </div>
      )}
      <button
        className="btn btn-primary"
        disabled={join.isPending}
        onClick={() =>
          join.mutate(token, {
            onSuccess: ({bandId}) => void navigate(`/bands/${bandId}`),
          })
        }
      >
        Join
      </button>
    </div>
  );
}
```

- [ ] **Step 4: Navbar + routes**

In `frontend/src/components/Layout.tsx`, add a Bands link with a pending-invite badge between the brand and Profile:

```tsx
import {Link, Outlet} from 'react-router';
import {useLogout} from '../hooks/auth';
import {useMyInvites} from '../hooks/invites';

export default function Layout() {
  const logout = useLogout();
  const {data: invites = []} = useMyInvites();
  return (
    <div className="bg-base-200 min-h-screen">
      <nav className="navbar bg-base-100 shadow">
        <div className="flex-1">
          <Link to="/" className="btn btn-ghost text-xl">
            BandWidth
          </Link>
        </div>
        <div className="flex-none gap-2">
          <Link to="/bands" className="btn btn-ghost">
            Bands
            {invites.length > 0 && (
              <span className="badge badge-primary badge-sm">{invites.length}</span>
            )}
          </Link>
          <Link to="/profile" className="btn btn-ghost">
            Profile
          </Link>
          <button
            className="btn btn-ghost"
            onClick={() => logout.mutate()}
            disabled={logout.isPending}
          >
            Log out
          </button>
        </div>
      </nav>
      <main className="mx-auto max-w-3xl p-4">
        <Outlet />
      </main>
    </div>
  );
}
```

In `frontend/src/App.tsx`, inside the Layout route group add:

```tsx
          <Route path="/bands" element={<BandsPage />} />
          <Route path="/bands/:id" element={<BandPage />} />
          <Route path="/join/:token" element={<JoinPage />} />
```

(BandPage arrives in Task 10 — create a minimal placeholder `frontend/src/pages/BandPage.tsx` NOW so the route compiles; Task 10 replaces it:)

```tsx
export default function BandPage() {
  return <p className="py-12 text-center">Band page coming right up.</p>;
}
```

NOTE: existing Layout-dependent tests (if any assert navbar contents) may need minimal assertion adjustments — Layout now fetches /api/invites, so component tests rendering Layout need that stub; check `just test-frontend` output and adapt minimally, reporting each change.

- [ ] **Step 5: All four frontend checks green, commit**

```bash
git add frontend
git commit -m "feat: bands page, invite handling, navbar badge, and join page"
```

---

### Task 10: Band page (roster, invites, settings)

**Files:**
- Replace: `frontend/src/pages/BandPage.tsx`
- Create: `frontend/src/components/bands/MemberList.tsx`, `InviteManager.tsx`, `BandSettings.tsx`
- Test: `frontend/src/pages/BandPage.test.tsx`

- [ ] **Step 1: Failing tests** — `frontend/src/pages/BandPage.test.tsx`:

```tsx
import {screen, waitFor} from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import {beforeEach, describe, expect, it, vi} from 'vitest';
import {Route, Routes} from 'react-router';
import {renderWithProviders} from '../test/utils';
import BandPage from './BandPage';

const detail = {
  id: 1,
  name: 'The Quietones',
  creatorId: 10,
  myRole: 'admin',
  members: [
    {userId: 10, username: 'alice', role: 'admin'},
    {userId: 11, username: 'bob', role: 'editor'},
  ],
};

function jsonResponse(status: number, body: unknown) {
  return new Response(JSON.stringify(body), {status});
}

function renderBandPage(myRole = 'admin') {
  vi.stubGlobal(
    'fetch',
    vi.fn().mockImplementation((input: RequestInfo | URL, init?: RequestInit) => {
      const url = String(input);
      if (url.includes('/invites') && init?.method === 'POST') {
        return Promise.resolve(
          jsonResponse(201, {role: 'viewer', token: 'TOK123', isLink: true}),
        );
      }
      if (url.includes('/invites')) {
        return Promise.resolve(jsonResponse(200, []));
      }
      if (init?.method === 'PATCH' || init?.method === 'DELETE') {
        return Promise.resolve(new Response(null, {status: 204}));
      }
      return Promise.resolve(jsonResponse(200, {...detail, myRole}));
    }),
  );
  return renderWithProviders(
    <Routes>
      <Route path="/bands/:id" element={<BandPage />} />
      <Route path="/bands" element={<p>bands list</p>} />
    </Routes>,
    {route: '/bands/1'},
  );
}

describe('BandPage', () => {
  beforeEach(() => vi.unstubAllGlobals());

  it('shows the roster with roles', async () => {
    renderBandPage();
    expect(await screen.findByText('alice')).toBeInTheDocument();
    expect(screen.getByText('bob')).toBeInTheDocument();
  });

  it('admins can change a member role', async () => {
    renderBandPage('admin');
    await screen.findByText('bob');
    const select = screen.getByLabelText(/role for bob/i);
    await userEvent.selectOptions(select, 'viewer');
    await waitFor(() => {
      const calls = vi.mocked(fetch).mock.calls;
      expect(
        calls.some(
          ([input, init]) =>
            String(input).includes('/members/11') && init?.method === 'PATCH',
        ),
      ).toBe(true);
    });
  });

  it('admins can create a share link and see the token once', async () => {
    renderBandPage('admin');
    await screen.findByText('bob');
    await userEvent.click(screen.getByRole('button', {name: /create invite link/i}));
    await waitFor(() =>
      expect(screen.getByText(/\/join\/TOK123/)).toBeInTheDocument(),
    );
  });

  it('non-admins see the roster but no management controls', async () => {
    renderBandPage('viewer');
    await screen.findByText('bob');
    expect(screen.queryByLabelText(/role for bob/i)).not.toBeInTheDocument();
    expect(
      screen.queryByRole('button', {name: /create invite link/i}),
    ).not.toBeInTheDocument();
  });
});
```

Run: `just test-frontend` — FAIL (placeholder page).

- [ ] **Step 2: Write `frontend/src/components/bands/MemberList.tsx`**

```tsx
import {useMe} from '../../hooks/auth';
import {useRemoveMember, useSetMemberRole} from '../../hooks/bands';
import type {BandDetail, BandRole} from '../../lib/types';

const roles: BandRole[] = ['viewer', 'editor', 'admin'];

export default function MemberList({band}: {band: BandDetail}) {
  const {data: me} = useMe();
  const setRole = useSetMemberRole(band.id);
  const removeMember = useRemoveMember(band.id);
  const isAdmin = band.myRole === 'admin';

  return (
    <section className="card bg-base-100 shadow">
      <div className="card-body">
        <h2 className="card-title">Members</h2>
        <ul className="flex flex-col gap-2">
          {band.members.map(member => {
            const isCreator = member.userId === band.creatorId;
            const isSelf = member.userId === me?.id;
            return (
              <li key={member.userId} className="flex items-center gap-3">
                <span className="min-w-0 flex-1 truncate">
                  {member.username}
                  {isCreator && (
                    <span className="badge badge-ghost badge-sm ml-2">creator</span>
                  )}
                </span>
                {isAdmin && !isCreator ? (
                  <select
                    className="select select-sm"
                    aria-label={`Role for ${member.username}`}
                    value={member.role}
                    onChange={e =>
                      setRole.mutate({
                        userId: member.userId,
                        role: e.target.value as BandRole,
                      })
                    }
                  >
                    {roles.map(r => (
                      <option key={r} value={r}>
                        {r}
                      </option>
                    ))}
                  </select>
                ) : (
                  <span className="badge badge-ghost">{member.role}</span>
                )}
                {((isAdmin && !isCreator) || (isSelf && !isCreator)) && (
                  <button
                    className="btn btn-ghost btn-xs"
                    aria-label={
                      isSelf ? 'Leave band' : `Remove ${member.username}`
                    }
                    onClick={() => removeMember.mutate(member.userId)}
                  >
                    {isSelf ? 'Leave' : '✕'}
                  </button>
                )}
              </li>
            );
          })}
        </ul>
        {(setRole.error ?? removeMember.error) && (
          <div role="alert" className="alert alert-error">
            {(setRole.error ?? removeMember.error)?.message}
          </div>
        )}
      </div>
    </section>
  );
}
```

- [ ] **Step 3: Write `frontend/src/components/bands/InviteManager.tsx`**

```tsx
import {useState} from 'react';
import type {FormEvent} from 'react';
import {useBandInvites, useCreateInvite, useRevokeInvite} from '../../hooks/bands';
import type {BandRole} from '../../lib/types';

export default function InviteManager({bandId}: {bandId: number}) {
  const {data: invites = []} = useBandInvites(bandId, true);
  const createInvite = useCreateInvite(bandId);
  const revokeInvite = useRevokeInvite(bandId);
  const [username, setUsername] = useState('');
  const [role, setRole] = useState<BandRole>('editor');
  const [linkURL, setLinkURL] = useState<string | null>(null);

  const inviteUser = (e: FormEvent) => {
    e.preventDefault();
    if (!username.trim()) return;
    createInvite.mutate(
      {username: username.trim(), role},
      {onSuccess: () => setUsername('')},
    );
  };

  const createLink = () => {
    createInvite.mutate(
      {link: true, role},
      {
        onSuccess: created => {
          if (created.token) {
            setLinkURL(`${window.location.origin}/join/${created.token}`);
          }
        },
      },
    );
  };

  return (
    <section className="card bg-base-100 shadow">
      <div className="card-body">
        <h2 className="card-title">Invites</h2>
        <form className="flex flex-wrap gap-2" onSubmit={inviteUser}>
          <input
            className="input input-sm min-w-0 flex-1"
            placeholder="Username or email…"
            value={username}
            onChange={e => setUsername(e.target.value)}
          />
          <select
            className="select select-sm"
            aria-label="Invite role"
            value={role}
            onChange={e => setRole(e.target.value as BandRole)}
          >
            <option value="viewer">viewer</option>
            <option value="editor">editor</option>
            <option value="admin">admin</option>
          </select>
          <button className="btn btn-sm" disabled={createInvite.isPending}>
            Invite
          </button>
        </form>
        <button
          className="btn btn-outline btn-sm"
          onClick={createLink}
          disabled={createInvite.isPending}
        >
          Create invite link
        </button>
        {linkURL && (
          <div className="bg-base-200 rounded-box flex items-center gap-2 p-3">
            <code className="min-w-0 flex-1 truncate text-sm">{linkURL}</code>
            <button
              className="btn btn-ghost btn-xs"
              onClick={() => void navigator.clipboard.writeText(linkURL)}
            >
              Copy
            </button>
          </div>
        )}
        {createInvite.error && (
          <div role="alert" className="alert alert-error">
            {createInvite.error.message}
          </div>
        )}
        {invites.length > 0 && (
          <ul className="flex flex-col gap-1">
            {invites.map(invite => (
              <li key={invite.id} className="flex items-center gap-2 text-sm">
                <span className="min-w-0 flex-1 truncate">
                  {invite.isLink ? 'Invite link' : invite.invitedUsername}
                  {' · '}
                  {invite.role}
                </span>
                <button
                  className="btn btn-ghost btn-xs"
                  onClick={() => revokeInvite.mutate(invite.id)}
                >
                  Revoke
                </button>
              </li>
            ))}
          </ul>
        )}
      </div>
    </section>
  );
}
```

- [ ] **Step 4: Write `frontend/src/components/bands/BandSettings.tsx`**

```tsx
import {useState} from 'react';
import type {FormEvent} from 'react';
import {useNavigate} from 'react-router';
import ConfirmModal from '../songs/ConfirmModal';
import {useDeleteBand, useRenameBand} from '../../hooks/bands';
import {useMe} from '../../hooks/auth';
import type {BandDetail} from '../../lib/types';

export default function BandSettings({band}: {band: BandDetail}) {
  const {data: me} = useMe();
  const renameBand = useRenameBand(band.id);
  const deleteBand = useDeleteBand();
  const navigate = useNavigate();
  const [name, setName] = useState(band.name);
  const [confirming, setConfirming] = useState(false);
  const isCreator = me?.id === band.creatorId;

  const rename = (e: FormEvent) => {
    e.preventDefault();
    if (name.trim()) {
      renameBand.mutate({name: name.trim()});
    }
  };

  return (
    <section className="card bg-base-100 shadow">
      <div className="card-body">
        <h2 className="card-title">Settings</h2>
        <form className="flex gap-2" onSubmit={rename}>
          <input
            className="input min-w-0 flex-1"
            aria-label="Band name"
            value={name}
            onChange={e => setName(e.target.value)}
          />
          <button className="btn" disabled={renameBand.isPending}>
            Rename
          </button>
        </form>
        {renameBand.error && (
          <div role="alert" className="alert alert-error">
            {renameBand.error.message}
          </div>
        )}
        {isCreator && (
          <div className="card-actions">
            <button
              className="btn btn-error btn-outline"
              onClick={() => setConfirming(true)}
            >
              Delete band
            </button>
          </div>
        )}
        <ConfirmModal
          open={confirming}
          title="Delete band"
          message={`Delete “${band.name}” for every member?`}
          confirmLabel="Delete"
          onConfirm={() =>
            deleteBand.mutate(band.id, {
              onSuccess: () => void navigate('/bands'),
            })
          }
          onCancel={() => setConfirming(false)}
        />
      </div>
    </section>
  );
}
```

- [ ] **Step 5: Replace `frontend/src/pages/BandPage.tsx`**

```tsx
import {Link, useParams} from 'react-router';
import BandSettings from '../components/bands/BandSettings';
import InviteManager from '../components/bands/InviteManager';
import MemberList from '../components/bands/MemberList';
import {useBand} from '../hooks/bands';

export default function BandPage() {
  const {id: idParam} = useParams();
  const id = Number(idParam);
  const {data: band, isPending, isError, error, refetch} = useBand(id);

  if (isPending) {
    return (
      <div className="flex justify-center py-12">
        <span className="loading loading-spinner" aria-label="Loading" />
      </div>
    );
  }
  if (isError || !band) {
    return (
      <div className="flex flex-col items-center gap-4 py-12">
        <p>{error?.message ?? 'Could not load this band.'}</p>
        <div className="flex gap-2">
          <button className="btn" onClick={() => void refetch()}>
            Retry
          </button>
          <Link className="btn btn-ghost" to="/bands">
            Back to bands
          </Link>
        </div>
      </div>
    );
  }

  const isAdmin = band.myRole === 'admin';
  return (
    <div className="flex flex-col gap-6">
      <h1 className="text-3xl font-bold">{band.name}</h1>
      <MemberList band={band} />
      {isAdmin && <InviteManager bandId={band.id} />}
      {isAdmin && <BandSettings band={band} />}
    </div>
  );
}
```

- [ ] **Step 6: All four frontend checks green, commit**

```bash
git add frontend
git commit -m "feat: band page with roster, invites, and settings"
```

---

### Task 11: Profile bands card with leave

**Files:**
- Create: `frontend/src/components/bands/BandsCard.tsx`
- Modify: `frontend/src/pages/ProfilePage.tsx`

- [ ] **Step 1: Write `frontend/src/components/bands/BandsCard.tsx`**

```tsx
import {useState} from 'react';
import {Link} from 'react-router';
import ConfirmModal from '../songs/ConfirmModal';
import {useMe} from '../../hooks/auth';
import {useBands, useRemoveMember} from '../../hooks/bands';
import type {BandSummary} from '../../lib/types';

function LeaveButton({band}: {band: BandSummary}) {
  const {data: me} = useMe();
  const removeMember = useRemoveMember(band.id);
  const [confirming, setConfirming] = useState(false);

  return (
    <>
      <button className="btn btn-ghost btn-xs" onClick={() => setConfirming(true)}>
        Leave
      </button>
      <ConfirmModal
        open={confirming}
        title="Leave band"
        message={`Leave “${band.name}”?`}
        confirmLabel="Leave"
        onConfirm={() => {
          if (me) {
            removeMember.mutate(me.id);
          }
          setConfirming(false);
        }}
        onCancel={() => setConfirming(false)}
      />
    </>
  );
}

export default function BandsCard() {
  const {data: bands = []} = useBands();

  return (
    <section className="card bg-base-100 shadow">
      <div className="card-body">
        <h2 className="card-title">Bands</h2>
        {bands.length === 0 ? (
          <p className="text-base-content/60 text-sm">
            You are not in any bands yet.{' '}
            <Link className="link" to="/bands">
              Create or join one.
            </Link>
          </p>
        ) : (
          <ul className="flex flex-col gap-2">
            {bands.map(band => (
              <li key={band.id} className="flex items-center gap-3">
                <Link to={`/bands/${band.id}`} className="link min-w-0 flex-1 truncate">
                  {band.name}
                </Link>
                <span className="badge badge-ghost">{band.role}</span>
                <LeaveButton band={band} />
              </li>
            ))}
          </ul>
        )}
      </div>
    </section>
  );
}
```

NOTE: the creator's Leave is rejected server-side with a clear message — the error isn't currently surfaced in this card. Add an error alert fed by the LeaveButton's mutation (lift `removeMember.error` rendering inside LeaveButton below the modal):

```tsx
      {removeMember.error && (
        <div role="alert" className="alert alert-error">
          {removeMember.error.message}
        </div>
      )}
```

- [ ] **Step 2: Mount in `frontend/src/pages/ProfilePage.tsx`** — add `<BandsCard />` after `<TwoFactorSettings />` (plus the import).

- [ ] **Step 3: All four frontend checks green, commit**

```bash
git add frontend
git commit -m "feat: profile bands card with leave"
```

---

### Task 12: Docs and final verification

**Files:**
- Modify: `AGENTS.md`, `README.md`

- [ ] **Step 1: Update `AGENTS.md`** (verify every claim against the code):

1. Architecture: extend the `internal/handlers/` bullet's parenthetical with `bands, members, invites`; extend `internal/model/` with `Band, BandMember, BandInvite`.
2. Append to the Domain model section:

```markdown
Bands have Admin/Editor/Viewer roles; the creator is a permanent Admin
(cannot be demoted, removed, or leave — they delete the band). New members
default to Editor. `MemberRole(bandID, userID)` is the authorization
primitive; non-members receive 404s for band resources. Direct invites are
single-use (14-day expiry); share links are multi-use until revoked (7-day
expiry); invite tokens are stored hashed. Band songs and the band metadata
layer arrive in the bands-songs plan.
```

- [ ] **Step 2: Update `README.md`** — in the Stack paragraph, change "Planned per the design doc: bands, installable PWA, single container on fly.io." to "Bands with roles and invites are implemented (band songs next). Planned per the design doc: band song sharing, installable PWA, single container on fly.io."

- [ ] **Step 3: `just check` → "all checks passed"; only the two doc files dirty. Commit:**

```bash
git add AGENTS.md README.md
git commit -m "docs: document bands core"
```

---

## Done criteria

- `just check` green.
- Through the dev loop: create a band; invite another account by username; see the invite badge; accept; roster shows both; admin changes a role; share link joins a third account at the link's role; revoke link; member leaves; creator renames and deletes the band.
- Creator protections enforced (no demote/remove/leave); non-members get 404s; non-admins get 403s on management.
- Next: Plan 4b (Band Songs & Interleaving) gets written against this codebase.
