# Admin panel & signup access policy Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add a site-admin panel (list/delete users and bands) and a
DB-backed signup access policy (deny-all-except-allow-list), replacing the
need for an external gateway like Cloudflare Access.

**Architecture:** Site-admin identity is computed from a `BANDWIDTH_ADMIN_EMAILS`
Fly secret (never touches git), checked against the logged-in user's email
on every admin request. The signup allow-list and its enable/disable toggle
are DB-backed and managed through a new `/admin` panel that reuses the
existing session/login system. A new `Repo.DeleteUser` cascades through a
user's personal data and any band they created, reusing `DeleteBand`'s and
`DeleteSong`'s logic factored into transaction-scoped helpers.

**Tech Stack:** Go, Echo v5, GORM/SQLite (existing); React, react-router,
@tanstack/react-query, DaisyUI (existing). No new dependencies.

## Global Constraints

- `BANDWIDTH_ADMIN_EMAILS` (comma-separated) is a Fly secret / env var only — it must never be committed to git or hardcoded.
- The signup allow-list matches emails exactly (no domain or wildcard matching).
- The access policy gates registration only; it never affects login for existing accounts.
- Admin panel capabilities are list + delete only: no edit, no force-logout, no band-membership drill-down.
- An admin cannot delete their own account through the panel.
- Deleting a user cascades to delete any band they created (and that band's other members lose access).
- Follow existing repo conventions exactly: GORM `AutoMigrate`, Echo route groups, `map[string]any` JSON responses for ad hoc payloads vs. `json`-tagged DTOs in `internal/repository` for list responses, react-query hooks in `frontend/src/hooks/`, DaisyUI classes as used in `Layout.tsx`/`BandsPage.tsx`.

---

## Task 1: Config — `BANDWIDTH_ADMIN_EMAILS` + `API.IsAdminEmail`

**Files:**
- Modify: `cmd/bandwidth/main.go` (add viper default)
- Modify: `cmd/bandwidth/server.go` (parse env var, wire into `API`)
- Modify: `internal/handlers/api.go` (add `AdminEmails` field + `IsAdminEmail` method)
- Test: `internal/handlers/api_test.go` (new)

**Interfaces:**
- Produces: `API.AdminEmails map[string]bool` (lowercase, trimmed keys), `func (a *API) IsAdminEmail(email string) bool`. Every later task that needs to check admin status uses `a.IsAdminEmail(...)`.

- [ ] **Step 1: Write the failing test**

Create `internal/handlers/api_test.go`:

```go
package handlers

import "testing"

func TestIsAdminEmail(t *testing.T) {
	api := &API{AdminEmails: map[string]bool{"admin@example.com": true}}
	tests := []struct {
		email string
		want  bool
	}{
		{"admin@example.com", true},
		{"ADMIN@EXAMPLE.COM", true},
		{"  admin@example.com  ", true},
		{"nobody@example.com", false},
	}
	for _, tt := range tests {
		if got := api.IsAdminEmail(tt.email); got != tt.want {
			t.Errorf("IsAdminEmail(%q) = %v, want %v", tt.email, got, tt.want)
		}
	}
}

func TestIsAdminEmailNilSet(t *testing.T) {
	api := &API{}
	if api.IsAdminEmail("anyone@example.com") {
		t.Error("nil AdminEmails should match nobody")
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `just check` (or, if iterating locally inside the dev container, `go test ./internal/handlers/... -run TestIsAdminEmail`)
Expected: FAIL — `API` has no field `AdminEmails` / no method `IsAdminEmail`.

- [ ] **Step 3: Add the field and method**

In `internal/handlers/api.go`, add `"strings"` to the import block and update the `API` struct and add the method:

```go
import (
	"log/slog"
	"net/http"
	"strings"

	"github.com/labstack/echo/v5"

	"github.com/jwhumphries/bandwidth/internal/auth"
	"github.com/jwhumphries/bandwidth/internal/mail"
	"github.com/jwhumphries/bandwidth/internal/model"
	"github.com/jwhumphries/bandwidth/internal/repository"
)

// API holds the dependencies shared by all HTTP handlers.
type API struct {
	Repo          *repository.Repo
	Mailer        mail.Mailer
	Logger        *slog.Logger
	BaseURL       string
	SecureCookies bool
	AdminEmails   map[string]bool
}

// IsAdminEmail reports whether email belongs to a configured site admin.
func (a *API) IsAdminEmail(email string) bool {
	return a.AdminEmails[strings.ToLower(strings.TrimSpace(email))]
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `just check`
Expected: PASS

- [ ] **Step 5: Wire the env var through config and server startup**

In `cmd/bandwidth/main.go`, add a default and update the doc comment:

```go
// initConfig wires Viper to BANDWIDTH_* environment variables.
// Keys: port, log_level, db_path, secure_cookies, base_url, smtp_*, admin_emails.
func initConfig() {
	viper.SetDefault("port", ":8080")
	viper.SetDefault("log_level", "info")
	viper.SetDefault("db_path", "data/bandwidth.db")
	viper.SetDefault("secure_cookies", false)
	viper.SetDefault("base_url", "http://localhost:3000")
	viper.SetDefault("smtp_host", "")
	viper.SetDefault("smtp_port", 587)
	viper.SetDefault("smtp_user", "")
	viper.SetDefault("smtp_pass", "")
	viper.SetDefault("smtp_from", "")
	viper.SetDefault("admin_emails", "")
	viper.SetEnvPrefix("BANDWIDTH")
	viper.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	viper.AutomaticEnv()
}
```

In `cmd/bandwidth/server.go`, add a parsing helper and wire it into the `API` literal inside `runServer()`:

```go
	api := &handlers.API{
		Repo:          repo,
		Mailer:        mailer,
		Logger:        logger,
		BaseURL:       viper.GetString("base_url"),
		SecureCookies: viper.GetBool("secure_cookies"),
		AdminEmails:   parseAdminEmails(viper.GetString("admin_emails")),
	}
```

Add the helper function anywhere at package scope in `server.go` (near `redactPath`):

```go
// parseAdminEmails splits a comma-separated BANDWIDTH_ADMIN_EMAILS value into
// a lowercase, trimmed lookup set.
func parseAdminEmails(raw string) map[string]bool {
	emails := map[string]bool{}
	for _, e := range strings.Split(raw, ",") {
		e = strings.ToLower(strings.TrimSpace(e))
		if e != "" {
			emails[e] = true
		}
	}
	return emails
}
```

`strings` is already imported in `server.go` (used by `redactPath`), so no import changes are needed there.

- [ ] **Step 6: Run the full check suite**

Run: `just check`
Expected: PASS (build + tests unaffected elsewhere)

- [ ] **Step 7: Commit**

```bash
git add cmd/bandwidth/main.go cmd/bandwidth/server.go internal/handlers/api.go internal/handlers/api_test.go
git commit -m "Add BANDWIDTH_ADMIN_EMAILS config and API.IsAdminEmail"
```

---

## Task 2: Model + migration — `AccessPolicy` & `AllowedEmail`

**Files:**
- Create: `internal/model/accesspolicy.go`
- Modify: `internal/repository/repository.go:39-53` (AutoMigrate list)
- Test: `internal/repository/repository_test.go:25-39` (extend `TestOpenMigratesSchema`)

**Interfaces:**
- Produces: `model.AccessPolicy{ID uint, Enabled bool}`, `model.AllowedEmail{ID uint, Email string, CreatedBy uint, CreatedAt time.Time}`. Task 3's repository functions operate on these.

- [ ] **Step 1: Write the failing test**

In `internal/repository/repository_test.go`, extend the table list inside `TestOpenMigratesSchema`:

```go
func TestOpenMigratesSchema(t *testing.T) {
	repo := testRepo(t)

	for _, table := range []string{
		"users", "sessions", "backup_codes", "password_resets",
		"songs", "song_annotations", "resources", "practice_events",
		"folders", "folder_entries",
		"bands", "band_members", "band_invites",
		"access_policies", "allowed_emails",
	} {
		var n int64
		if err := repo.db.Table(table).Count(&n).Error; err != nil {
			t.Errorf("table %s not migrated: %v", table, err)
		}
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `just check`
Expected: FAIL — `table access_policies not migrated` (and `allowed_emails`).

- [ ] **Step 3: Add the models**

Create `internal/model/accesspolicy.go`:

```go
// Package model holds the persisted domain types.
package model

import "time"

// AccessPolicy is the singleton settings row controlling signup gating.
type AccessPolicy struct {
	ID      uint `gorm:"primarykey"`
	Enabled bool `gorm:"not null"`
}

// AllowedEmail is one entry on the signup allow-list.
type AllowedEmail struct {
	ID        uint   `gorm:"primarykey"`
	Email     string `gorm:"uniqueIndex;not null"`
	CreatedBy uint   `gorm:"not null"`
	CreatedAt time.Time
}
```

- [ ] **Step 4: Add them to AutoMigrate**

In `internal/repository/repository.go`, add the two new types to the `AutoMigrate` call:

```go
	if err := db.AutoMigrate(
		&model.User{},
		&model.Session{},
		&model.BackupCode{},
		&model.PasswordReset{},
		&model.Song{},
		&model.SongAnnotation{},
		&model.Resource{},
		&model.PracticeEvent{},
		&model.Folder{},
		&model.FolderEntry{},
		&model.Band{},
		&model.BandMember{},
		&model.BandInvite{},
		&model.AccessPolicy{},
		&model.AllowedEmail{},
	); err != nil {
```

- [ ] **Step 5: Run test to verify it passes**

Run: `just check`
Expected: PASS

- [ ] **Step 6: Commit**

```bash
git add internal/model/accesspolicy.go internal/repository/repository.go internal/repository/repository_test.go
git commit -m "Add AccessPolicy and AllowedEmail models"
```

---

## Task 3: Repo — access-policy CRUD + admin listings

**Files:**
- Create: `internal/repository/admin.go`
- Test: `internal/repository/admin_test.go`

**Interfaces:**
- Consumes: `model.AccessPolicy`, `model.AllowedEmail` (Task 2); `Repo.CreateUser`, `Repo.CreateBand`, `Repo.AddMember`, `IsDuplicate` (existing).
- Produces:
  - `Repo.AccessPolicyEnabled() (bool, error)`
  - `Repo.SetAccessPolicyEnabled(enabled bool) error`
  - `Repo.EmailAllowed(email string) (bool, error)`
  - `Repo.AllowedEmails() ([]AllowedEmailInfo, error)`
  - `Repo.AddAllowedEmail(email string, addedBy uint) (*model.AllowedEmail, error)`
  - `Repo.RemoveAllowedEmail(id uint) error`
  - `Repo.AllUsers() ([]AdminUserSummary, error)`
  - `Repo.AllBands() ([]AdminBandSummary, error)`
  - DTOs: `AllowedEmailInfo{ID uint, Email string, CreatedAt time.Time}`, `AdminUserSummary{ID uint, Username, Email string, CreatedAt time.Time}`, `AdminBandSummary{ID uint, Name, CreatorUsername string, MemberCount int}`.
  Task 5 (signup enforcement) uses `AccessPolicyEnabled`/`EmailAllowed`. Task 8 (admin handlers) uses all of these.

- [ ] **Step 1: Write the failing tests**

Create `internal/repository/admin_test.go`:

```go
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
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `just check`
Expected: FAIL — none of `AccessPolicyEnabled`, `SetAccessPolicyEnabled`, `EmailAllowed`, `AllowedEmails`, `AddAllowedEmail`, `RemoveAllowedEmail`, `AllUsers`, `AllBands` exist yet.

- [ ] **Step 3: Implement**

Create `internal/repository/admin.go`:

```go
package repository

import (
	"errors"
	"time"

	"gorm.io/gorm"

	"github.com/jwhumphries/bandwidth/internal/model"
)

// AdminUserSummary is one row of the admin user list.
type AdminUserSummary struct {
	ID        uint      `json:"id"`
	Username  string    `json:"username"`
	Email     string    `json:"email"`
	CreatedAt time.Time `json:"createdAt"`
}

// AdminBandSummary is one row of the admin band list.
type AdminBandSummary struct {
	ID              uint   `json:"id"`
	Name            string `json:"name"`
	CreatorUsername string `json:"creatorUsername"`
	MemberCount     int    `json:"memberCount"`
}

// AllowedEmailInfo is one row of the signup allow-list.
type AllowedEmailInfo struct {
	ID        uint      `json:"id"`
	Email     string    `json:"email"`
	CreatedAt time.Time `json:"createdAt"`
}

// AllUsers lists every account for the admin panel.
func (r *Repo) AllUsers() ([]AdminUserSummary, error) {
	users := []AdminUserSummary{}
	err := r.db.Model(&model.User{}).
		Select("id, username, email, created_at").
		Order("username COLLATE NOCASE, id").
		Scan(&users).Error
	if err != nil {
		return nil, err
	}
	return users, nil
}

// AllBands lists every band for the admin panel.
func (r *Repo) AllBands() ([]AdminBandSummary, error) {
	bands := []AdminBandSummary{}
	err := r.db.Table("bands").
		Select(`bands.id, bands.name, users.username AS creator_username,
			(SELECT COUNT(*) FROM band_members bm WHERE bm.band_id = bands.id) AS member_count`).
		Joins("JOIN users ON users.id = bands.creator_id").
		Order("bands.name COLLATE NOCASE, bands.id").
		Scan(&bands).Error
	if err != nil {
		return nil, err
	}
	return bands, nil
}

// accessPolicy returns the singleton settings row, creating it (disabled)
// if this is the first access.
func (r *Repo) accessPolicy() (*model.AccessPolicy, error) {
	var policy model.AccessPolicy
	err := r.db.First(&policy).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		policy = model.AccessPolicy{Enabled: false}
		if err := r.db.Create(&policy).Error; err != nil {
			return nil, err
		}
		return &policy, nil
	}
	if err != nil {
		return nil, err
	}
	return &policy, nil
}

// AccessPolicyEnabled reports whether signup is currently gated.
func (r *Repo) AccessPolicyEnabled() (bool, error) {
	policy, err := r.accessPolicy()
	if err != nil {
		return false, err
	}
	return policy.Enabled, nil
}

// SetAccessPolicyEnabled toggles signup gating.
func (r *Repo) SetAccessPolicyEnabled(enabled bool) error {
	policy, err := r.accessPolicy()
	if err != nil {
		return err
	}
	policy.Enabled = enabled
	return r.db.Save(policy).Error
}

// EmailAllowed reports whether email is on the signup allow-list.
func (r *Repo) EmailAllowed(email string) (bool, error) {
	var n int64
	if err := r.db.Model(&model.AllowedEmail{}).Where("email = ?", email).Count(&n).Error; err != nil {
		return false, err
	}
	return n > 0, nil
}

// AllowedEmails lists the signup allow-list.
func (r *Repo) AllowedEmails() ([]AllowedEmailInfo, error) {
	emails := []AllowedEmailInfo{}
	err := r.db.Model(&model.AllowedEmail{}).
		Select("id, email, created_at").
		Order("email").
		Scan(&emails).Error
	if err != nil {
		return nil, err
	}
	return emails, nil
}

// AddAllowedEmail adds an email to the signup allow-list.
func (r *Repo) AddAllowedEmail(email string, addedBy uint) (*model.AllowedEmail, error) {
	row := &model.AllowedEmail{Email: email, CreatedBy: addedBy}
	if err := r.db.Create(row).Error; err != nil {
		return nil, err
	}
	return row, nil
}

// RemoveAllowedEmail removes an allow-list entry.
func (r *Repo) RemoveAllowedEmail(id uint) error {
	res := r.db.Delete(&model.AllowedEmail{}, id)
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected != 1 {
		return gorm.ErrRecordNotFound
	}
	return nil
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `just check`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/repository/admin.go internal/repository/admin_test.go
git commit -m "Add access-policy CRUD and admin listing queries"
```

---

## Task 4: Repo — factor `DeleteBand`/`DeleteSong` tx bodies + `DeleteUser` cascade

**Files:**
- Modify: `internal/repository/bands.go:146-177` (extract `deleteBandTx`)
- Modify: `internal/repository/songs.go:205-222` (extract `deleteSongRowsTx`)
- Modify: `internal/repository/users.go` (add `DeleteUser`)
- Test: `internal/repository/users_test.go` (extend)

**Interfaces:**
- Consumes: `convertBandSongForUser`, `deleteBandSongRows` (existing, unchanged, in `bandsongs.go`).
- Produces: `deleteBandTx(tx *gorm.DB, bandID uint) error`, `deleteSongRowsTx(tx *gorm.DB, songID uint) error` (both unexported, package-internal — Task 4's own `DeleteUser` is their only new caller), `Repo.DeleteUser(userID uint) error`. Task 8 (admin handlers) calls `Repo.DeleteUser`.

- [ ] **Step 1: Refactor `DeleteBand` — extract `deleteBandTx`**

This step is a pure refactor (no behavior change), verified by the existing test suite rather than a new failing test. In `internal/repository/bands.go`, replace the `DeleteBand` function (lines 146-177) with:

```go
// DeleteBand removes the band, its songs, memberships, and invites. Each
// member's personal work on the band's songs is converted to personal copies
// first. Atomic.
func (r *Repo) DeleteBand(bandID uint) error {
	return r.db.Transaction(func(tx *gorm.DB) error {
		return deleteBandTx(tx, bandID)
	})
}

// deleteBandTx runs DeleteBand's work inside an existing transaction, so
// DeleteUser can cascade a created band without nesting transactions.
func deleteBandTx(tx *gorm.DB, bandID uint) error {
	var songs []model.Song
	if err := tx.Where("owner_band_id = ?", bandID).Find(&songs).Error; err != nil {
		return err
	}
	var members []model.BandMember
	if err := tx.Where("band_id = ?", bandID).Find(&members).Error; err != nil {
		return err
	}
	for i := range songs {
		for _, m := range members {
			if err := convertBandSongForUser(tx, &songs[i], m.UserID); err != nil {
				return err
			}
		}
		if err := deleteBandSongRows(tx, songs[i].ID); err != nil {
			return err
		}
	}
	if err := tx.Where("band_id = ?", bandID).Delete(&model.BandMember{}).Error; err != nil {
		return err
	}
	if err := tx.Where("band_id = ?", bandID).Delete(&model.BandInvite{}).Error; err != nil {
		return err
	}
	return tx.Delete(&model.Band{}, bandID).Error
}
```

- [ ] **Step 2: Run existing band tests to confirm no regression**

Run: `just check`
Expected: PASS — `TestRenameAndDeleteBand`, `TestDeleteBandConvertsForAllMembers`, and all other existing repository/handler tests behave identically.

- [ ] **Step 3: Refactor `DeleteSong` — extract `deleteSongRowsTx`**

In `internal/repository/songs.go`, replace the `DeleteSong` function (lines 205-222) with:

```go
// DeleteSong removes an owned song and everything attached to it
// (annotations, resources, practice events, folder entries) atomically.
func (r *Repo) DeleteSong(songID, userID uint) error {
	if _, err := r.SongForUser(songID, userID); err != nil {
		return fmt.Errorf("song not found: %w", err)
	}
	return r.db.Transaction(func(tx *gorm.DB) error {
		return deleteSongRowsTx(tx, songID)
	})
}

// deleteSongRowsTx removes a song and everything attached to it inside an
// existing transaction, so DeleteUser can cascade owned songs without
// nesting transactions.
func deleteSongRowsTx(tx *gorm.DB, songID uint) error {
	for _, m := range []any{
		&model.FolderEntry{}, &model.PracticeEvent{},
		&model.Resource{}, &model.SongAnnotation{},
	} {
		if err := tx.Where("song_id = ?", songID).Delete(m).Error; err != nil {
			return err
		}
	}
	return tx.Delete(&model.Song{}, songID).Error
}
```

- [ ] **Step 4: Run existing song tests to confirm no regression**

Run: `just check`
Expected: PASS — `TestDeleteSongCascades` and all other existing tests behave identically.

- [ ] **Step 5: Write the failing tests for `DeleteUser`**

Extend `internal/repository/users_test.go` — add the `model` import and these tests:

```go
package repository

import (
	"testing"

	"github.com/jwhumphries/bandwidth/internal/model"
)
```

```go
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
```

(`touchPersonalLayer` is already defined in `internal/repository/conversion_test.go`, same package.)

- [ ] **Step 6: Run tests to verify they fail**

Run: `just check`
Expected: FAIL — `repo.DeleteUser` undefined.

- [ ] **Step 7: Implement `DeleteUser`**

Add `"gorm.io/gorm"` to the imports in `internal/repository/users.go` and append:

```go
// DeleteUser removes a user and everything they solely own: sessions, 2FA
// backup codes, pending password resets, personal (non-band) songs/folders,
// band memberships, pending invites addressed to them, and any band they
// created (cascaded, same as DeleteBand). Their personal layer (annotation/
// resource/practice rows) on any other band song is also cleared, since
// there is no longer a user to own a converted copy. Atomic.
func (r *Repo) DeleteUser(userID uint) error {
	return r.db.Transaction(func(tx *gorm.DB) error {
		var createdBands []model.Band
		if err := tx.Where("creator_id = ?", userID).Find(&createdBands).Error; err != nil {
			return err
		}
		for _, b := range createdBands {
			if err := deleteBandTx(tx, b.ID); err != nil {
				return err
			}
		}
		for _, m := range []any{&model.SongAnnotation{}, &model.Resource{}, &model.PracticeEvent{}} {
			if err := tx.Where("user_id = ?", userID).Delete(m).Error; err != nil {
				return err
			}
		}
		var personalSongs []model.Song
		if err := tx.Where("owner_user_id = ?", userID).Find(&personalSongs).Error; err != nil {
			return err
		}
		for _, s := range personalSongs {
			if err := deleteSongRowsTx(tx, s.ID); err != nil {
				return err
			}
		}
		var folders []model.Folder
		if err := tx.Where("owner_user_id = ?", userID).Find(&folders).Error; err != nil {
			return err
		}
		for _, f := range folders {
			if err := tx.Where("folder_id = ?", f.ID).Delete(&model.FolderEntry{}).Error; err != nil {
				return err
			}
		}
		if err := tx.Where("owner_user_id = ?", userID).Delete(&model.Folder{}).Error; err != nil {
			return err
		}
		if err := tx.Where("user_id = ?", userID).Delete(&model.BandMember{}).Error; err != nil {
			return err
		}
		if err := tx.Where("invited_user_id = ?", userID).Delete(&model.BandInvite{}).Error; err != nil {
			return err
		}
		if err := tx.Where("user_id = ?", userID).Delete(&model.Session{}).Error; err != nil {
			return err
		}
		if err := tx.Where("user_id = ?", userID).Delete(&model.BackupCode{}).Error; err != nil {
			return err
		}
		if err := tx.Where("user_id = ?", userID).Delete(&model.PasswordReset{}).Error; err != nil {
			return err
		}
		return tx.Delete(&model.User{}, userID).Error
	})
}
```

- [ ] **Step 8: Run tests to verify they pass**

Run: `just check`
Expected: PASS

- [ ] **Step 9: Commit**

```bash
git add internal/repository/bands.go internal/repository/songs.go internal/repository/users.go internal/repository/users_test.go
git commit -m "Factor DeleteBand/DeleteSong into tx helpers; add DeleteUser cascade"
```

---

## Task 5: Handler — Signup access-policy enforcement

**Files:**
- Modify: `internal/handlers/auth.go:50-87` (`Signup`)
- Test: `internal/handlers/auth_test.go` (extend)

**Interfaces:**
- Consumes: `a.IsAdminEmail` (Task 1), `a.Repo.AccessPolicyEnabled`, `a.Repo.EmailAllowed`, `a.Repo.AddAllowedEmail` (Task 3).

- [ ] **Step 1: Write the failing tests**

Append to `internal/handlers/auth_test.go`:

```go
func TestSignupAccessPolicy(t *testing.T) {
	e, api := newTestAPI(t)
	if err := api.Repo.SetAccessPolicyEnabled(true); err != nil {
		t.Fatal(err)
	}

	rec := postJSON(e, "/api/auth/signup",
		`{"username":"outsider","email":"outsider@example.com","password":"hunter2hunter2"}`)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want 403", rec.Code)
	}

	admin, _ := api.Repo.CreateUser("admin", "admin@example.com", "h")
	if _, err := api.Repo.AddAllowedEmail("friend@example.com", admin.ID); err != nil {
		t.Fatal(err)
	}
	rec = postJSON(e, "/api/auth/signup",
		`{"username":"friend","email":"friend@example.com","password":"hunter2hunter2"}`)
	if rec.Code != http.StatusCreated {
		t.Fatalf("allow-listed signup: %d %s", rec.Code, rec.Body.String())
	}
}

func TestSignupAccessPolicyAllowsAdminEmail(t *testing.T) {
	e, api := newTestAPI(t)
	api.AdminEmails = map[string]bool{"admin@example.com": true}
	if err := api.Repo.SetAccessPolicyEnabled(true); err != nil {
		t.Fatal(err)
	}

	rec := postJSON(e, "/api/auth/signup",
		`{"username":"admin","email":"admin@example.com","password":"hunter2hunter2"}`)
	if rec.Code != http.StatusCreated {
		t.Fatalf("admin signup: %d %s", rec.Code, rec.Body.String())
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `just check`
Expected: FAIL — signup succeeds (201) for `outsider@example.com` even with the policy enabled, since the check doesn't exist yet.

- [ ] **Step 3: Implement**

In `internal/handlers/auth.go`, insert the check into `Signup` after the existing `validEmail` check and before `auth.HashPassword`:

```go
	if !validEmail(req.Email) {
		return echo.NewHTTPError(http.StatusBadRequest, "a valid email address is required")
	}
	if !a.IsAdminEmail(req.Email) {
		enabled, err := a.Repo.AccessPolicyEnabled()
		if err != nil {
			return err
		}
		if enabled {
			allowed, err := a.Repo.EmailAllowed(req.Email)
			if err != nil {
				return err
			}
			if !allowed {
				return echo.NewHTTPError(http.StatusForbidden, "registration is not open")
			}
		}
	}

	hash, err := auth.HashPassword(req.Password)
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `just check`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/handlers/auth.go internal/handlers/auth_test.go
git commit -m "Enforce signup access policy"
```

---

## Task 6: Handler — `userResponse` gains `isAdmin`

**Files:**
- Modify: `internal/handlers/api.go:32-39` (`userResponse` → method)
- Modify: `internal/handlers/account.go:20,59,102` (call sites)
- Modify: `internal/handlers/auth.go:86,132` (call sites)
- Modify: `internal/handlers/twofa.go:100` (call site)
- Test: `internal/handlers/auth_test.go` (extend)

**Interfaces:**
- Consumes: `a.IsAdminEmail` (Task 1).
- Produces: `/api/me` (and every other endpoint returning a user) JSON payload gains `"isAdmin": bool`. Task 9 (frontend `User` type / `RequireAdmin`) relies on this field.

- [ ] **Step 1: Write the failing test**

Append to `internal/handlers/auth_test.go`:

```go
func TestMeIncludesIsAdmin(t *testing.T) {
	e, api := newTestAPI(t)
	api.AdminEmails = map[string]bool{"alice@example.com": true}
	rec := postJSON(e, "/api/auth/signup",
		`{"username":"alice","email":"alice@example.com","password":"hunter2hunter2"}`)
	cookie := sessionCookie(t, rec)

	req := httptest.NewRequest(http.MethodGet, "/api/me", nil)
	req.AddCookie(cookie)
	mrec := httptest.NewRecorder()
	e.ServeHTTP(mrec, req)

	var body map[string]any
	if err := json.Unmarshal(mrec.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if body["isAdmin"] != true {
		t.Errorf("isAdmin = %v, want true", body["isAdmin"])
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `just check`
Expected: FAIL — response body has no `isAdmin` key, so `body["isAdmin"] != true`.

- [ ] **Step 3: Convert `userResponse` to a method**

In `internal/handlers/api.go`, replace the `userResponse` function:

```go
func (a *API) userResponse(u *model.User) map[string]any {
	return map[string]any{
		"id":          u.ID,
		"username":    u.Username,
		"email":       u.Email,
		"totpEnabled": u.TOTPEnabled(),
		"isAdmin":     a.IsAdminEmail(u.Email),
	}
}
```

- [ ] **Step 4: Update every call site**

Every call site is already inside a method with receiver `a *API`, so this is a mechanical `userResponse(user)` → `a.userResponse(user)` substitution, `replace_all` within each file:

- `internal/handlers/account.go` — three identical occurrences (lines 20, 59, 102):

  ```go
  return c.JSON(http.StatusOK, userResponse(user))
  ```

  become:

  ```go
  return c.JSON(http.StatusOK, a.userResponse(user))
  ```

- `internal/handlers/auth.go` — line 86:

  ```go
  return c.JSON(http.StatusCreated, userResponse(user))
  ```

  becomes:

  ```go
  return c.JSON(http.StatusCreated, a.userResponse(user))
  ```

  and line 132:

  ```go
  return c.JSON(http.StatusOK, userResponse(user))
  ```

  becomes:

  ```go
  return c.JSON(http.StatusOK, a.userResponse(user))
  ```

- `internal/handlers/twofa.go` — line 100:

  ```go
  return c.JSON(http.StatusOK, userResponse(user))
  ```

  becomes:

  ```go
  return c.JSON(http.StatusOK, a.userResponse(user))
  ```

- [ ] **Step 5: Run tests to verify they pass**

Run: `just check`
Expected: PASS

- [ ] **Step 6: Commit**

```bash
git add internal/handlers/api.go internal/handlers/account.go internal/handlers/auth.go internal/handlers/twofa.go internal/handlers/auth_test.go
git commit -m "Include isAdmin in user response payloads"
```

---

## Task 7: Middleware — `RequireAdmin`

**Files:**
- Modify: `internal/middleware/auth.go` (add `RequireAdmin`)
- Test: `internal/middleware/auth_test.go` (extend)

**Interfaces:**
- Consumes: `CurrentUser(c)` (existing), a caller-supplied `isAdminEmail func(string) bool` (matches `API.IsAdminEmail`'s signature from Task 1).
- Produces: `middleware.RequireAdmin(isAdminEmail func(string) bool) echo.MiddlewareFunc`. Task 8 wires it as `appmw.RequireAdmin(api.IsAdminEmail)`.

- [ ] **Step 1: Write the failing test**

Append to `internal/middleware/auth_test.go`:

```go
func newAdminServer(t *testing.T) (*echo.Echo, *repository.Repo) {
	t.Helper()
	repo, err := repository.Open(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	isAdmin := func(email string) bool { return email == "admin@example.com" }
	e := echo.New()
	e.GET("/admin-only", func(c *echo.Context) error {
		return c.String(http.StatusOK, "ok")
	}, RequireAuth(repo), RequireAdmin(isAdmin))
	return e, repo
}

func TestRequireAdmin(t *testing.T) {
	e, repo := newAdminServer(t)
	admin, _ := repo.CreateUser("admin", "admin@example.com", "h")
	adminToken, _ := repo.CreateSession(admin.ID)
	member, _ := repo.CreateUser("bob", "bob@example.com", "h")
	memberToken, _ := repo.CreateSession(member.ID)

	tests := []struct {
		name       string
		token      string
		wantStatus int
	}{
		{name: "non-admin", token: memberToken, wantStatus: http.StatusForbidden},
		{name: "admin", token: adminToken, wantStatus: http.StatusOK},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/admin-only", nil)
			req.AddCookie(&http.Cookie{Name: auth.SessionCookieName, Value: tt.token})
			rec := httptest.NewRecorder()
			e.ServeHTTP(rec, req)
			if rec.Code != tt.wantStatus {
				t.Fatalf("status = %d, want %d", rec.Code, tt.wantStatus)
			}
		})
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `just check`
Expected: FAIL — `RequireAdmin` undefined.

- [ ] **Step 3: Implement**

Append to `internal/middleware/auth.go`:

```go
// RequireAdmin rejects callers whose email is not a configured site admin.
// Must run after RequireAuth so CurrentUser is populated.
func RequireAdmin(isAdminEmail func(string) bool) echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c *echo.Context) error {
			user := CurrentUser(c)
			if user == nil || !isAdminEmail(user.Email) {
				return echo.NewHTTPError(http.StatusForbidden, "admin access required")
			}
			return next(c)
		}
	}
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `just check`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/middleware/auth.go internal/middleware/auth_test.go
git commit -m "Add RequireAdmin middleware"
```

---

## Task 8: Handler — admin endpoints + routes

**Files:**
- Create: `internal/handlers/admin.go`
- Modify: `cmd/bandwidth/server.go` (route registration)
- Test: `internal/handlers/admin_test.go`

**Interfaces:**
- Consumes: `a.Repo.AllUsers/AllBands/AccessPolicyEnabled/AllowedEmails/SetAccessPolicyEnabled/AddAllowedEmail/RemoveAllowedEmail/DeleteUser/DeleteBand/UserByID/BandByID` (Tasks 3-4), `appmw.RequireAdmin`, `appmw.RequireAuth` (Task 7 + existing), `validEmail`, `notFoundOr`, `repository.IsDuplicate` (existing).
- Produces: `API.AdminUsers/AdminDeleteUser/AdminBands/AdminDeleteBand/AdminGetAccessPolicy/AdminSetAccessPolicy/AdminAddAllowedEmail/AdminRemoveAllowedEmail` handler methods, routed under `/api/admin/*`. Task 9-11 (frontend) call these exact paths.

- [ ] **Step 1: Write the failing tests**

Create `internal/handlers/admin_test.go`:

```go
package handlers

import (
	"encoding/json"
	"fmt"
	"net/http"
	"testing"

	"github.com/labstack/echo/v5"

	appmw "github.com/jwhumphries/bandwidth/internal/middleware"
)

func newAdminAPI(t *testing.T) (*echo.Echo, *API) {
	t.Helper()
	e, api := newTestAPI(t)
	g := e.Group("/api/admin", appmw.RequireAuth(api.Repo), appmw.RequireAdmin(api.IsAdminEmail))
	g.GET("/users", api.AdminUsers)
	g.DELETE("/users/:id", api.AdminDeleteUser)
	g.GET("/bands", api.AdminBands)
	g.DELETE("/bands/:id", api.AdminDeleteBand)
	g.GET("/access-policy", api.AdminGetAccessPolicy)
	g.PUT("/access-policy", api.AdminSetAccessPolicy)
	g.POST("/access-policy/emails", api.AdminAddAllowedEmail)
	g.DELETE("/access-policy/emails/:id", api.AdminRemoveAllowedEmail)
	return e, api
}

func mustAdminUserID(t *testing.T, api *API, username string) uint {
	t.Helper()
	user, err := api.Repo.UserByLogin(username)
	if err != nil {
		t.Fatalf("UserByLogin(%s): %v", username, err)
	}
	return user.ID
}

func TestAdminUsersRequiresAdmin(t *testing.T) {
	e, api := newAdminAPI(t)
	bob := signupAndCookie(t, e, "bob")
	rec := jsonReq(e, http.MethodGet, "/api/admin/users", "", bob)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("non-admin: %d, want 403", rec.Code)
	}

	api.AdminEmails = map[string]bool{"admin@example.com": true}
	admin := signupAndCookie(t, e, "admin")
	rec = jsonReq(e, http.MethodGet, "/api/admin/users", "", admin)
	if rec.Code != http.StatusOK {
		t.Fatalf("admin: %d %s", rec.Code, rec.Body.String())
	}
	var users []struct {
		Username string `json:"username"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &users); err != nil {
		t.Fatal(err)
	}
	if len(users) != 2 {
		t.Fatalf("users = %+v, want 2", users)
	}
}

func TestAdminDeleteUser(t *testing.T) {
	e, api := newAdminAPI(t)
	api.AdminEmails = map[string]bool{"admin@example.com": true}
	admin := signupAndCookie(t, e, "admin")
	signupAndCookie(t, e, "bob")
	bobID := mustAdminUserID(t, api, "bob")
	adminID := mustAdminUserID(t, api, "admin")

	rec := jsonReq(e, http.MethodDelete, fmt.Sprintf("/api/admin/users/%d", adminID), "", admin)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("self-delete: %d, want 400", rec.Code)
	}

	rec = jsonReq(e, http.MethodDelete, "/api/admin/users/9999", "", admin)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("delete missing user: %d, want 404", rec.Code)
	}

	rec = jsonReq(e, http.MethodDelete, fmt.Sprintf("/api/admin/users/%d", bobID), "", admin)
	if rec.Code != http.StatusNoContent {
		t.Fatalf("delete bob: %d %s", rec.Code, rec.Body.String())
	}
	if _, err := api.Repo.UserByID(bobID); err == nil {
		t.Error("bob survived admin delete")
	}
}

func TestAdminBandsAndDelete(t *testing.T) {
	e, api := newAdminAPI(t)
	api.AdminEmails = map[string]bool{"admin@example.com": true}
	admin := signupAndCookie(t, e, "admin")
	adminID := mustAdminUserID(t, api, "admin")
	band, err := api.Repo.CreateBand(adminID, "The Quietones")
	if err != nil {
		t.Fatal(err)
	}

	rec := jsonReq(e, http.MethodGet, "/api/admin/bands", "", admin)
	if rec.Code != http.StatusOK {
		t.Fatalf("list bands: %d %s", rec.Code, rec.Body.String())
	}
	var bands []struct {
		Name string `json:"name"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &bands); err != nil {
		t.Fatal(err)
	}
	if len(bands) != 1 || bands[0].Name != "The Quietones" {
		t.Fatalf("bands = %+v", bands)
	}

	rec = jsonReq(e, http.MethodDelete, "/api/admin/bands/9999", "", admin)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("delete missing band: %d, want 404", rec.Code)
	}

	rec = jsonReq(e, http.MethodDelete, fmt.Sprintf("/api/admin/bands/%d", band.ID), "", admin)
	if rec.Code != http.StatusNoContent {
		t.Fatalf("delete band: %d %s", rec.Code, rec.Body.String())
	}
	if _, err := api.Repo.BandByID(band.ID); err == nil {
		t.Error("band survived admin delete")
	}
}

func TestAdminAccessPolicy(t *testing.T) {
	e, api := newAdminAPI(t)
	api.AdminEmails = map[string]bool{"admin@example.com": true}
	admin := signupAndCookie(t, e, "admin")

	rec := jsonReq(e, http.MethodGet, "/api/admin/access-policy", "", admin)
	var policy struct {
		Enabled       bool `json:"enabled"`
		AllowedEmails []struct {
			ID    uint   `json:"id"`
			Email string `json:"email"`
		} `json:"allowedEmails"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &policy); err != nil {
		t.Fatal(err)
	}
	if policy.Enabled || len(policy.AllowedEmails) != 0 {
		t.Fatalf("initial policy = %+v", policy)
	}

	rec = jsonReq(e, http.MethodPut, "/api/admin/access-policy", `{"enabled":true}`, admin)
	if rec.Code != http.StatusNoContent {
		t.Fatalf("enable: %d %s", rec.Code, rec.Body.String())
	}

	rec = jsonReq(e, http.MethodPost, "/api/admin/access-policy/emails",
		`{"email":"Friend@Example.com"}`, admin)
	if rec.Code != http.StatusCreated {
		t.Fatalf("add email: %d %s", rec.Code, rec.Body.String())
	}
	var created struct {
		ID    uint   `json:"id"`
		Email string `json:"email"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &created); err != nil {
		t.Fatal(err)
	}
	if created.Email != "friend@example.com" {
		t.Errorf("email not normalized: %q", created.Email)
	}

	rec = jsonReq(e, http.MethodPost, "/api/admin/access-policy/emails",
		`{"email":"friend@example.com"}`, admin)
	if rec.Code != http.StatusConflict {
		t.Fatalf("duplicate email: %d, want 409", rec.Code)
	}

	rec = jsonReq(e, http.MethodDelete,
		fmt.Sprintf("/api/admin/access-policy/emails/%d", created.ID), "", admin)
	if rec.Code != http.StatusNoContent {
		t.Fatalf("remove email: %d %s", rec.Code, rec.Body.String())
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `just check`
Expected: FAIL — none of the `Admin*` handler methods exist yet (compile error).

- [ ] **Step 3: Implement the handlers**

Create `internal/handlers/admin.go`:

```go
package handlers

import (
	"net/http"
	"strconv"
	"strings"

	"github.com/labstack/echo/v5"

	appmw "github.com/jwhumphries/bandwidth/internal/middleware"
	"github.com/jwhumphries/bandwidth/internal/repository"
)

// adminTargetID parses the :id path parameter shared by the admin delete routes.
func adminTargetID(c *echo.Context) (uint, error) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil || id == 0 {
		return 0, echo.NewHTTPError(http.StatusNotFound, "not found")
	}
	return uint(id), nil
}

// AdminUsers lists every account.
func (a *API) AdminUsers(c *echo.Context) error {
	users, err := a.Repo.AllUsers()
	if err != nil {
		return err
	}
	return c.JSON(http.StatusOK, users)
}

// AdminDeleteUser deletes a user and everything they solely own. Admins
// cannot delete their own account (avoids self-lockout).
func (a *API) AdminDeleteUser(c *echo.Context) error {
	admin := appmw.CurrentUser(c)
	if admin == nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "user not in context")
	}
	id, err := adminTargetID(c)
	if err != nil {
		return err
	}
	if id == admin.ID {
		return echo.NewHTTPError(http.StatusBadRequest, "cannot delete your own account")
	}
	if _, err := a.Repo.UserByID(id); err != nil {
		return notFoundOr(err, "user")
	}
	if err := a.Repo.DeleteUser(id); err != nil {
		return err
	}
	return c.NoContent(http.StatusNoContent)
}

// AdminBands lists every band.
func (a *API) AdminBands(c *echo.Context) error {
	bands, err := a.Repo.AllBands()
	if err != nil {
		return err
	}
	return c.JSON(http.StatusOK, bands)
}

// AdminDeleteBand deletes any band.
func (a *API) AdminDeleteBand(c *echo.Context) error {
	id, err := adminTargetID(c)
	if err != nil {
		return err
	}
	if _, err := a.Repo.BandByID(id); err != nil {
		return notFoundOr(err, "band")
	}
	if err := a.Repo.DeleteBand(id); err != nil {
		return err
	}
	return c.NoContent(http.StatusNoContent)
}

// AdminGetAccessPolicy returns the signup gate state and its allow-list.
func (a *API) AdminGetAccessPolicy(c *echo.Context) error {
	enabled, err := a.Repo.AccessPolicyEnabled()
	if err != nil {
		return err
	}
	emails, err := a.Repo.AllowedEmails()
	if err != nil {
		return err
	}
	return c.JSON(http.StatusOK, map[string]any{
		"enabled":       enabled,
		"allowedEmails": emails,
	})
}

type setAccessPolicyRequest struct {
	Enabled bool `json:"enabled"`
}

// AdminSetAccessPolicy toggles signup gating.
func (a *API) AdminSetAccessPolicy(c *echo.Context) error {
	var req setAccessPolicyRequest
	if err := c.Bind(&req); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid request body")
	}
	if err := a.Repo.SetAccessPolicyEnabled(req.Enabled); err != nil {
		return err
	}
	return c.NoContent(http.StatusNoContent)
}

type addAllowedEmailRequest struct {
	Email string `json:"email"`
}

// AdminAddAllowedEmail adds an email to the signup allow-list.
func (a *API) AdminAddAllowedEmail(c *echo.Context) error {
	admin := appmw.CurrentUser(c)
	if admin == nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "user not in context")
	}
	var req addAllowedEmailRequest
	if err := c.Bind(&req); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid request body")
	}
	email := strings.ToLower(strings.TrimSpace(req.Email))
	if !validEmail(email) {
		return echo.NewHTTPError(http.StatusBadRequest, "a valid email address is required")
	}
	entry, err := a.Repo.AddAllowedEmail(email, admin.ID)
	if err != nil {
		if repository.IsDuplicate(err) {
			return echo.NewHTTPError(http.StatusConflict, "email already on the allow-list")
		}
		return err
	}
	return c.JSON(http.StatusCreated, map[string]any{
		"id":        entry.ID,
		"email":     entry.Email,
		"createdAt": entry.CreatedAt,
	})
}

// AdminRemoveAllowedEmail removes an allow-list entry.
func (a *API) AdminRemoveAllowedEmail(c *echo.Context) error {
	id, err := adminTargetID(c)
	if err != nil {
		return err
	}
	if err := a.Repo.RemoveAllowedEmail(id); err != nil {
		return notFoundOr(err, "allow-list entry")
	}
	return c.NoContent(http.StatusNoContent)
}
```

- [ ] **Step 4: Register the routes**

In `cmd/bandwidth/server.go`, insert this block into `newEcho` right after the `apiGroup.GET("/invites/link/:token", ...)` line and before `dist, err := fs.Sub(static.Dist, "dist")`:

```go
	admin := apiGroup.Group("/admin", appmw.RequireAuth(api.Repo), appmw.RequireAdmin(api.IsAdminEmail))
	admin.GET("/users", api.AdminUsers)
	admin.DELETE("/users/:id", api.AdminDeleteUser)
	admin.GET("/bands", api.AdminBands)
	admin.DELETE("/bands/:id", api.AdminDeleteBand)
	admin.GET("/access-policy", api.AdminGetAccessPolicy)
	admin.PUT("/access-policy", api.AdminSetAccessPolicy)
	admin.POST("/access-policy/emails", api.AdminAddAllowedEmail)
	admin.DELETE("/access-policy/emails/:id", api.AdminRemoveAllowedEmail)
```

- [ ] **Step 5: Run tests to verify they pass**

Run: `just check`
Expected: PASS

- [ ] **Step 6: Commit**

```bash
git add internal/handlers/admin.go internal/handlers/admin_test.go cmd/bandwidth/server.go
git commit -m "Add admin user/band/access-policy endpoints"
```

---

## Task 9: Frontend — types, `RequireAdmin`, nav gating, route

**Files:**
- Modify: `frontend/src/lib/types.ts` (`User.isAdmin` + new admin types)
- Create: `frontend/src/components/RequireAdmin.tsx`
- Test: `frontend/src/components/RequireAdmin.test.tsx`
- Modify: `frontend/src/components/Layout.tsx` (conditional nav link)
- Modify: `frontend/src/App.tsx` (route)

**Interfaces:**
- Consumes: `useMe()` (existing, `frontend/src/hooks/auth.ts`).
- Produces: `User.isAdmin: boolean`; `AdminUser`, `AdminBand`, `AllowedEmail`, `AccessPolicy` types; `<RequireAdmin />` route guard. Tasks 10-11 use these types and the `/admin` route.

- [ ] **Step 1: Add types**

In `frontend/src/lib/types.ts`, add `isAdmin` to `User` and append the new interfaces at the end of the file:

```ts
export interface User {
  id: number;
  username: string;
  email: string;
  totpEnabled: boolean;
  isAdmin: boolean;
}
```

```ts
export interface AdminUser {
  id: number;
  username: string;
  email: string;
  createdAt: string;
}

export interface AdminBand {
  id: number;
  name: string;
  creatorUsername: string;
  memberCount: number;
}

export interface AllowedEmail {
  id: number;
  email: string;
  createdAt: string;
}

export interface AccessPolicy {
  enabled: boolean;
  allowedEmails: AllowedEmail[];
}
```

- [ ] **Step 2: Write the failing test for `RequireAdmin`**

Create `frontend/src/components/RequireAdmin.test.tsx`:

```tsx
import {screen, waitFor} from '@testing-library/react';
import {describe, expect, it, vi} from 'vitest';
import {Route, Routes} from 'react-router';
import {renderWithProviders} from '../test/utils';
import RequireAdmin from './RequireAdmin';

function jsonResponse(status: number, body: unknown) {
  return new Response(JSON.stringify(body), {status});
}

describe('RequireAdmin', () => {
  it('renders the nested route when the user is an admin', async () => {
    vi.stubGlobal(
      'fetch',
      vi.fn().mockResolvedValue(
        jsonResponse(200, {
          id: 1,
          username: 'admin',
          email: 'a@b.c',
          totpEnabled: false,
          isAdmin: true,
        }),
      ),
    );
    renderWithProviders(
      <Routes>
        <Route element={<RequireAdmin />}>
          <Route path="/" element={<p>admin area</p>} />
        </Route>
      </Routes>,
    );
    expect(await screen.findByText('admin area')).toBeInTheDocument();
  });

  it('redirects home when the user is not an admin', async () => {
    vi.stubGlobal(
      'fetch',
      vi.fn().mockResolvedValue(
        jsonResponse(200, {
          id: 1,
          username: 'bob',
          email: 'b@b.c',
          totpEnabled: false,
          isAdmin: false,
        }),
      ),
    );
    renderWithProviders(
      <Routes>
        <Route path="/" element={<p>home</p>} />
        <Route element={<RequireAdmin />}>
          <Route path="/admin" element={<p>admin area</p>} />
        </Route>
      </Routes>,
      {route: '/admin'},
    );
    await waitFor(() => expect(screen.getByText('home')).toBeInTheDocument());
    expect(screen.queryByText('admin area')).not.toBeInTheDocument();
  });
});
```

- [ ] **Step 3: Run test to verify it fails**

Run: `just check`
Expected: FAIL — `frontend/src/components/RequireAdmin.tsx` does not exist.

- [ ] **Step 4: Implement `RequireAdmin`**

Create `frontend/src/components/RequireAdmin.tsx`:

```tsx
import {Navigate, Outlet} from 'react-router';
import {useMe} from '../hooks/auth';

export default function RequireAdmin() {
  const {data, isPending} = useMe();
  if (isPending) {
    return (
      <div className="flex min-h-screen items-center justify-center">
        <span
          className="loading loading-spinner loading-lg"
          aria-label="Loading"
        />
      </div>
    );
  }
  if (!data?.isAdmin) {
    return <Navigate to="/" replace />;
  }
  return <Outlet />;
}
```

- [ ] **Step 5: Run test to verify it passes**

Run: `just check`
Expected: PASS

- [ ] **Step 6: Wire the nav link and route**

In `frontend/src/components/Layout.tsx`, add `Shield` to the `lucide-react` import, import `useMe`, read `me`, and render a conditional `NavItem`:

```tsx
import {
  AudioLines,
  LibraryBig,
  LogOut,
  Moon,
  Shield,
  Sun,
  User,
  Users,
} from 'lucide-react';
import type {ReactNode} from 'react';
import {Link, NavLink, Outlet} from 'react-router';
import {useLogout, useMe} from '../hooks/auth';
import {useMyInvites} from '../hooks/invites';
import {useTheme} from '../lib/theme';
```

```tsx
export default function Layout() {
  const logout = useLogout();
  const {data: invites = []} = useMyInvites();
  const {data: me} = useMe();
  const {theme, toggle} = useTheme();
```

Add the nav item right after the existing `/profile` `NavItem` (before the `<span className="bg-base-300/70 ...` divider):

```tsx
            <NavItem
              to="/profile"
              icon={<User className="size-4" />}
              label="Profile"
            />
            {me?.isAdmin && (
              <NavItem
                to="/admin"
                icon={<Shield className="size-4" />}
                label="Admin"
              />
            )}
```

In `frontend/src/App.tsx`, import `AdminPage` and `RequireAdmin`, and nest the new route inside the existing `Layout` route:

```tsx
import {Route, Routes} from 'react-router';
import Layout from './components/Layout';
import UpdateToast from './components/UpdateToast';
import RequireAdmin from './components/RequireAdmin';
import RequireAuth from './components/RequireAuth';
import AdminPage from './pages/AdminPage';
import BandPage from './pages/BandPage';
import BandSongPage from './pages/BandSongPage';
import BandsPage from './pages/BandsPage';
import ForgotPasswordPage from './pages/ForgotPasswordPage';
import HomePage from './pages/HomePage';
import JoinPage from './pages/JoinPage';
import LoginPage from './pages/LoginPage';
import ProfilePage from './pages/ProfilePage';
import ResetPasswordPage from './pages/ResetPasswordPage';
import SignupPage from './pages/SignupPage';
import SongPage from './pages/SongPage';

export default function App() {
  return (
    <>
      <UpdateToast />
      <Routes>
        <Route path="/login" element={<LoginPage />} />
        <Route path="/signup" element={<SignupPage />} />
        <Route path="/forgot-password" element={<ForgotPasswordPage />} />
        <Route path="/reset-password" element={<ResetPasswordPage />} />
        <Route path="/join/:token" element={<JoinPage />} />
        <Route element={<RequireAuth />}>
          <Route element={<Layout />}>
            <Route path="/" element={<HomePage />} />
            <Route path="/profile" element={<ProfilePage />} />
            <Route path="/songs/:id" element={<SongPage />} />
            <Route path="/bands" element={<BandsPage />} />
            <Route path="/bands/:id" element={<BandPage />} />
            <Route path="/bands/:id/songs/:songId" element={<BandSongPage />} />
            <Route element={<RequireAdmin />}>
              <Route path="/admin" element={<AdminPage />} />
            </Route>
          </Route>
        </Route>
      </Routes>
    </>
  );
}
```

`AdminPage` doesn't exist yet — it's created in Task 11. This will not typecheck until then; that's fine, this task's own test (`RequireAdmin.test.tsx`) doesn't render `App.tsx`. Task 11 is the next task in the plan, so the tree is only briefly red between tasks.

- [ ] **Step 7: Commit**

```bash
git add frontend/src/lib/types.ts frontend/src/components/RequireAdmin.tsx frontend/src/components/RequireAdmin.test.tsx frontend/src/components/Layout.tsx frontend/src/App.tsx
git commit -m "Add isAdmin type, RequireAdmin guard, and admin nav link"
```

---

## Task 10: Frontend — `hooks/admin.ts`

**Files:**
- Create: `frontend/src/hooks/admin.ts`

**Interfaces:**
- Consumes: `api` (from `frontend/src/lib/api.ts`), `AdminUser`, `AdminBand`, `AccessPolicy` (Task 9), backend routes from Task 8 (`/api/admin/users`, `/api/admin/bands`, `/api/admin/access-policy`, `/api/admin/access-policy/emails`).
- Produces: `useAdminUsers`, `useDeleteAdminUser`, `useAdminBands`, `useDeleteAdminBand`, `useAccessPolicy`, `useSetAccessPolicy`, `useAddAllowedEmail`, `useRemoveAllowedEmail`. Task 11 (`AdminPage.tsx`) is the sole consumer.

This task has no dedicated test file, matching the existing convention: hook files (`frontend/src/hooks/bands.ts`, `auth.ts`, etc.) are exercised indirectly through the pages that use them, not tested in isolation. Task 11's `AdminPage.test.tsx` covers this file's behavior.

- [ ] **Step 1: Implement**

Create `frontend/src/hooks/admin.ts`:

```ts
import {useMutation, useQuery, useQueryClient} from '@tanstack/react-query';
import {api, ApiError} from '../lib/api';
import type {AccessPolicy, AdminBand, AdminUser} from '../lib/types';

export function useAdminUsers() {
  return useQuery<AdminUser[], ApiError>({
    queryKey: ['admin', 'users'],
    queryFn: () => api.get<AdminUser[]>('/api/admin/users'),
  });
}

export function useDeleteAdminUser() {
  const queryClient = useQueryClient();
  return useMutation<void, ApiError, number>({
    mutationFn: id => api.delete(`/api/admin/users/${id}`),
    onSuccess: () =>
      void queryClient.invalidateQueries({queryKey: ['admin', 'users']}),
  });
}

export function useAdminBands() {
  return useQuery<AdminBand[], ApiError>({
    queryKey: ['admin', 'bands'],
    queryFn: () => api.get<AdminBand[]>('/api/admin/bands'),
  });
}

export function useDeleteAdminBand() {
  const queryClient = useQueryClient();
  return useMutation<void, ApiError, number>({
    mutationFn: id => api.delete(`/api/admin/bands/${id}`),
    onSuccess: () =>
      void queryClient.invalidateQueries({queryKey: ['admin', 'bands']}),
  });
}

export function useAccessPolicy() {
  return useQuery<AccessPolicy, ApiError>({
    queryKey: ['admin', 'access-policy'],
    queryFn: () => api.get<AccessPolicy>('/api/admin/access-policy'),
  });
}

export function useSetAccessPolicy() {
  const queryClient = useQueryClient();
  return useMutation<void, ApiError, {enabled: boolean}>({
    mutationFn: data => api.put<void>('/api/admin/access-policy', data),
    onSuccess: () =>
      void queryClient.invalidateQueries({
        queryKey: ['admin', 'access-policy'],
      }),
  });
}

export function useAddAllowedEmail() {
  const queryClient = useQueryClient();
  return useMutation<{id: number; email: string}, ApiError, {email: string}>({
    mutationFn: data =>
      api.post<{id: number; email: string}>(
        '/api/admin/access-policy/emails',
        data,
      ),
    onSuccess: () =>
      void queryClient.invalidateQueries({
        queryKey: ['admin', 'access-policy'],
      }),
  });
}

export function useRemoveAllowedEmail() {
  const queryClient = useQueryClient();
  return useMutation<void, ApiError, number>({
    mutationFn: id => api.delete(`/api/admin/access-policy/emails/${id}`),
    onSuccess: () =>
      void queryClient.invalidateQueries({
        queryKey: ['admin', 'access-policy'],
      }),
  });
}
```

- [ ] **Step 2: Typecheck**

Run: `just check`
Expected: This file typechecks cleanly. `App.tsx`/`AdminPage` will still fail until Task 11 lands (see Task 9 Step 6 note) — confirm the only remaining error is the missing `./pages/AdminPage` module.

- [ ] **Step 3: Commit**

```bash
git add frontend/src/hooks/admin.ts
git commit -m "Add admin panel react-query hooks"
```

---

## Task 11: Frontend — `AdminPage.tsx`

**Files:**
- Create: `frontend/src/pages/AdminPage.tsx`
- Test: `frontend/src/pages/AdminPage.test.tsx`

**Interfaces:**
- Consumes: all hooks from Task 10; `ConfirmModal` (`frontend/src/components/songs/ConfirmModal.tsx`, existing, generic — reused as-is); `AdminUser`, `AdminBand`, `AccessPolicy` types (Task 9).
- Produces: default export `AdminPage`, referenced by `App.tsx` (Task 9).

- [ ] **Step 1: Write the failing tests**

Create `frontend/src/pages/AdminPage.test.tsx`:

```tsx
import {screen, waitFor} from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import {beforeEach, describe, expect, it, vi} from 'vitest';
import {renderWithProviders} from '../test/utils';
import AdminPage from './AdminPage';

const users = [
  {
    id: 1,
    username: 'admin',
    email: 'admin@example.com',
    createdAt: '2026-01-01',
  },
  {id: 2, username: 'bob', email: 'bob@example.com', createdAt: '2026-01-02'},
];
const bands = [
  {id: 1, name: 'The Quietones', creatorUsername: 'admin', memberCount: 2},
];
let policy: {
  enabled: boolean;
  allowedEmails: {id: number; email: string; createdAt: string}[];
};

function jsonResponse(status: number, body: unknown) {
  return new Response(JSON.stringify(body), {status});
}

describe('AdminPage', () => {
  beforeEach(() => {
    policy = {enabled: false, allowedEmails: []};
    vi.stubGlobal(
      'fetch',
      vi
        .fn()
        .mockImplementation((input: RequestInfo | URL, init?: RequestInit) => {
          const url = String(input);
          const method = init?.method ?? 'GET';
          if (url.endsWith('/api/admin/users') && method === 'GET') {
            return Promise.resolve(jsonResponse(200, users));
          }
          if (url.includes('/api/admin/users/') && method === 'DELETE') {
            return Promise.resolve(jsonResponse(204, null));
          }
          if (url.endsWith('/api/admin/bands') && method === 'GET') {
            return Promise.resolve(jsonResponse(200, bands));
          }
          if (url.endsWith('/api/admin/access-policy') && method === 'GET') {
            return Promise.resolve(jsonResponse(200, policy));
          }
          if (url.endsWith('/api/admin/access-policy') && method === 'PUT') {
            return Promise.resolve(jsonResponse(204, null));
          }
          if (
            url.endsWith('/api/admin/access-policy/emails') &&
            method === 'POST'
          ) {
            return Promise.resolve(
              jsonResponse(201, {id: 9, email: 'friend@example.com'}),
            );
          }
          return Promise.resolve(jsonResponse(404, {message: 'not found'}));
        }),
    );
  });

  it('lists users and deletes one after confirming', async () => {
    renderWithProviders(<AdminPage />);
    expect(await screen.findByText('bob')).toBeInTheDocument();

    await userEvent.click(screen.getByRole('button', {name: /delete bob/i}));
    await userEvent.click(await screen.findByRole('button', {name: 'Delete'}));

    await waitFor(() => {
      const calls = vi.mocked(fetch).mock.calls;
      expect(
        calls.some(
          ([input, init]) =>
            String(input).endsWith('/api/admin/users/2') &&
            init?.method === 'DELETE',
        ),
      ).toBe(true);
    });
  });

  it('lists bands on the Bands tab', async () => {
    renderWithProviders(<AdminPage />);
    await userEvent.click(screen.getByRole('tab', {name: /bands/i}));
    expect(await screen.findByText('The Quietones')).toBeInTheDocument();
    expect(screen.getByText(/created by admin/i)).toBeInTheDocument();
  });

  it('toggles the access policy and adds an allowed email', async () => {
    renderWithProviders(<AdminPage />);
    await userEvent.click(screen.getByRole('tab', {name: /access policy/i}));
    await screen.findByText(/registration open to anyone/i);

    await userEvent.click(screen.getByRole('checkbox'));
    await waitFor(() => {
      const calls = vi.mocked(fetch).mock.calls;
      expect(
        calls.some(
          ([input, init]) =>
            String(input).endsWith('/api/admin/access-policy') &&
            init?.method === 'PUT' &&
            String(init.body).includes('true'),
        ),
      ).toBe(true);
    });

    await userEvent.type(
      screen.getByPlaceholderText(/friend@example.com/i),
      'friend@example.com',
    );
    await userEvent.click(screen.getByRole('button', {name: /add/i}));
    await waitFor(() => {
      const calls = vi.mocked(fetch).mock.calls;
      expect(
        calls.some(
          ([input, init]) =>
            String(input).endsWith('/api/admin/access-policy/emails') &&
            init?.method === 'POST',
        ),
      ).toBe(true);
    });
  });
});
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `just check`
Expected: FAIL — `frontend/src/pages/AdminPage.tsx` does not exist.

- [ ] **Step 3: Implement**

Create `frontend/src/pages/AdminPage.tsx`:

```tsx
import {Mail, Plus, Shield, Trash2, Users} from 'lucide-react';
import {useState} from 'react';
import type {FormEvent} from 'react';
import ConfirmModal from '../components/songs/ConfirmModal';
import {
  useAccessPolicy,
  useAddAllowedEmail,
  useAdminBands,
  useAdminUsers,
  useDeleteAdminBand,
  useDeleteAdminUser,
  useRemoveAllowedEmail,
  useSetAccessPolicy,
} from '../hooks/admin';

type Tab = 'users' | 'bands' | 'access';

export default function AdminPage() {
  const [tab, setTab] = useState<Tab>('users');

  return (
    <div className="mx-auto flex max-w-3xl flex-col gap-6">
      <h1 className="font-display text-3xl font-bold tracking-tight">
        Admin
      </h1>
      <div role="tablist" className="tabs tabs-boxed w-fit">
        <button
          role="tab"
          className={`tab ${tab === 'users' ? 'tab-active' : ''}`}
          onClick={() => setTab('users')}
        >
          Users
        </button>
        <button
          role="tab"
          className={`tab ${tab === 'bands' ? 'tab-active' : ''}`}
          onClick={() => setTab('bands')}
        >
          Bands
        </button>
        <button
          role="tab"
          className={`tab ${tab === 'access' ? 'tab-active' : ''}`}
          onClick={() => setTab('access')}
        >
          Access policy
        </button>
      </div>
      {tab === 'users' && <AdminUsersPanel />}
      {tab === 'bands' && <AdminBandsPanel />}
      {tab === 'access' && <AdminAccessPolicyPanel />}
    </div>
  );
}

function AdminUsersPanel() {
  const {data: users = []} = useAdminUsers();
  const deleteUser = useDeleteAdminUser();
  const [target, setTarget] = useState<{id: number; username: string} | null>(
    null,
  );

  return (
    <section className="flex flex-col gap-2">
      {users.length === 0 ? (
        <div className="border-base-300/60 text-base-content/60 flex flex-col items-center gap-3 rounded-box border border-dashed py-16 text-center">
          <Users className="text-base-content/30 size-10" />
          <p>No users yet.</p>
        </div>
      ) : (
        <ul className="flex flex-col gap-2">
          {users.map(u => (
            <li
              key={u.id}
              className="border-base-300/60 bg-base-100 flex items-center gap-3 rounded-box border p-4"
            >
              <span className="min-w-0 flex-1 truncate font-display font-semibold">
                {u.username}
              </span>
              <span className="text-base-content/55 truncate text-sm">
                {u.email}
              </span>
              <button
                className="btn btn-error btn-outline btn-sm gap-1.5"
                aria-label={`Delete ${u.username}`}
                onClick={() => setTarget({id: u.id, username: u.username})}
              >
                <Trash2 className="size-4" />
              </button>
            </li>
          ))}
        </ul>
      )}
      {deleteUser.error && (
        <div role="alert" className="alert alert-error">
          {deleteUser.error.message}
        </div>
      )}
      <ConfirmModal
        open={target !== null}
        title="Delete user"
        message={
          target
            ? `Delete "${target.username}" and everything they own, including any band they created?`
            : ''
        }
        confirmLabel="Delete"
        onConfirm={() => {
          if (target) deleteUser.mutate(target.id);
          setTarget(null);
        }}
        onCancel={() => setTarget(null)}
      />
    </section>
  );
}

function AdminBandsPanel() {
  const {data: bands = []} = useAdminBands();
  const deleteBand = useDeleteAdminBand();
  const [target, setTarget] = useState<{id: number; name: string} | null>(
    null,
  );

  return (
    <section className="flex flex-col gap-2">
      {bands.length === 0 ? (
        <div className="border-base-300/60 text-base-content/60 flex flex-col items-center gap-3 rounded-box border border-dashed py-16 text-center">
          <Shield className="text-base-content/30 size-10" />
          <p>No bands yet.</p>
        </div>
      ) : (
        <ul className="flex flex-col gap-2">
          {bands.map(b => (
            <li
              key={b.id}
              className="border-base-300/60 bg-base-100 flex items-center gap-3 rounded-box border p-4"
            >
              <span className="min-w-0 flex-1 truncate font-display font-semibold">
                {b.name}
              </span>
              <span className="text-base-content/55 text-sm">
                created by {b.creatorUsername}
              </span>
              <span className="text-base-content/55 font-mono text-xs">
                {b.memberCount} {b.memberCount === 1 ? 'member' : 'members'}
              </span>
              <button
                className="btn btn-error btn-outline btn-sm gap-1.5"
                aria-label={`Delete ${b.name}`}
                onClick={() => setTarget({id: b.id, name: b.name})}
              >
                <Trash2 className="size-4" />
              </button>
            </li>
          ))}
        </ul>
      )}
      {deleteBand.error && (
        <div role="alert" className="alert alert-error">
          {deleteBand.error.message}
        </div>
      )}
      <ConfirmModal
        open={target !== null}
        title="Delete band"
        message={target ? `Delete "${target.name}" for every member?` : ''}
        confirmLabel="Delete"
        onConfirm={() => {
          if (target) deleteBand.mutate(target.id);
          setTarget(null);
        }}
        onCancel={() => setTarget(null)}
      />
    </section>
  );
}

function AdminAccessPolicyPanel() {
  const {data: policy} = useAccessPolicy();
  const setPolicy = useSetAccessPolicy();
  const addEmail = useAddAllowedEmail();
  const removeEmail = useRemoveAllowedEmail();
  const [email, setEmail] = useState('');

  const submitEmail = (e: FormEvent) => {
    e.preventDefault();
    if (!email.trim()) return;
    addEmail.mutate({email: email.trim()}, {onSuccess: () => setEmail('')});
  };

  return (
    <section className="flex flex-col gap-4">
      <label className="flex items-center gap-3">
        <input
          type="checkbox"
          className="toggle toggle-primary"
          checked={policy?.enabled ?? false}
          onChange={e => setPolicy.mutate({enabled: e.target.checked})}
        />
        <span>
          {policy?.enabled
            ? 'Registration restricted to the allow-list below'
            : 'Registration open to anyone'}
        </span>
      </label>

      <ul className="flex flex-col gap-2">
        {(policy?.allowedEmails ?? []).map(entry => (
          <li
            key={entry.id}
            className="border-base-300/60 bg-base-100 flex items-center gap-3 rounded-box border p-3"
          >
            <Mail className="text-base-content/40 size-4" />
            <span className="min-w-0 flex-1 truncate">{entry.email}</span>
            <button
              className="btn btn-ghost btn-sm btn-square"
              aria-label={`Remove ${entry.email}`}
              onClick={() => removeEmail.mutate(entry.id)}
            >
              <Trash2 className="size-4" />
            </button>
          </li>
        ))}
      </ul>

      <form className="flex gap-2" onSubmit={submitEmail}>
        <input
          type="email"
          className="input min-w-0 flex-1"
          placeholder="friend@example.com"
          value={email}
          onChange={e => setEmail(e.target.value)}
        />
        <button
          className="btn btn-primary gap-1.5"
          disabled={addEmail.isPending}
        >
          <Plus className="size-4" />
          Add
        </button>
      </form>
      {(addEmail.error ?? removeEmail.error) && (
        <div role="alert" className="alert alert-error">
          {(addEmail.error ?? removeEmail.error)?.message}
        </div>
      )}
    </section>
  );
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `just check`
Expected: PASS — this also resolves the `App.tsx` import left dangling since Task 9, so the full `just check` (lint, typecheck, format, Go tests, frontend tests) should be clean end to end.

- [ ] **Step 5: Commit**

```bash
git add frontend/src/pages/AdminPage.tsx frontend/src/pages/AdminPage.test.tsx
git commit -m "Add AdminPage with users, bands, and access-policy tabs"
```

---

## After implementation: one manual deploy step

Once this plan is merged and deployed, the admin panel is unreachable until
at least one admin email is configured. This is a manual, one-time action —
not part of any task above, since it's an infrastructure change, not a code
change:

```bash
flyctl secrets set BANDWIDTH_ADMIN_EMAILS=you@example.com -a bandwidth
```

Multiple admins: comma-separate the list. This redeploys the app (Fly
restarts machines on a secrets change) to pick up the new env var.
</content>
