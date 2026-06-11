# BandWidth Personal Songs Implementation Plan (Plan 3 of 5)

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** The personal song library: songs with status/notes/resources, day-granularity practice logging with undo, playlist-style folders with drag reordering, client-side fuzzy search — backend and frontend.

**Architecture:** Implements the spec's identity + annotation-layers model. A Song is identity only (title/artist + owner); ALL metadata (status, notes, resources, practice events) lives in subject-keyed rows (subject = user now; band columns exist in the schema from day one so Plan 4 adds no churn to these tables, but no band rows are ever written in this plan). Missing annotation = "not_learned"/empty, created lazily on first edit. Practice events are unique per (song, subject, date) with dates as YYYY-MM-DD strings. Folders are playlist-style (a song in many folders), with integer positions reindexed on reorder. List/search/folder-filtering is client-side over one GET /api/songs payload plus GET /api/folders.

**Tech Stack:** Existing Go/Echo/GORM stack + existing React/TanStack Query frontend; new frontend deps: fuse.js (search), @dnd-kit/core + @dnd-kit/sortable + @dnd-kit/utilities (drag reorder).

---

## Conventions for the executor

- Repo root: `/Users/john/code/git/BandWidth`, branch off `main` (e.g. `personal-songs`).
- All verification through `just` recipes (Dagger, Bash timeout 600000 ms): `just test`, `just lint-go`, `just test-frontend`, `just typecheck`, `just lint-js`, `just format-check` (`just format` to fix), full gate `just check` (Tasks 9 and 14). Host commands only for dependency management (`go mod tidy`, `bun add`) and `go doc`.
- Echo v5: handlers `func(c *echo.Context) error`; `c.Bind`, `c.JSON`, `c.NoContent`, `echo.NewHTTPError`; `c.Param("id")` returns string. Handlers nil-guard `appmw.CurrentUser(c)` like the existing ones.
- Existing test helpers to reuse: `internal/repository`'s `testRepo(t)`; `internal/handlers`' `newTestAPI`, `postJSON`, `sessionCookie`, `signupAndCookie`, `jsonReq`.
- Frontend: react-router v7 ('react-router'), explicit vitest imports, `renderWithProviders` from `src/test/utils.tsx`, Prettier via `just format`. Existing `api`/`ApiError` client and hook patterns in `src/hooks/auth.ts`.
- API-shape decisions made in this plan (vs the spec's sketch): `PUT /api/folders/order` added for folder reordering; song detail responses are FLAT (`{id, title, artist, status, notes, resources, lastPracticedAt, practiceCount}`) so Plan 4 can add a nested `band` object non-breakingly; practice endpoints return updated `{lastPracticedAt, practiceCount}`.

## File structure being built

```
internal/model/songs.go             # Song, SongAnnotation, Resource, PracticeEvent,
                                    # Folder, FolderEntry, SongStatus
internal/repository/repository.go   # AutoMigrate gains the six new models
internal/repository/songs.go        # CreateSong, SongsForUser (joined list), SongForUser,
                                    # SaveSong, UpsertAnnotation, DeleteSong (cascade)
internal/repository/practice.go     # LogPractice (idempotent), DeletePractice, PracticeStats
internal/repository/resources.go    # ResourcesForSongUser, CreateResource, UpdateResource, DeleteResource
internal/repository/folders.go      # CreateFolder, FoldersForUser, RenameFolder, DeleteFolder,
                                    # ReorderFolders, SetFolderEntries
internal/handlers/songs.go          # Songs, CreateSong, Song, UpdateSong, DeleteSong
internal/handlers/practice.go       # LogPractice, DeletePractice
internal/handlers/resources.go      # CreateResource, UpdateResource, DeleteResource
internal/handlers/folders.go        # Folders, CreateFolder, UpdateFolder, DeleteFolder,
                                    # ReorderFolders, SetFolderEntries
cmd/bandwidth/server.go             # /api/songs + /api/folders route groups
frontend/src/lib/types.ts           # SongListItem, SongDetail, Resource, Folder, PracticeStats
frontend/src/hooks/songs.ts         # song + practice + resource hooks
frontend/src/hooks/folders.ts       # folder hooks
frontend/src/pages/HomePage.tsx     # becomes the library (search, folders, rows, add)
frontend/src/pages/SongPage.tsx     # song detail
frontend/src/components/songs/*     # SongRow, AddSongModal, ConfirmModal, PracticeButton,
                                    # StatusBadge, ResourceList, FolderPicker
frontend/src/components/folders/*   # FolderSidebar (dnd), SortableSongList (dnd)
frontend/src/App.tsx                # /songs/:id route
```

---

### Task 1: Song domain models + migration

**Files:**
- Create: `internal/model/songs.go`
- Modify: `internal/repository/repository.go` (AutoMigrate list)
- Test: `internal/repository/repository_test.go` (extend table list)

- [ ] **Step 1: Write the failing test**

In `internal/repository/repository_test.go`, extend the table list in `TestOpenMigratesSchema`:

```go
	for _, table := range []string{
		"users", "sessions", "backup_codes", "password_resets",
		"songs", "song_annotations", "resources", "practice_events",
		"folders", "folder_entries",
	} {
```

Run: `just test` (timeout 600000). Expected: FAIL — the six new tables don't exist.

- [ ] **Step 2: Write `internal/model/songs.go`**

```go
package model

import "time"

// SongStatus is the learning state of a song for one subject (user or band).
type SongStatus string

// Song learning statuses, in progression order.
const (
	StatusNotLearned SongStatus = "not_learned"
	StatusLearning   SongStatus = "learning"
	StatusLearned    SongStatus = "learned"
	StatusNailed     SongStatus = "nailed"
)

// Valid reports whether s is a known status.
func (s SongStatus) Valid() bool {
	switch s {
	case StatusNotLearned, StatusLearning, StatusLearned, StatusNailed:
		return true
	}
	return false
}

// Song is identity only: title/artist plus an owner (a user XOR a band).
// All metadata lives in subject-keyed annotation tables. Band ownership
// columns exist now but are only written by the bands plan.
type Song struct {
	ID          uint   `gorm:"primarykey"`
	Title       string `gorm:"not null"`
	Artist      string
	OwnerUserID *uint `gorm:"index"`
	OwnerBandID *uint `gorm:"index"`
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

// SongAnnotation holds one subject's status and notes for one song.
// A missing row reads as StatusNotLearned with empty notes. Partial unique
// indexes (per subject column) are required because SQLite treats NULLs as
// distinct in plain composite unique indexes.
type SongAnnotation struct {
	ID     uint  `gorm:"primarykey"`
	SongID uint  `gorm:"not null;uniqueIndex:idx_annotation_user,where:user_id IS NOT NULL;uniqueIndex:idx_annotation_band,where:band_id IS NOT NULL"`
	UserID *uint `gorm:"uniqueIndex:idx_annotation_user,where:user_id IS NOT NULL"`
	BandID *uint `gorm:"uniqueIndex:idx_annotation_band,where:band_id IS NOT NULL"`
	Status SongStatus `gorm:"not null;default:not_learned"`
	Notes  string
	UpdatedAt time.Time
}

// Resource is a subject-scoped link attached to a song (tab, video, ...).
type Resource struct {
	ID       uint   `gorm:"primarykey"`
	SongID   uint   `gorm:"index;not null"`
	UserID   *uint  `gorm:"index"`
	BandID   *uint  `gorm:"index"`
	URL      string `gorm:"not null"`
	Label    string
	Position int `gorm:"not null"`
}

// PracticeEvent records "this song was practiced on this date" for one
// subject. Date is YYYY-MM-DD; per-subject partial unique indexes dedupe
// per day (plain composite indexes don't fire when the other subject
// column is NULL).
type PracticeEvent struct {
	ID     uint   `gorm:"primarykey"`
	SongID uint   `gorm:"not null;uniqueIndex:idx_practice_user_day,where:user_id IS NOT NULL;uniqueIndex:idx_practice_band_day,where:band_id IS NOT NULL"`
	UserID *uint  `gorm:"uniqueIndex:idx_practice_user_day,where:user_id IS NOT NULL"`
	BandID *uint  `gorm:"uniqueIndex:idx_practice_band_day,where:band_id IS NOT NULL"`
	Date   string `gorm:"not null;uniqueIndex:idx_practice_user_day,where:user_id IS NOT NULL;uniqueIndex:idx_practice_band_day,where:band_id IS NOT NULL"`
}

// Folder is a playlist-style, subject-owned ordered group of songs.
type Folder struct {
	ID          uint   `gorm:"primarykey"`
	Name        string `gorm:"not null"`
	Position    int    `gorm:"not null"`
	OwnerUserID *uint  `gorm:"index"`
	OwnerBandID *uint  `gorm:"index"`
}

// FolderEntry places one song at one position inside one folder.
type FolderEntry struct {
	ID       uint `gorm:"primarykey"`
	FolderID uint `gorm:"uniqueIndex:idx_folder_song;not null"`
	SongID   uint `gorm:"uniqueIndex:idx_folder_song;not null"`
	Position int  `gorm:"not null"`
}
```

- [ ] **Step 3: Extend AutoMigrate**

In `internal/repository/repository.go`, the AutoMigrate call becomes:

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
	); err != nil {
```

- [ ] **Step 4: Run tests, lint, commit**

Run: `just test` and `just lint-go` (timeout 600000 each). Expected: PASS / clean.

```bash
git add internal/model/ internal/repository/
git commit -m "feat: song domain models and migration"
```

---

### Task 2: Song repository (create, list with metadata, update, delete cascade)

**Files:**
- Create: `internal/repository/songs.go`
- Test: `internal/repository/songs_test.go`

- [ ] **Step 1: Write the failing tests**

`internal/repository/songs_test.go`:

```go
package repository

import (
	"testing"

	"github.com/jwhumphries/bandwidth/internal/model"
)

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
```

NOTE: this test file references `CreateResource`, `LogPractice`, `CreateFolder`, `SetFolderEntries` from Tasks 3–5. To keep TDD per-task, Tasks 2–5 are committed TOGETHER only at the end of Task 5 — OR (preferred, do this): write only `TestCreateAndListSongs`, `TestSongForUserVisibility`, `TestUpsertAnnotation` plus a REDUCED `TestDeleteSongCascades` now (drop the resource/practice/folder setup lines and their table checks, keep annotation cascade + non-owner check), and EXTEND the cascade test back to the full version above in Task 5 Step 4. Take the preferred path and say so in your report.

- [ ] **Step 2: Run tests to verify they fail**

Run: `just test` (timeout 600000). Expected: FAIL — undefined: CreateSong (etc.).

- [ ] **Step 3: Write `internal/repository/songs.go`**

```go
package repository

import (
	"fmt"

	"gorm.io/gorm"

	"github.com/jwhumphries/bandwidth/internal/model"
)

// SongListItem is one row of a user's library: identity plus the user's
// own metadata layer, with practice stats pre-aggregated.
type SongListItem struct {
	ID              uint             `json:"id"`
	Title           string           `json:"title"`
	Artist          string           `json:"artist"`
	Status          model.SongStatus `json:"status"`
	LastPracticedAt string           `json:"lastPracticedAt"`
	PracticeCount   int              `json:"practiceCount"`
}

// CreateSong inserts a user-owned song.
func (r *Repo) CreateSong(userID uint, title, artist string) (*model.Song, error) {
	song := &model.Song{Title: title, Artist: artist, OwnerUserID: &userID}
	if err := r.db.Create(song).Error; err != nil {
		return nil, err
	}
	return song, nil
}

// SongsForUser returns the user's library with their annotation layer and
// practice aggregates joined in. Missing annotations read as not_learned.
func (r *Repo) SongsForUser(userID uint) ([]SongListItem, error) {
	items := []SongListItem{}
	err := r.db.Table("songs").
		Select(`songs.id, songs.title, songs.artist,
			COALESCE(sa.status, 'not_learned') AS status,
			COALESCE(pe.last_practiced_at, '') AS last_practiced_at,
			COALESCE(pe.practice_count, 0) AS practice_count`).
		Joins(`LEFT JOIN song_annotations sa
			ON sa.song_id = songs.id AND sa.user_id = ?`, userID).
		Joins(`LEFT JOIN (
			SELECT song_id, MAX(date) AS last_practiced_at, COUNT(*) AS practice_count
			FROM practice_events WHERE user_id = ? GROUP BY song_id
		) pe ON pe.song_id = songs.id`, userID).
		Where("songs.owner_user_id = ?", userID).
		Order("songs.title COLLATE NOCASE, songs.id").
		Scan(&items).Error
	if err != nil {
		return nil, err
	}
	return items, nil
}

// SongForUser returns a song if it is visible to the user (owned by them;
// band visibility arrives with the bands plan).
func (r *Repo) SongForUser(songID, userID uint) (*model.Song, error) {
	var song model.Song
	err := r.db.Where("id = ? AND owner_user_id = ?", songID, userID).
		First(&song).Error
	if err != nil {
		return nil, err
	}
	return &song, nil
}

// SaveSong persists identity changes to a song.
func (r *Repo) SaveSong(song *model.Song) error {
	return r.db.Save(song).Error
}

// AnnotationForSongUser returns the user's annotation row, or
// gorm.ErrRecordNotFound when none exists yet.
func (r *Repo) AnnotationForSongUser(songID, userID uint) (*model.SongAnnotation, error) {
	var ann model.SongAnnotation
	err := r.db.Where("song_id = ? AND user_id = ?", songID, userID).
		First(&ann).Error
	if err != nil {
		return nil, err
	}
	return &ann, nil
}

// UpsertAnnotation lazily creates the user's annotation row and applies any
// provided fields (nil pointers leave the existing value untouched).
func (r *Repo) UpsertAnnotation(songID, userID uint, status *model.SongStatus, notes *string) error {
	ann, err := r.AnnotationForSongUser(songID, userID)
	switch {
	case err == nil:
		// existing row
	case err == gorm.ErrRecordNotFound:
		ann = &model.SongAnnotation{
			SongID: songID,
			UserID: &userID,
			Status: model.StatusNotLearned,
		}
	default:
		return err
	}
	if status != nil {
		ann.Status = *status
	}
	if notes != nil {
		ann.Notes = *notes
	}
	return r.db.Save(ann).Error
}

// DeleteSong removes an owned song and everything attached to it
// (annotations, resources, practice events, folder entries) atomically.
func (r *Repo) DeleteSong(songID, userID uint) error {
	if _, err := r.SongForUser(songID, userID); err != nil {
		return fmt.Errorf("song not found: %w", err)
	}
	return r.db.Transaction(func(tx *gorm.DB) error {
		for _, m := range []any{
			&model.FolderEntry{}, &model.PracticeEvent{},
			&model.Resource{}, &model.SongAnnotation{},
		} {
			if err := tx.Where("song_id = ?", songID).Delete(m).Error; err != nil {
				return err
			}
		}
		return tx.Delete(&model.Song{}, songID).Error
	})
}
```

(Use `errors.Is(err, gorm.ErrRecordNotFound)` instead of `==` if the linter complains; report if so.)

- [ ] **Step 4: Run tests, lint, commit**

Run: `just test` and `just lint-go` (timeout 600000 each). Expected: PASS / clean.

```bash
git add internal/repository/
git commit -m "feat: song repository with annotation upsert and cascade delete"
```

---

### Task 3: Practice repository

**Files:**
- Create: `internal/repository/practice.go`
- Test: `internal/repository/practice_test.go`

- [ ] **Step 1: Write the failing tests**

`internal/repository/practice_test.go`:

```go
package repository

import "testing"

func TestLogPracticeIdempotentPerDay(t *testing.T) {
	repo := testRepo(t)
	user, _ := repo.CreateUser("alice", "alice@example.com", "h")
	song, _ := repo.CreateSong(user.ID, "Wonderwall", "Oasis")

	if err := repo.LogPractice(song.ID, user.ID, "2026-06-10"); err != nil {
		t.Fatalf("LogPractice: %v", err)
	}
	// Same day again: no error, no duplicate.
	if err := repo.LogPractice(song.ID, user.ID, "2026-06-10"); err != nil {
		t.Fatalf("LogPractice(dup): %v", err)
	}
	if err := repo.LogPractice(song.ID, user.ID, "2026-06-11"); err != nil {
		t.Fatalf("LogPractice(day 2): %v", err)
	}

	last, count, err := repo.PracticeStats(song.ID, user.ID)
	if err != nil {
		t.Fatalf("PracticeStats: %v", err)
	}
	if last != "2026-06-11" || count != 2 {
		t.Fatalf("stats = %q/%d, want 2026-06-11/2", last, count)
	}
}

func TestDeletePractice(t *testing.T) {
	repo := testRepo(t)
	user, _ := repo.CreateUser("alice", "alice@example.com", "h")
	song, _ := repo.CreateSong(user.ID, "Wonderwall", "Oasis")
	_ = repo.LogPractice(song.ID, user.ID, "2026-06-10")
	_ = repo.LogPractice(song.ID, user.ID, "2026-06-11")

	if err := repo.DeletePractice(song.ID, user.ID, "2026-06-11"); err != nil {
		t.Fatalf("DeletePractice: %v", err)
	}
	last, count, _ := repo.PracticeStats(song.ID, user.ID)
	if last != "2026-06-10" || count != 1 {
		t.Fatalf("stats after delete = %q/%d", last, count)
	}
	// Deleting a date that isn't logged is a no-op.
	if err := repo.DeletePractice(song.ID, user.ID, "2026-01-01"); err != nil {
		t.Fatalf("DeletePractice(absent): %v", err)
	}
}

func TestPracticeStatsEmpty(t *testing.T) {
	repo := testRepo(t)
	user, _ := repo.CreateUser("alice", "alice@example.com", "h")
	song, _ := repo.CreateSong(user.ID, "Wonderwall", "Oasis")

	last, count, err := repo.PracticeStats(song.ID, user.ID)
	if err != nil || last != "" || count != 0 {
		t.Fatalf("empty stats = %q/%d (%v)", last, count, err)
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `just test`. Expected: FAIL — undefined: LogPractice (etc.).

- [ ] **Step 3: Write `internal/repository/practice.go`**

```go
package repository

import (
	"gorm.io/gorm/clause"

	"github.com/jwhumphries/bandwidth/internal/model"
)

// LogPractice records that the user practiced the song on date (YYYY-MM-DD).
// Logging the same day twice is a no-op.
func (r *Repo) LogPractice(songID, userID uint, date string) error {
	event := &model.PracticeEvent{SongID: songID, UserID: &userID, Date: date}
	return r.db.Clauses(clause.OnConflict{DoNothing: true}).Create(event).Error
}

// DeletePractice removes the user's practice event for one date (no-op when absent).
func (r *Repo) DeletePractice(songID, userID uint, date string) error {
	return r.db.
		Where("song_id = ? AND user_id = ? AND date = ?", songID, userID, date).
		Delete(&model.PracticeEvent{}).Error
}

// PracticeStats returns the user's last practiced date ("" when never) and
// total practiced-day count for one song.
func (r *Repo) PracticeStats(songID, userID uint) (string, int, error) {
	var row struct {
		Last  string
		Count int
	}
	err := r.db.Model(&model.PracticeEvent{}).
		Select("COALESCE(MAX(date), '') AS last, COUNT(*) AS count").
		Where("song_id = ? AND user_id = ?", songID, userID).
		Scan(&row).Error
	if err != nil {
		return "", 0, err
	}
	return row.Last, row.Count, nil
}
```

(If `clause.OnConflict{DoNothing: true}` misbehaves with the composite unique index under gormlite, fall back to checking existence first inside the function and report the deviation.)

- [ ] **Step 4: Run tests, lint, commit**

Run: `just test` and `just lint-go`. Expected: PASS / clean.

```bash
git add internal/repository/
git commit -m "feat: practice event repository"
```

---

### Task 4: Resource repository

**Files:**
- Create: `internal/repository/resources.go`
- Test: `internal/repository/resources_test.go`

- [ ] **Step 1: Write the failing tests**

`internal/repository/resources_test.go`:

```go
package repository

import "testing"

func TestResourceLifecycle(t *testing.T) {
	repo := testRepo(t)
	user, _ := repo.CreateUser("alice", "alice@example.com", "h")
	song, _ := repo.CreateSong(user.ID, "Wonderwall", "Oasis")

	first, err := repo.CreateResource(song.ID, user.ID, "https://example.com/tab", "tab")
	if err != nil {
		t.Fatalf("CreateResource: %v", err)
	}
	second, _ := repo.CreateResource(song.ID, user.ID, "https://example.com/video", "video")
	if second.Position <= first.Position {
		t.Errorf("positions not appended: %d then %d", first.Position, second.Position)
	}

	list, err := repo.ResourcesForSongUser(song.ID, user.ID)
	if err != nil || len(list) != 2 {
		t.Fatalf("ResourcesForSongUser: %d (%v)", len(list), err)
	}
	if list[0].ID != first.ID {
		t.Error("resources not ordered by position")
	}

	url := "https://example.com/tab2"
	updated, err := repo.UpdateResource(first.ID, user.ID, &url, nil)
	if err != nil || updated.URL != url || updated.Label != "tab" {
		t.Fatalf("UpdateResource: %+v (%v)", updated, err)
	}

	if err := repo.DeleteResource(first.ID, user.ID); err != nil {
		t.Fatalf("DeleteResource: %v", err)
	}
	list, _ = repo.ResourcesForSongUser(song.ID, user.ID)
	if len(list) != 1 {
		t.Fatalf("resources after delete = %d", len(list))
	}
}

func TestResourceSubjectIsolation(t *testing.T) {
	repo := testRepo(t)
	alice, _ := repo.CreateUser("alice", "alice@example.com", "h")
	bob, _ := repo.CreateUser("bob", "bob@example.com", "h")
	song, _ := repo.CreateSong(alice.ID, "Wonderwall", "Oasis")
	res, _ := repo.CreateResource(song.ID, alice.ID, "https://example.com", "x")

	if _, err := repo.UpdateResource(res.ID, bob.ID, nil, nil); err == nil {
		t.Error("non-owner updated resource")
	}
	if err := repo.DeleteResource(res.ID, bob.ID); err == nil {
		t.Error("non-owner deleted resource")
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `just test`. Expected: FAIL — undefined: CreateResource (etc.).

- [ ] **Step 3: Write `internal/repository/resources.go`**

```go
package repository

import (
	"github.com/jwhumphries/bandwidth/internal/model"
)

// ResourcesForSongUser returns the user's resources for a song, in position order.
func (r *Repo) ResourcesForSongUser(songID, userID uint) ([]model.Resource, error) {
	resources := []model.Resource{}
	err := r.db.Where("song_id = ? AND user_id = ?", songID, userID).
		Order("position, id").Find(&resources).Error
	if err != nil {
		return nil, err
	}
	return resources, nil
}

// CreateResource appends a resource to the user's list for a song.
func (r *Repo) CreateResource(songID, userID uint, url, label string) (*model.Resource, error) {
	var maxPos int
	err := r.db.Model(&model.Resource{}).
		Select("COALESCE(MAX(position), 0)").
		Where("song_id = ? AND user_id = ?", songID, userID).
		Scan(&maxPos).Error
	if err != nil {
		return nil, err
	}
	res := &model.Resource{
		SongID: songID, UserID: &userID,
		URL: url, Label: label, Position: maxPos + 1,
	}
	if err := r.db.Create(res).Error; err != nil {
		return nil, err
	}
	return res, nil
}

// resourceForUser loads a resource only when it belongs to the user.
func (r *Repo) resourceForUser(resourceID, userID uint) (*model.Resource, error) {
	var res model.Resource
	err := r.db.Where("id = ? AND user_id = ?", resourceID, userID).
		First(&res).Error
	if err != nil {
		return nil, err
	}
	return &res, nil
}

// UpdateResource applies any provided fields to the user's resource.
func (r *Repo) UpdateResource(resourceID, userID uint, url, label *string) (*model.Resource, error) {
	res, err := r.resourceForUser(resourceID, userID)
	if err != nil {
		return nil, err
	}
	if url != nil {
		res.URL = *url
	}
	if label != nil {
		res.Label = *label
	}
	if err := r.db.Save(res).Error; err != nil {
		return nil, err
	}
	return res, nil
}

// DeleteResource removes the user's resource.
func (r *Repo) DeleteResource(resourceID, userID uint) error {
	res, err := r.resourceForUser(resourceID, userID)
	if err != nil {
		return err
	}
	return r.db.Delete(res).Error
}
```

- [ ] **Step 4: Run tests, lint, commit**

Run: `just test` and `just lint-go`. Expected: PASS / clean.

```bash
git add internal/repository/
git commit -m "feat: resource repository"
```

---

### Task 5: Folder repository

**Files:**
- Create: `internal/repository/folders.go`
- Test: `internal/repository/folders_test.go`
- Modify: `internal/repository/songs_test.go` (restore the full cascade test from Task 2)

- [ ] **Step 1: Write the failing tests**

`internal/repository/folders_test.go`:

```go
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
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `just test`. Expected: FAIL — undefined: CreateFolder (etc.).

- [ ] **Step 3: Write `internal/repository/folders.go`**

```go
package repository

import (
	"fmt"

	"gorm.io/gorm"

	"github.com/jwhumphries/bandwidth/internal/model"
)

// FolderWithSongs is a folder plus its ordered song IDs.
type FolderWithSongs struct {
	ID       uint   `json:"id"`
	Name     string `json:"name"`
	Position int    `json:"position"`
	SongIDs  []uint `json:"songIds"`
}

// CreateFolder appends a folder to the user's list.
func (r *Repo) CreateFolder(userID uint, name string) (*model.Folder, error) {
	var maxPos int
	err := r.db.Model(&model.Folder{}).
		Select("COALESCE(MAX(position), 0)").
		Where("owner_user_id = ?", userID).
		Scan(&maxPos).Error
	if err != nil {
		return nil, err
	}
	folder := &model.Folder{Name: name, Position: maxPos + 1, OwnerUserID: &userID}
	if err := r.db.Create(folder).Error; err != nil {
		return nil, err
	}
	return folder, nil
}

// folderForUser loads a folder only when the user owns it.
func (r *Repo) folderForUser(folderID, userID uint) (*model.Folder, error) {
	var folder model.Folder
	err := r.db.Where("id = ? AND owner_user_id = ?", folderID, userID).
		First(&folder).Error
	if err != nil {
		return nil, err
	}
	return &folder, nil
}

// FoldersForUser returns the user's folders in position order, each with
// its ordered song IDs.
func (r *Repo) FoldersForUser(userID uint) ([]FolderWithSongs, error) {
	var folders []model.Folder
	err := r.db.Where("owner_user_id = ?", userID).
		Order("position, id").Find(&folders).Error
	if err != nil {
		return nil, err
	}
	result := make([]FolderWithSongs, 0, len(folders))
	for _, f := range folders {
		var entries []model.FolderEntry
		if err := r.db.Where("folder_id = ?", f.ID).
			Order("position, id").Find(&entries).Error; err != nil {
			return nil, err
		}
		songIDs := make([]uint, 0, len(entries))
		for _, e := range entries {
			songIDs = append(songIDs, e.SongID)
		}
		result = append(result, FolderWithSongs{
			ID: f.ID, Name: f.Name, Position: f.Position, SongIDs: songIDs,
		})
	}
	return result, nil
}

// RenameFolder renames the user's folder.
func (r *Repo) RenameFolder(folderID, userID uint, name string) error {
	folder, err := r.folderForUser(folderID, userID)
	if err != nil {
		return err
	}
	folder.Name = name
	return r.db.Save(folder).Error
}

// DeleteFolder removes the user's folder and its entries; songs are untouched.
func (r *Repo) DeleteFolder(folderID, userID uint) error {
	folder, err := r.folderForUser(folderID, userID)
	if err != nil {
		return err
	}
	return r.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("folder_id = ?", folder.ID).
			Delete(&model.FolderEntry{}).Error; err != nil {
			return err
		}
		return tx.Delete(folder).Error
	})
}

// ReorderFolders applies the given order to the user's folders. IDs not
// owned by the user are rejected.
func (r *Repo) ReorderFolders(userID uint, folderIDs []uint) error {
	return r.db.Transaction(func(tx *gorm.DB) error {
		for i, id := range folderIDs {
			res := tx.Model(&model.Folder{}).
				Where("id = ? AND owner_user_id = ?", id, userID).
				Update("position", i+1)
			if res.Error != nil {
				return res.Error
			}
			if res.RowsAffected != 1 {
				return fmt.Errorf("folder %d not found", id)
			}
		}
		return nil
	})
}

// SetFolderEntries replaces the folder's membership and order with songIDs.
// Every song must be visible to the user.
func (r *Repo) SetFolderEntries(folderID, userID uint, songIDs []uint) error {
	if _, err := r.folderForUser(folderID, userID); err != nil {
		return err
	}
	if len(songIDs) > 0 {
		var visible int64
		err := r.db.Model(&model.Song{}).
			Where("id IN ? AND owner_user_id = ?", songIDs, userID).
			Count(&visible).Error
		if err != nil {
			return err
		}
		if visible != int64(len(songIDs)) {
			return fmt.Errorf("one or more songs not found")
		}
	}
	return r.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("folder_id = ?", folderID).
			Delete(&model.FolderEntry{}).Error; err != nil {
			return err
		}
		for i, songID := range songIDs {
			entry := &model.FolderEntry{FolderID: folderID, SongID: songID, Position: i + 1}
			if err := tx.Create(entry).Error; err != nil {
				return err
			}
		}
		return nil
	})
}
```

(Note: duplicate IDs within songIDs would break the visibility count and the unique index — the handler layer dedupes before calling. The visibility check counts DISTINCT matches; duplicated input produces "not found", which is acceptable.)

- [ ] **Step 4: Restore the full cascade test**

In `internal/repository/songs_test.go`, extend `TestDeleteSongCascades` back to the full version shown in Task 2 Step 1 (resource + practice + folder setup and all five table checks).

- [ ] **Step 5: Run tests, lint, commit**

Run: `just test` and `just lint-go`. Expected: PASS / clean.

```bash
git add internal/repository/
git commit -m "feat: folder repository with ordered playlist-style entries"
```

---

### Task 6: Song handlers (list, create, detail, update, delete)

**Files:**
- Create: `internal/handlers/songs.go`
- Test: `internal/handlers/songs_test.go`

- [ ] **Step 1: Write the failing tests**

`internal/handlers/songs_test.go`:

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

// newSongsAPI registers the personal-song routes on a test server.
func newSongsAPI(t *testing.T) (*echo.Echo, *API) {
	t.Helper()
	e, api := newTestAPI(t)
	g := e.Group("/api/songs", appmw.RequireAuth(api.Repo))
	g.GET("", api.Songs)
	g.POST("", api.CreateSong)
	g.GET("/:id", api.Song)
	g.PATCH("/:id", api.UpdateSong)
	g.DELETE("/:id", api.DeleteSong)
	return e, api
}

func TestSongCRUD(t *testing.T) {
	e, _ := newSongsAPI(t)
	cookie := signupAndCookie(t, e, "alice")

	// Create.
	rec := jsonReq(e, http.MethodPost, "/api/songs",
		`{"title":"Wonderwall","artist":"Oasis"}`, cookie)
	if rec.Code != http.StatusCreated {
		t.Fatalf("create: %d %s", rec.Code, rec.Body.String())
	}
	var created struct {
		ID     uint   `json:"id"`
		Title  string `json:"title"`
		Status string `json:"status"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &created); err != nil || created.ID == 0 {
		t.Fatalf("create body: %s (%v)", rec.Body.String(), err)
	}
	if created.Status != "not_learned" {
		t.Errorf("default status = %q", created.Status)
	}

	// Validation.
	if rec := jsonReq(e, http.MethodPost, "/api/songs", `{"title":"  "}`, cookie); rec.Code != http.StatusBadRequest {
		t.Fatalf("blank title: %d, want 400", rec.Code)
	}

	// List.
	rec = jsonReq(e, http.MethodGet, "/api/songs", "", cookie)
	if rec.Code != http.StatusOK {
		t.Fatalf("list: %d", rec.Code)
	}
	var list []map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &list); err != nil || len(list) != 1 {
		t.Fatalf("list body: %s (%v)", rec.Body.String(), err)
	}

	// Detail.
	rec = jsonReq(e, http.MethodGet, fmt.Sprintf("/api/songs/%d", created.ID), "", cookie)
	if rec.Code != http.StatusOK {
		t.Fatalf("detail: %d %s", rec.Code, rec.Body.String())
	}
	var detail struct {
		Title         string           `json:"title"`
		Notes         string           `json:"notes"`
		Resources     []map[string]any `json:"resources"`
		PracticeCount int              `json:"practiceCount"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &detail); err != nil {
		t.Fatal(err)
	}
	if detail.Title != "Wonderwall" || detail.Resources == nil || detail.PracticeCount != 0 {
		t.Errorf("detail = %+v", detail)
	}

	// Update identity + annotation in one PATCH.
	rec = jsonReq(e, http.MethodPatch, fmt.Sprintf("/api/songs/%d", created.ID),
		`{"artist":"Oasis (1995)","status":"learning","notes":"capo 2"}`, cookie)
	if rec.Code != http.StatusOK {
		t.Fatalf("update: %d %s", rec.Code, rec.Body.String())
	}
	rec = jsonReq(e, http.MethodGet, fmt.Sprintf("/api/songs/%d", created.ID), "", cookie)
	var after struct {
		Artist string `json:"artist"`
		Status string `json:"status"`
		Notes  string `json:"notes"`
	}
	_ = json.Unmarshal(rec.Body.Bytes(), &after)
	if after.Artist != "Oasis (1995)" || after.Status != "learning" || after.Notes != "capo 2" {
		t.Errorf("after update = %+v", after)
	}

	// Bad status value.
	rec = jsonReq(e, http.MethodPatch, fmt.Sprintf("/api/songs/%d", created.ID),
		`{"status":"shredded"}`, cookie)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("bad status: %d, want 400", rec.Code)
	}

	// Delete.
	rec = jsonReq(e, http.MethodDelete, fmt.Sprintf("/api/songs/%d", created.ID), "", cookie)
	if rec.Code != http.StatusNoContent {
		t.Fatalf("delete: %d", rec.Code)
	}
	rec = jsonReq(e, http.MethodGet, fmt.Sprintf("/api/songs/%d", created.ID), "", cookie)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("detail after delete: %d, want 404", rec.Code)
	}
}

func TestSongIsolationBetweenUsers(t *testing.T) {
	e, _ := newSongsAPI(t)
	alice := signupAndCookie(t, e, "alice")
	bob := signupAndCookie(t, e, "bob")

	rec := jsonReq(e, http.MethodPost, "/api/songs", `{"title":"Mine"}`, alice)
	var created struct {
		ID uint `json:"id"`
	}
	_ = json.Unmarshal(rec.Body.Bytes(), &created)

	for _, tc := range []struct {
		method, path, body string
	}{
		{http.MethodGet, fmt.Sprintf("/api/songs/%d", created.ID), ""},
		{http.MethodPatch, fmt.Sprintf("/api/songs/%d", created.ID), `{"title":"Stolen"}`},
		{http.MethodDelete, fmt.Sprintf("/api/songs/%d", created.ID), ""},
	} {
		if rec := jsonReq(e, tc.method, tc.path, tc.body, bob); rec.Code != http.StatusNotFound {
			t.Errorf("%s %s as bob: %d, want 404", tc.method, tc.path, rec.Code)
		}
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `just test`. Expected: FAIL — undefined: (*API).Songs (etc.).

- [ ] **Step 3: Write `internal/handlers/songs.go`**

```go
package handlers

import (
	"errors"
	"net/http"
	"strconv"
	"strings"

	"github.com/labstack/echo/v5"
	"gorm.io/gorm"

	appmw "github.com/jwhumphries/bandwidth/internal/middleware"
	"github.com/jwhumphries/bandwidth/internal/model"
	"github.com/jwhumphries/bandwidth/internal/repository"
)

const (
	maxTitleLen = 200
	maxNotesLen = 10000
)

// songID parses the :id path parameter.
func songID(c *echo.Context) (uint, error) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil || id == 0 {
		return 0, echo.NewHTTPError(http.StatusNotFound, "song not found")
	}
	return uint(id), nil
}

// notFoundOr maps gorm.ErrRecordNotFound to a 404 and passes other errors through.
func notFoundOr(err error, what string) error {
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return echo.NewHTTPError(http.StatusNotFound, what+" not found")
	}
	return err
}

// songDetailResponse builds the flat detail payload for one song. The bands
// plan adds a nested "band" object alongside these fields.
func (a *API) songDetailResponse(song *model.Song, userID uint) (map[string]any, error) {
	status := model.StatusNotLearned
	notes := ""
	if ann, err := a.Repo.AnnotationForSongUser(song.ID, userID); err == nil {
		status = ann.Status
		notes = ann.Notes
	} else if !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, err
	}
	resources, err := a.Repo.ResourcesForSongUser(song.ID, userID)
	if err != nil {
		return nil, err
	}
	last, count, err := a.Repo.PracticeStats(song.ID, userID)
	if err != nil {
		return nil, err
	}
	resList := make([]map[string]any, 0, len(resources))
	for _, r := range resources {
		resList = append(resList, map[string]any{
			"id": r.ID, "url": r.URL, "label": r.Label,
		})
	}
	return map[string]any{
		"id":              song.ID,
		"title":           song.Title,
		"artist":          song.Artist,
		"status":          status,
		"notes":           notes,
		"resources":       resList,
		"lastPracticedAt": last,
		"practiceCount":   count,
	}, nil
}

// Songs returns the user's library list.
func (a *API) Songs(c *echo.Context) error {
	user := appmw.CurrentUser(c)
	if user == nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "user not in context")
	}
	items, err := a.Repo.SongsForUser(user.ID)
	if err != nil {
		return err
	}
	return c.JSON(http.StatusOK, items)
}

type createSongRequest struct {
	Title  string `json:"title"`
	Artist string `json:"artist"`
}

// CreateSong adds a personal song.
func (a *API) CreateSong(c *echo.Context) error {
	user := appmw.CurrentUser(c)
	if user == nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "user not in context")
	}
	var req createSongRequest
	if err := c.Bind(&req); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid request body")
	}
	req.Title = strings.TrimSpace(req.Title)
	req.Artist = strings.TrimSpace(req.Artist)
	if req.Title == "" || len(req.Title) > maxTitleLen || len(req.Artist) > maxTitleLen {
		return echo.NewHTTPError(http.StatusBadRequest,
			"a title (at most 200 characters) is required")
	}
	song, err := a.Repo.CreateSong(user.ID, req.Title, req.Artist)
	if err != nil {
		return err
	}
	detail, err := a.songDetailResponse(song, user.ID)
	if err != nil {
		return err
	}
	return c.JSON(http.StatusCreated, detail)
}

// Song returns one song's detail (identity + the user's metadata layer).
func (a *API) Song(c *echo.Context) error {
	user := appmw.CurrentUser(c)
	if user == nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "user not in context")
	}
	id, err := songID(c)
	if err != nil {
		return err
	}
	song, err := a.Repo.SongForUser(id, user.ID)
	if err != nil {
		return notFoundOr(err, "song")
	}
	detail, err := a.songDetailResponse(song, user.ID)
	if err != nil {
		return err
	}
	return c.JSON(http.StatusOK, detail)
}

type updateSongRequest struct {
	Title  *string `json:"title"`
	Artist *string `json:"artist"`
	Status *string `json:"status"`
	Notes  *string `json:"notes"`
}

// UpdateSong patches identity (owner only) and/or the user's annotation.
func (a *API) UpdateSong(c *echo.Context) error {
	user := appmw.CurrentUser(c)
	if user == nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "user not in context")
	}
	id, err := songID(c)
	if err != nil {
		return err
	}
	song, err := a.Repo.SongForUser(id, user.ID)
	if err != nil {
		return notFoundOr(err, "song")
	}
	var req updateSongRequest
	if err := c.Bind(&req); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid request body")
	}

	if req.Title != nil || req.Artist != nil {
		if req.Title != nil {
			title := strings.TrimSpace(*req.Title)
			if title == "" || len(title) > maxTitleLen {
				return echo.NewHTTPError(http.StatusBadRequest,
					"title must be 1-200 characters")
			}
			song.Title = title
		}
		if req.Artist != nil {
			artist := strings.TrimSpace(*req.Artist)
			if len(artist) > maxTitleLen {
				return echo.NewHTTPError(http.StatusBadRequest,
					"artist must be at most 200 characters")
			}
			song.Artist = artist
		}
		if err := a.Repo.SaveSong(song); err != nil {
			return err
		}
	}

	var status *model.SongStatus
	if req.Status != nil {
		s := model.SongStatus(*req.Status)
		if !s.Valid() {
			return echo.NewHTTPError(http.StatusBadRequest, "invalid status")
		}
		status = &s
	}
	if req.Notes != nil && len(*req.Notes) > maxNotesLen {
		return echo.NewHTTPError(http.StatusBadRequest, "notes too long")
	}
	if status != nil || req.Notes != nil {
		if err := a.Repo.UpsertAnnotation(song.ID, user.ID, status, req.Notes); err != nil {
			return err
		}
	}

	detail, err := a.songDetailResponse(song, user.ID)
	if err != nil {
		return err
	}
	return c.JSON(http.StatusOK, detail)
}

// DeleteSong removes an owned song and all attached data.
func (a *API) DeleteSong(c *echo.Context) error {
	user := appmw.CurrentUser(c)
	if user == nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "user not in context")
	}
	id, err := songID(c)
	if err != nil {
		return err
	}
	if err := a.Repo.DeleteSong(id, user.ID); err != nil {
		return notFoundOr(err, "song")
	}
	return c.NoContent(http.StatusNoContent)
}
```

NOTE: `notFoundOr` wraps `DeleteSong`'s error, which wraps gorm.ErrRecordNotFound with fmt.Errorf("%w") — errors.Is still matches. Verify the 404 test passes.

- [ ] **Step 4: Run tests, lint, commit**

Run: `just test` and `just lint-go`. Expected: PASS / clean.

```bash
git add internal/handlers/
git commit -m "feat: personal song handlers"
```

---

### Task 7: Practice + resource handlers

**Files:**
- Create: `internal/handlers/practice.go`, `internal/handlers/resources.go`
- Test: `internal/handlers/practice_test.go`

- [ ] **Step 1: Write the failing tests**

`internal/handlers/practice_test.go`:

```go
package handlers

import (
	"encoding/json"
	"fmt"
	"net/http"
	"testing"
	"time"

	"github.com/labstack/echo/v5"

	appmw "github.com/jwhumphries/bandwidth/internal/middleware"
)

func newPracticeAPI(t *testing.T) (*echo.Echo, *API) {
	t.Helper()
	e, api := newSongsAPI(t)
	g := e.Group("/api/songs", appmw.RequireAuth(api.Repo))
	g.PUT("/:id/practice", api.LogPractice)
	g.DELETE("/:id/practice/:date", api.DeletePractice)
	g.POST("/:id/resources", api.CreateResource)
	g.PATCH("/:id/resources/:resourceId", api.UpdateResource)
	g.DELETE("/:id/resources/:resourceId", api.DeleteResource)
	return e, api
}

func createSongFor(t *testing.T, e *echo.Echo, cookie *http.Cookie) uint {
	t.Helper()
	rec := jsonReq(e, http.MethodPost, "/api/songs", `{"title":"Wonderwall"}`, cookie)
	if rec.Code != http.StatusCreated {
		t.Fatalf("create song: %d %s", rec.Code, rec.Body.String())
	}
	var created struct {
		ID uint `json:"id"`
	}
	_ = json.Unmarshal(rec.Body.Bytes(), &created)
	return created.ID
}

func TestPracticeEndpoints(t *testing.T) {
	e, _ := newPracticeAPI(t)
	cookie := signupAndCookie(t, e, "alice")
	id := createSongFor(t, e, cookie)

	// Explicit date.
	rec := jsonReq(e, http.MethodPut, fmt.Sprintf("/api/songs/%d/practice", id),
		`{"date":"2026-06-10"}`, cookie)
	if rec.Code != http.StatusOK {
		t.Fatalf("practice: %d %s", rec.Code, rec.Body.String())
	}
	var stats struct {
		LastPracticedAt string `json:"lastPracticedAt"`
		PracticeCount   int    `json:"practiceCount"`
	}
	_ = json.Unmarshal(rec.Body.Bytes(), &stats)
	if stats.LastPracticedAt != "2026-06-10" || stats.PracticeCount != 1 {
		t.Errorf("stats = %+v", stats)
	}

	// Same day again: idempotent.
	rec = jsonReq(e, http.MethodPut, fmt.Sprintf("/api/songs/%d/practice", id),
		`{"date":"2026-06-10"}`, cookie)
	_ = json.Unmarshal(rec.Body.Bytes(), &stats)
	if stats.PracticeCount != 1 {
		t.Errorf("idempotency: count = %d", stats.PracticeCount)
	}

	// Empty body defaults to today.
	rec = jsonReq(e, http.MethodPut, fmt.Sprintf("/api/songs/%d/practice", id), "{}", cookie)
	_ = json.Unmarshal(rec.Body.Bytes(), &stats)
	if stats.LastPracticedAt != time.Now().UTC().Format("2006-01-02") {
		t.Errorf("default date = %q", stats.LastPracticedAt)
	}

	// Bad dates.
	for _, body := range []string{`{"date":"junk"}`, `{"date":"2126-01-01"}`} {
		rec = jsonReq(e, http.MethodPut, fmt.Sprintf("/api/songs/%d/practice", id), body, cookie)
		if rec.Code != http.StatusBadRequest {
			t.Errorf("bad date %s: %d, want 400", body, rec.Code)
		}
	}

	// Undo.
	rec = jsonReq(e, http.MethodDelete,
		fmt.Sprintf("/api/songs/%d/practice/2026-06-10", id), "", cookie)
	if rec.Code != http.StatusOK {
		t.Fatalf("delete practice: %d", rec.Code)
	}
	_ = json.Unmarshal(rec.Body.Bytes(), &stats)
	if stats.PracticeCount != 1 {
		t.Errorf("count after undo = %d, want 1 (today remains)", stats.PracticeCount)
	}
}

func TestResourceEndpoints(t *testing.T) {
	e, _ := newPracticeAPI(t)
	cookie := signupAndCookie(t, e, "alice")
	id := createSongFor(t, e, cookie)

	rec := jsonReq(e, http.MethodPost, fmt.Sprintf("/api/songs/%d/resources", id),
		`{"url":"https://example.com/tab","label":"tab"}`, cookie)
	if rec.Code != http.StatusCreated {
		t.Fatalf("create resource: %d %s", rec.Code, rec.Body.String())
	}
	var res struct {
		ID uint `json:"id"`
	}
	_ = json.Unmarshal(rec.Body.Bytes(), &res)

	// Validation: URL must be http(s).
	rec = jsonReq(e, http.MethodPost, fmt.Sprintf("/api/songs/%d/resources", id),
		`{"url":"javascript:alert(1)"}`, cookie)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("bad url: %d, want 400", rec.Code)
	}

	rec = jsonReq(e, http.MethodPatch,
		fmt.Sprintf("/api/songs/%d/resources/%d", id, res.ID),
		`{"label":"chords"}`, cookie)
	if rec.Code != http.StatusOK {
		t.Fatalf("update resource: %d %s", rec.Code, rec.Body.String())
	}

	rec = jsonReq(e, http.MethodDelete,
		fmt.Sprintf("/api/songs/%d/resources/%d", id, res.ID), "", cookie)
	if rec.Code != http.StatusNoContent {
		t.Fatalf("delete resource: %d", rec.Code)
	}

	// Other users get 404s.
	bob := signupAndCookie(t, e, "bob")
	rec = jsonReq(e, http.MethodPost, fmt.Sprintf("/api/songs/%d/resources", id),
		`{"url":"https://example.com"}`, bob)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("bob create on alice song: %d, want 404", rec.Code)
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `just test`. Expected: FAIL — undefined handlers.

- [ ] **Step 3: Write `internal/handlers/practice.go`**

```go
package handlers

import (
	"net/http"
	"time"

	"github.com/labstack/echo/v5"

	appmw "github.com/jwhumphries/bandwidth/internal/middleware"
)

// validPracticeDate parses a YYYY-MM-DD date and rejects far-future entries
// (one day of slack absorbs timezone differences with the client).
func validPracticeDate(date string) bool {
	parsed, err := time.Parse("2006-01-02", date)
	if err != nil {
		return false
	}
	return parsed.Before(time.Now().UTC().Add(48 * time.Hour))
}

func (a *API) practiceStatsResponse(c *echo.Context, songID, userID uint) error {
	last, count, err := a.Repo.PracticeStats(songID, userID)
	if err != nil {
		return err
	}
	return c.JSON(http.StatusOK, map[string]any{
		"lastPracticedAt": last,
		"practiceCount":   count,
	})
}

type logPracticeRequest struct {
	Date string `json:"date"`
}

// LogPractice records a practice day (default: today, UTC) idempotently.
func (a *API) LogPractice(c *echo.Context) error {
	user := appmw.CurrentUser(c)
	if user == nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "user not in context")
	}
	id, err := songID(c)
	if err != nil {
		return err
	}
	if _, err := a.Repo.SongForUser(id, user.ID); err != nil {
		return notFoundOr(err, "song")
	}
	var req logPracticeRequest
	if err := c.Bind(&req); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid request body")
	}
	if req.Date == "" {
		req.Date = time.Now().UTC().Format("2006-01-02")
	}
	if !validPracticeDate(req.Date) {
		return echo.NewHTTPError(http.StatusBadRequest, "date must be YYYY-MM-DD and not in the future")
	}
	if err := a.Repo.LogPractice(id, user.ID, req.Date); err != nil {
		return err
	}
	return a.practiceStatsResponse(c, id, user.ID)
}

// DeletePractice removes one practiced day (undo).
func (a *API) DeletePractice(c *echo.Context) error {
	user := appmw.CurrentUser(c)
	if user == nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "user not in context")
	}
	id, err := songID(c)
	if err != nil {
		return err
	}
	if _, err := a.Repo.SongForUser(id, user.ID); err != nil {
		return notFoundOr(err, "song")
	}
	date := c.Param("date")
	if !validPracticeDate(date) {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid date")
	}
	if err := a.Repo.DeletePractice(id, user.ID, date); err != nil {
		return err
	}
	return a.practiceStatsResponse(c, id, user.ID)
}
```

- [ ] **Step 4: Write `internal/handlers/resources.go`**

```go
package handlers

import (
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"github.com/labstack/echo/v5"

	appmw "github.com/jwhumphries/bandwidth/internal/middleware"
	"github.com/jwhumphries/bandwidth/internal/model"
)

// validResourceURL accepts absolute http(s) URLs only.
func validResourceURL(raw string) bool {
	u, err := url.Parse(raw)
	return err == nil && (u.Scheme == "http" || u.Scheme == "https") && u.Host != ""
}

func resourceID(c *echo.Context) (uint, error) {
	id, err := strconv.ParseUint(c.Param("resourceId"), 10, 32)
	if err != nil || id == 0 {
		return 0, echo.NewHTTPError(http.StatusNotFound, "resource not found")
	}
	return uint(id), nil
}

func resourceResponse(r *model.Resource) map[string]any {
	return map[string]any{"id": r.ID, "url": r.URL, "label": r.Label}
}

type resourceRequest struct {
	URL   *string `json:"url"`
	Label *string `json:"label"`
}

// CreateResource appends a link to the user's resource list for a song.
func (a *API) CreateResource(c *echo.Context) error {
	user := appmw.CurrentUser(c)
	if user == nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "user not in context")
	}
	id, err := songID(c)
	if err != nil {
		return err
	}
	if _, err := a.Repo.SongForUser(id, user.ID); err != nil {
		return notFoundOr(err, "song")
	}
	var req resourceRequest
	if err := c.Bind(&req); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid request body")
	}
	if req.URL == nil || !validResourceURL(*req.URL) {
		return echo.NewHTTPError(http.StatusBadRequest, "a valid http(s) url is required")
	}
	label := ""
	if req.Label != nil {
		label = strings.TrimSpace(*req.Label)
		if len(label) > maxTitleLen {
			return echo.NewHTTPError(http.StatusBadRequest, "label too long")
		}
	}
	res, err := a.Repo.CreateResource(id, user.ID, *req.URL, label)
	if err != nil {
		return err
	}
	return c.JSON(http.StatusCreated, resourceResponse(res))
}

// UpdateResource patches a resource's url and/or label.
func (a *API) UpdateResource(c *echo.Context) error {
	user := appmw.CurrentUser(c)
	if user == nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "user not in context")
	}
	rid, err := resourceID(c)
	if err != nil {
		return err
	}
	var req resourceRequest
	if err := c.Bind(&req); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid request body")
	}
	if req.URL != nil && !validResourceURL(*req.URL) {
		return echo.NewHTTPError(http.StatusBadRequest, "a valid http(s) url is required")
	}
	if req.Label != nil && len(*req.Label) > maxTitleLen {
		return echo.NewHTTPError(http.StatusBadRequest, "label too long")
	}
	res, err := a.Repo.UpdateResource(rid, user.ID, req.URL, req.Label)
	if err != nil {
		return notFoundOr(err, "resource")
	}
	return c.JSON(http.StatusOK, resourceResponse(res))
}

// DeleteResource removes a resource.
func (a *API) DeleteResource(c *echo.Context) error {
	user := appmw.CurrentUser(c)
	if user == nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "user not in context")
	}
	rid, err := resourceID(c)
	if err != nil {
		return err
	}
	if err := a.Repo.DeleteResource(rid, user.ID); err != nil {
		return notFoundOr(err, "resource")
	}
	return c.NoContent(http.StatusNoContent)
}
```

- [ ] **Step 5: Run tests, lint, commit**

Run: `just test` and `just lint-go`. Expected: PASS / clean.

```bash
git add internal/handlers/
git commit -m "feat: practice and resource handlers"
```

---

### Task 8: Folder handlers

**Files:**
- Create: `internal/handlers/folders.go`
- Test: `internal/handlers/folders_test.go`

- [ ] **Step 1: Write the failing tests**

`internal/handlers/folders_test.go`:

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

func newFoldersAPI(t *testing.T) (*echo.Echo, *API) {
	t.Helper()
	e, api := newSongsAPI(t)
	g := e.Group("/api/folders", appmw.RequireAuth(api.Repo))
	g.GET("", api.Folders)
	g.POST("", api.CreateFolder)
	g.PATCH("/:id", api.UpdateFolder)
	g.DELETE("/:id", api.DeleteFolder)
	g.PUT("/order", api.ReorderFolders)
	g.PUT("/:id/entries", api.SetFolderEntries)
	return e, api
}

func TestFolderEndpoints(t *testing.T) {
	e, _ := newFoldersAPI(t)
	cookie := signupAndCookie(t, e, "alice")
	song := createSongFor(t, e, cookie)

	// Create two folders.
	rec := jsonReq(e, http.MethodPost, "/api/folders", `{"name":"Setlist"}`, cookie)
	if rec.Code != http.StatusCreated {
		t.Fatalf("create folder: %d %s", rec.Code, rec.Body.String())
	}
	var f1 struct {
		ID uint `json:"id"`
	}
	_ = json.Unmarshal(rec.Body.Bytes(), &f1)
	rec = jsonReq(e, http.MethodPost, "/api/folders", `{"name":"Queue"}`, cookie)
	var f2 struct {
		ID uint `json:"id"`
	}
	_ = json.Unmarshal(rec.Body.Bytes(), &f2)

	// Blank name rejected.
	if rec := jsonReq(e, http.MethodPost, "/api/folders", `{"name":" "}`, cookie); rec.Code != http.StatusBadRequest {
		t.Fatalf("blank folder name: %d, want 400", rec.Code)
	}

	// Entries (membership + order).
	rec = jsonReq(e, http.MethodPut, fmt.Sprintf("/api/folders/%d/entries", f1.ID),
		fmt.Sprintf(`{"songIds":[%d]}`, song), cookie)
	if rec.Code != http.StatusNoContent {
		t.Fatalf("set entries: %d %s", rec.Code, rec.Body.String())
	}

	// Unknown song rejected.
	rec = jsonReq(e, http.MethodPut, fmt.Sprintf("/api/folders/%d/entries", f1.ID),
		`{"songIds":[99999]}`, cookie)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("bad entries: %d, want 400", rec.Code)
	}

	// List reflects entries and order.
	rec = jsonReq(e, http.MethodGet, "/api/folders", "", cookie)
	var folders []struct {
		ID      uint   `json:"id"`
		Name    string `json:"name"`
		SongIDs []uint `json:"songIds"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &folders); err != nil || len(folders) != 2 {
		t.Fatalf("folders list: %s (%v)", rec.Body.String(), err)
	}
	if len(folders[0].SongIDs) != 1 || folders[0].SongIDs[0] != song {
		t.Errorf("f1 entries = %v", folders[0].SongIDs)
	}

	// Rename.
	rec = jsonReq(e, http.MethodPatch, fmt.Sprintf("/api/folders/%d", f1.ID),
		`{"name":"Gig"}`, cookie)
	if rec.Code != http.StatusNoContent {
		t.Fatalf("rename: %d", rec.Code)
	}

	// Reorder.
	rec = jsonReq(e, http.MethodPut, "/api/folders/order",
		fmt.Sprintf(`{"folderIds":[%d,%d]}`, f2.ID, f1.ID), cookie)
	if rec.Code != http.StatusNoContent {
		t.Fatalf("reorder: %d %s", rec.Code, rec.Body.String())
	}
	rec = jsonReq(e, http.MethodGet, "/api/folders", "", cookie)
	_ = json.Unmarshal(rec.Body.Bytes(), &folders)
	if folders[0].ID != f2.ID {
		t.Error("reorder not applied")
	}

	// Delete folder; songs survive.
	rec = jsonReq(e, http.MethodDelete, fmt.Sprintf("/api/folders/%d", f1.ID), "", cookie)
	if rec.Code != http.StatusNoContent {
		t.Fatalf("delete folder: %d", rec.Code)
	}
	rec = jsonReq(e, http.MethodGet, "/api/songs", "", cookie)
	var songs []map[string]any
	_ = json.Unmarshal(rec.Body.Bytes(), &songs)
	if len(songs) != 1 {
		t.Errorf("songs after folder delete = %d", len(songs))
	}

	// Cross-user isolation.
	bob := signupAndCookie(t, e, "bob")
	if rec := jsonReq(e, http.MethodPatch, fmt.Sprintf("/api/folders/%d", f2.ID), `{"name":"x"}`, bob); rec.Code != http.StatusNotFound {
		t.Errorf("bob renamed alice folder: %d, want 404", rec.Code)
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `just test`. Expected: FAIL — undefined handlers.

- [ ] **Step 3: Write `internal/handlers/folders.go`**

```go
package handlers

import (
	"net/http"
	"strings"

	"github.com/labstack/echo/v5"

	appmw "github.com/jwhumphries/bandwidth/internal/middleware"
	"github.com/jwhumphries/bandwidth/internal/model"
)

func folderResponse(f *model.Folder) map[string]any {
	return map[string]any{
		"id": f.ID, "name": f.Name, "position": f.Position, "songIds": []uint{},
	}
}

// Folders lists the user's folders with ordered song IDs.
func (a *API) Folders(c *echo.Context) error {
	user := appmw.CurrentUser(c)
	if user == nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "user not in context")
	}
	folders, err := a.Repo.FoldersForUser(user.ID)
	if err != nil {
		return err
	}
	return c.JSON(http.StatusOK, folders)
}

type folderNameRequest struct {
	Name string `json:"name"`
}

// CreateFolder adds a folder.
func (a *API) CreateFolder(c *echo.Context) error {
	user := appmw.CurrentUser(c)
	if user == nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "user not in context")
	}
	var req folderNameRequest
	if err := c.Bind(&req); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid request body")
	}
	req.Name = strings.TrimSpace(req.Name)
	if req.Name == "" || len(req.Name) > maxTitleLen {
		return echo.NewHTTPError(http.StatusBadRequest, "a folder name is required")
	}
	folder, err := a.Repo.CreateFolder(user.ID, req.Name)
	if err != nil {
		return err
	}
	return c.JSON(http.StatusCreated, folderResponse(folder))
}

// UpdateFolder renames a folder.
func (a *API) UpdateFolder(c *echo.Context) error {
	user := appmw.CurrentUser(c)
	if user == nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "user not in context")
	}
	id, err := songID(c) // same :id uint parsing
	if err != nil {
		return echo.NewHTTPError(http.StatusNotFound, "folder not found")
	}
	var req folderNameRequest
	if err := c.Bind(&req); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid request body")
	}
	req.Name = strings.TrimSpace(req.Name)
	if req.Name == "" || len(req.Name) > maxTitleLen {
		return echo.NewHTTPError(http.StatusBadRequest, "a folder name is required")
	}
	if err := a.Repo.RenameFolder(id, user.ID, req.Name); err != nil {
		return notFoundOr(err, "folder")
	}
	return c.NoContent(http.StatusNoContent)
}

// DeleteFolder removes a folder (entries only; songs are untouched).
func (a *API) DeleteFolder(c *echo.Context) error {
	user := appmw.CurrentUser(c)
	if user == nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "user not in context")
	}
	id, err := songID(c)
	if err != nil {
		return echo.NewHTTPError(http.StatusNotFound, "folder not found")
	}
	if err := a.Repo.DeleteFolder(id, user.ID); err != nil {
		return notFoundOr(err, "folder")
	}
	return c.NoContent(http.StatusNoContent)
}

type reorderFoldersRequest struct {
	FolderIDs []uint `json:"folderIds"`
}

// ReorderFolders applies a new folder order.
func (a *API) ReorderFolders(c *echo.Context) error {
	user := appmw.CurrentUser(c)
	if user == nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "user not in context")
	}
	var req reorderFoldersRequest
	if err := c.Bind(&req); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid request body")
	}
	if err := a.Repo.ReorderFolders(user.ID, req.FolderIDs); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "one or more folders not found")
	}
	return c.NoContent(http.StatusNoContent)
}

type folderEntriesRequest struct {
	SongIDs []uint `json:"songIds"`
}

// SetFolderEntries replaces a folder's membership and order.
func (a *API) SetFolderEntries(c *echo.Context) error {
	user := appmw.CurrentUser(c)
	if user == nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "user not in context")
	}
	id, err := songID(c)
	if err != nil {
		return echo.NewHTTPError(http.StatusNotFound, "folder not found")
	}
	var req folderEntriesRequest
	if err := c.Bind(&req); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid request body")
	}
	// Dedupe while preserving first-seen order.
	seen := map[uint]bool{}
	songIDs := make([]uint, 0, len(req.SongIDs))
	for _, sid := range req.SongIDs {
		if !seen[sid] {
			seen[sid] = true
			songIDs = append(songIDs, sid)
		}
	}
	if err := a.Repo.SetFolderEntries(id, user.ID, songIDs); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "folder or songs not found")
	}
	return c.NoContent(http.StatusNoContent)
}
```

(Style note: `songID(c)` is reused for folder :id parsing since it is just uint parsing; if the reviewers find the name confusing, rename the helper to `pathID(c, "id")` — acceptable adaptation, report it.)

- [ ] **Step 4: Run tests, lint, commit**

Run: `just test` and `just lint-go`. Expected: PASS / clean.

```bash
git add internal/handlers/
git commit -m "feat: folder handlers"
```

---

### Task 9: Route wiring + integration test

**Files:**
- Modify: `cmd/bandwidth/server.go`
- Test: `cmd/bandwidth/server_test.go` (one integration flow)

- [ ] **Step 1: Write the failing integration test**

Append to `cmd/bandwidth/server_test.go`:

```go
func TestSongLibraryFlow(t *testing.T) {
	e := testServer(t)

	rec := do(e, http.MethodPost, "/api/auth/signup",
		`{"username":"alice","email":"alice@example.com","password":"hunter2hunter2"}`, nil)
	if rec.Code != http.StatusCreated {
		t.Fatalf("signup: %d", rec.Code)
	}
	cookies := rec.Result().Cookies()

	rec = do(e, http.MethodPost, "/api/songs", `{"title":"Wonderwall","artist":"Oasis"}`, cookies)
	if rec.Code != http.StatusCreated {
		t.Fatalf("create song: %d %s", rec.Code, rec.Body.String())
	}

	rec = do(e, http.MethodGet, "/api/songs", "", cookies)
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), "Wonderwall") {
		t.Fatalf("list: %d %s", rec.Code, rec.Body.String())
	}

	rec = do(e, http.MethodGet, "/api/folders", "", cookies)
	if rec.Code != http.StatusOK {
		t.Fatalf("folders: %d", rec.Code)
	}
}
```

Run: `just test`. Expected: FAIL — 404s (routes not wired).

- [ ] **Step 2: Wire the routes**

In `cmd/bandwidth/server.go` `newEcho`, after the `me` group block, add:

```go
	songs := apiGroup.Group("/songs", appmw.RequireAuth(api.Repo))
	songs.GET("", api.Songs)
	songs.POST("", api.CreateSong)
	songs.GET("/:id", api.Song)
	songs.PATCH("/:id", api.UpdateSong)
	songs.DELETE("/:id", api.DeleteSong)
	songs.PUT("/:id/practice", api.LogPractice)
	songs.DELETE("/:id/practice/:date", api.DeletePractice)
	songs.POST("/:id/resources", api.CreateResource)
	songs.PATCH("/:id/resources/:resourceId", api.UpdateResource)
	songs.DELETE("/:id/resources/:resourceId", api.DeleteResource)

	folders := apiGroup.Group("/folders", appmw.RequireAuth(api.Repo))
	folders.GET("", api.Folders)
	folders.POST("", api.CreateFolder)
	folders.PUT("/order", api.ReorderFolders)
	folders.PATCH("/:id", api.UpdateFolder)
	folders.DELETE("/:id", api.DeleteFolder)
	folders.PUT("/:id/entries", api.SetFolderEntries)
```

(Note `/order` is registered before `/:id` routes for clarity; echo matches static segments first regardless.)

- [ ] **Step 3: Full gate and commit**

Run: `just test` then `just check` (timeout 600000 each). Expected: PASS / `all checks passed`.

```bash
git add cmd/
git commit -m "feat: wire song and folder routes"
```

---

### Task 10: Frontend foundation — types, date helper, song + folder hooks

**Files:**
- Modify: `frontend/src/lib/types.ts` (append)
- Create: `frontend/src/lib/dates.ts`, `frontend/src/lib/dates.test.ts`
- Create: `frontend/src/hooks/songs.ts`, `frontend/src/hooks/folders.ts`

- [ ] **Step 1: Add dependencies (host bun allowed)**

```bash
cd frontend && bun add fuse.js @dnd-kit/core @dnd-kit/sortable @dnd-kit/utilities && cd ..
```

- [ ] **Step 2: Append to `frontend/src/lib/types.ts`**

```ts
export type SongStatus = 'not_learned' | 'learning' | 'learned' | 'nailed';

export interface SongListItem {
  id: number;
  title: string;
  artist: string;
  status: SongStatus;
  lastPracticedAt: string;
  practiceCount: number;
}

export interface Resource {
  id: number;
  url: string;
  label: string;
}

export interface SongDetail extends SongListItem {
  notes: string;
  resources: Resource[];
}

export interface PracticeStats {
  lastPracticedAt: string;
  practiceCount: number;
}

export interface Folder {
  id: number;
  name: string;
  position: number;
  songIds: number[];
}
```

- [ ] **Step 3: Write `frontend/src/lib/dates.ts` + its test (TDD)**

`frontend/src/lib/dates.test.ts`:

```ts
import {describe, expect, it} from 'vitest';
import {localToday} from './dates';

describe('localToday', () => {
  it('returns a YYYY-MM-DD string for the local date', () => {
    const today = localToday();
    expect(today).toMatch(/^\d{4}-\d{2}-\d{2}$/);
    const d = new Date();
    expect(today.startsWith(String(d.getFullYear()))).toBe(true);
  });
});
```

`frontend/src/lib/dates.ts`:

```ts
// localToday returns the user's local calendar date as YYYY-MM-DD. Practice
// days are whatever day it is for the musician, not the server.
export function localToday(): string {
  const d = new Date();
  const pad = (n: number) => String(n).padStart(2, '0');
  return `${d.getFullYear()}-${pad(d.getMonth() + 1)}-${pad(d.getDate())}`;
}
```

- [ ] **Step 4: Write `frontend/src/hooks/songs.ts`**

```ts
import {useMutation, useQuery, useQueryClient} from '@tanstack/react-query';
import {api, ApiError} from '../lib/api';
import type {
  PracticeStats,
  Resource,
  SongDetail,
  SongListItem,
} from '../lib/types';

export function useSongs() {
  return useQuery<SongListItem[], ApiError>({
    queryKey: ['songs'],
    queryFn: () => api.get<SongListItem[]>('/api/songs'),
  });
}

export function useSong(id: number) {
  return useQuery<SongDetail, ApiError>({
    queryKey: ['songs', id],
    queryFn: () => api.get<SongDetail>(`/api/songs/${id}`),
  });
}

export function useCreateSong() {
  const queryClient = useQueryClient();
  return useMutation<SongDetail, ApiError, {title: string; artist: string}>({
    mutationFn: data => api.post<SongDetail>('/api/songs', data),
    onSuccess: () => void queryClient.invalidateQueries({queryKey: ['songs']}),
  });
}

export function useUpdateSong(id: number) {
  const queryClient = useQueryClient();
  return useMutation<
    SongDetail,
    ApiError,
    {title?: string; artist?: string; status?: string; notes?: string}
  >({
    mutationFn: data => api.patch<SongDetail>(`/api/songs/${id}`, data),
    onSuccess: detail => {
      queryClient.setQueryData(['songs', id], detail);
      void queryClient.invalidateQueries({queryKey: ['songs'], exact: true});
    },
  });
}

export function useDeleteSong() {
  const queryClient = useQueryClient();
  return useMutation<void, ApiError, number>({
    mutationFn: id => api.delete(`/api/songs/${id}`),
    onSuccess: () => {
      void queryClient.invalidateQueries({queryKey: ['songs']});
      void queryClient.invalidateQueries({queryKey: ['folders']});
    },
  });
}

// applyStats patches a song's practice stats into both caches.
function applyStats(
  queryClient: ReturnType<typeof useQueryClient>,
  id: number,
  stats: PracticeStats,
) {
  queryClient.setQueryData<SongListItem[] | undefined>(['songs'], list =>
    list?.map(s => (s.id === id ? {...s, ...stats} : s)),
  );
  queryClient.setQueryData<SongDetail | undefined>(['songs', id], d =>
    d ? {...d, ...stats} : d,
  );
}

export function useLogPractice() {
  const queryClient = useQueryClient();
  return useMutation<PracticeStats, ApiError, {id: number; date: string}>({
    mutationFn: ({id, date}) =>
      api.put<PracticeStats>(`/api/songs/${id}/practice`, {date}),
    onSuccess: (stats, {id}) => applyStats(queryClient, id, stats),
  });
}

export function useUndoPractice() {
  const queryClient = useQueryClient();
  return useMutation<PracticeStats, ApiError, {id: number; date: string}>({
    mutationFn: ({id, date}) =>
      api.delete<PracticeStats>(`/api/songs/${id}/practice/${date}`),
    onSuccess: (stats, {id}) => applyStats(queryClient, id, stats),
  });
}

export function useCreateResource(songId: number) {
  const queryClient = useQueryClient();
  return useMutation<Resource, ApiError, {url: string; label: string}>({
    mutationFn: data => api.post<Resource>(`/api/songs/${songId}/resources`, data),
    onSuccess: () =>
      void queryClient.invalidateQueries({queryKey: ['songs', songId]}),
  });
}

export function useUpdateResource(songId: number) {
  const queryClient = useQueryClient();
  return useMutation<Resource, ApiError, {id: number; url?: string; label?: string}>({
    mutationFn: ({id, ...data}) =>
      api.patch<Resource>(`/api/songs/${songId}/resources/${id}`, data),
    onSuccess: () =>
      void queryClient.invalidateQueries({queryKey: ['songs', songId]}),
  });
}

export function useDeleteResource(songId: number) {
  const queryClient = useQueryClient();
  return useMutation<void, ApiError, number>({
    mutationFn: id => api.delete(`/api/songs/${songId}/resources/${id}`),
    onSuccess: () =>
      void queryClient.invalidateQueries({queryKey: ['songs', songId]}),
  });
}
```

NOTE: this uses `api.delete<T>` — the existing client has no `delete` method. Add it to `frontend/src/lib/api.ts` alongside the others:

```ts
  delete: <T = void>(path: string) => request<T>(path, {method: 'DELETE'}),
```

- [ ] **Step 5: Write `frontend/src/hooks/folders.ts`**

```ts
import {useMutation, useQuery, useQueryClient} from '@tanstack/react-query';
import {api, ApiError} from '../lib/api';
import type {Folder} from '../lib/types';

export function useFolders() {
  return useQuery<Folder[], ApiError>({
    queryKey: ['folders'],
    queryFn: () => api.get<Folder[]>('/api/folders'),
  });
}

export function useCreateFolder() {
  const queryClient = useQueryClient();
  return useMutation<Folder, ApiError, {name: string}>({
    mutationFn: data => api.post<Folder>('/api/folders', data),
    onSuccess: () => void queryClient.invalidateQueries({queryKey: ['folders']}),
  });
}

export function useRenameFolder() {
  const queryClient = useQueryClient();
  return useMutation<void, ApiError, {id: number; name: string}>({
    mutationFn: ({id, name}) => api.patch<void>(`/api/folders/${id}`, {name}),
    onSuccess: () => void queryClient.invalidateQueries({queryKey: ['folders']}),
  });
}

export function useDeleteFolder() {
  const queryClient = useQueryClient();
  return useMutation<void, ApiError, number>({
    mutationFn: id => api.delete(`/api/folders/${id}`),
    onSuccess: () => void queryClient.invalidateQueries({queryKey: ['folders']}),
  });
}

export function useReorderFolders() {
  const queryClient = useQueryClient();
  return useMutation<void, ApiError, number[]>({
    mutationFn: folderIds => api.put<void>('/api/folders/order', {folderIds}),
    onMutate: folderIds => {
      // Optimistic: dnd already shows the new order; keep the cache in step.
      queryClient.setQueryData<Folder[] | undefined>(['folders'], folders => {
        if (!folders) return folders;
        const byID = new Map(folders.map(f => [f.id, f]));
        return folderIds
          .map(id => byID.get(id))
          .filter((f): f is Folder => f !== undefined);
      });
    },
    onError: () => void queryClient.invalidateQueries({queryKey: ['folders']}),
  });
}

export function useSetFolderEntries() {
  const queryClient = useQueryClient();
  return useMutation<void, ApiError, {id: number; songIds: number[]}>({
    mutationFn: ({id, songIds}) =>
      api.put<void>(`/api/folders/${id}/entries`, {songIds}),
    onMutate: ({id, songIds}) => {
      queryClient.setQueryData<Folder[] | undefined>(['folders'], folders =>
        folders?.map(f => (f.id === id ? {...f, songIds} : f)),
      );
    },
    onError: () => void queryClient.invalidateQueries({queryKey: ['folders']}),
    onSettled: () => void queryClient.invalidateQueries({queryKey: ['folders']}),
  });
}
```

- [ ] **Step 6: Run checks, commit**

`just test-frontend && just typecheck && just lint-js && just format-check` (timeout 600000 each; `just format` to fix). Expected: all pass (the dates test runs; hooks compile).

```bash
git add frontend
git commit -m "feat: song and folder types, hooks, and date helper"
```

---

### Task 11: Library page (search, rows, practiced + undo, add song)

**Files:**
- Create: `frontend/src/components/songs/StatusBadge.tsx`, `SongRow.tsx`, `AddSongModal.tsx`, `ConfirmModal.tsx`
- Modify: `frontend/src/pages/HomePage.tsx` (becomes the library), `frontend/src/pages/HomePage.test.tsx`

- [ ] **Step 1: Write the failing tests** — replace `frontend/src/pages/HomePage.test.tsx`:

```tsx
import {screen, waitFor} from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import {beforeEach, describe, expect, it, vi} from 'vitest';
import {renderWithProviders} from '../test/utils';
import HomePage from './HomePage';

const songs = [
  {id: 1, title: 'Wonderwall', artist: 'Oasis', status: 'learning', lastPracticedAt: '2026-06-10', practiceCount: 3},
  {id: 2, title: 'Creep', artist: 'Radiohead', status: 'nailed', lastPracticedAt: '', practiceCount: 0},
];

function jsonResponse(status: number, body: unknown) {
  return new Response(JSON.stringify(body), {status});
}

describe('HomePage library', () => {
  beforeEach(() => {
    vi.stubGlobal(
      'fetch',
      vi.fn().mockImplementation((input: RequestInfo | URL, init?: RequestInit) => {
        const url = String(input);
        if (url.includes('/api/songs') && init?.method === 'PUT') {
          return Promise.resolve(
            jsonResponse(200, {lastPracticedAt: '2026-06-11', practiceCount: 4}),
          );
        }
        if (url.includes('/api/songs') && init?.method === 'POST') {
          return Promise.resolve(jsonResponse(201, {...songs[0], id: 3, title: 'New One'}));
        }
        if (url.includes('/api/folders')) {
          return Promise.resolve(jsonResponse(200, []));
        }
        if (url.includes('/api/songs')) {
          return Promise.resolve(jsonResponse(200, songs));
        }
        return Promise.resolve(jsonResponse(200, {id: 1, username: 'alice', email: 'a@b.c', totpEnabled: false}));
      }),
    );
  });

  it('lists songs with status badges', async () => {
    renderWithProviders(<HomePage />);
    expect(await screen.findByText('Wonderwall')).toBeInTheDocument();
    expect(screen.getByText('Creep')).toBeInTheDocument();
    expect(screen.getByText(/nailed!/i)).toBeInTheDocument();
  });

  it('filters with the search box', async () => {
    renderWithProviders(<HomePage />);
    await screen.findByText('Wonderwall');
    await userEvent.type(screen.getByPlaceholderText(/search/i), 'creep');
    await waitFor(() => expect(screen.queryByText('Wonderwall')).not.toBeInTheDocument());
    expect(screen.getByText('Creep')).toBeInTheDocument();
  });

  it('logs practice and offers undo', async () => {
    renderWithProviders(<HomePage />);
    await screen.findByText('Wonderwall');
    const buttons = screen.getAllByRole('button', {name: /practiced/i});
    await userEvent.click(buttons[0]!);
    await waitFor(() =>
      expect(screen.getByRole('button', {name: /undo/i})).toBeInTheDocument(),
    );
  });

  it('adds a song through the modal', async () => {
    renderWithProviders(<HomePage />);
    await screen.findByText('Wonderwall');
    await userEvent.click(screen.getByRole('button', {name: /add song/i}));
    await userEvent.type(screen.getByLabelText(/title/i), 'New One');
    await userEvent.click(screen.getByRole('button', {name: /^add$/i}));
    await waitFor(() => {
      const calls = vi.mocked(fetch).mock.calls;
      expect(
        calls.some(([, init]) => init?.method === 'POST' && String(init.body).includes('New One')),
      ).toBe(true);
    });
  });
});
```

Run: `just test-frontend`. Expected: FAIL (old HomePage).

- [ ] **Step 2: Build the components**

`frontend/src/components/songs/StatusBadge.tsx`:

```tsx
import type {SongStatus} from '../../lib/types';

const styles: Record<SongStatus, {label: string; className: string}> = {
  not_learned: {label: 'Not learned', className: 'badge-ghost'},
  learning: {label: 'Learning', className: 'badge-warning'},
  learned: {label: 'Learned', className: 'badge-info'},
  nailed: {label: 'Nailed!', className: 'badge-success'},
};

export default function StatusBadge({status}: {status: SongStatus}) {
  const s = styles[status] ?? styles.not_learned;
  return <span className={`badge ${s.className}`}>{s.label}</span>;
}
```

`frontend/src/components/songs/SongRow.tsx`:

```tsx
import {Link} from 'react-router';
import {localToday} from '../../lib/dates';
import type {SongListItem} from '../../lib/types';
import StatusBadge from './StatusBadge';

export default function SongRow({
  song,
  onPracticed,
}: {
  song: SongListItem;
  onPracticed: (id: number, date: string) => void;
}) {
  return (
    <li className="bg-base-100 flex items-center gap-3 rounded-box p-3 shadow-sm">
      <Link to={`/songs/${song.id}`} className="min-w-0 flex-1">
        <span className="block truncate font-semibold">{song.title}</span>
        <span className="text-base-content/60 block truncate text-sm">
          {song.artist || '—'}
        </span>
      </Link>
      <StatusBadge status={song.status} />
      <span className="text-base-content/60 hidden text-sm sm:block">
        {song.lastPracticedAt || 'Never practiced'}
      </span>
      <button
        className="btn btn-sm btn-outline"
        onClick={() => onPracticed(song.id, localToday())}
      >
        Practiced
      </button>
    </li>
  );
}
```

`frontend/src/components/songs/AddSongModal.tsx`:

```tsx
import {useState} from 'react';
import type {FormEvent} from 'react';
import {useCreateSong} from '../../hooks/songs';

export default function AddSongModal({
  open,
  onClose,
}: {
  open: boolean;
  onClose: () => void;
}) {
  const createSong = useCreateSong();
  const [title, setTitle] = useState('');
  const [artist, setArtist] = useState('');

  const submit = (e: FormEvent) => {
    e.preventDefault();
    createSong.mutate(
      {title, artist},
      {
        onSuccess: () => {
          setTitle('');
          setArtist('');
          onClose();
        },
      },
    );
  };

  return (
    <dialog className={`modal ${open ? 'modal-open' : ''}`}>
      <div className="modal-box">
        <h3 className="text-lg font-bold">Add song</h3>
        <form onSubmit={submit}>
          <label className="label" htmlFor="song-title">
            Title
          </label>
          <input
            id="song-title"
            className="input w-full"
            value={title}
            onChange={e => setTitle(e.target.value)}
            required
          />
          <label className="label" htmlFor="song-artist">
            Artist
          </label>
          <input
            id="song-artist"
            className="input w-full"
            value={artist}
            onChange={e => setArtist(e.target.value)}
          />
          {createSong.error && (
            <div role="alert" className="alert alert-error mt-2">
              {createSong.error.message}
            </div>
          )}
          <div className="modal-action">
            <button type="button" className="btn" onClick={onClose}>
              Cancel
            </button>
            <button className="btn btn-primary" disabled={createSong.isPending}>
              Add
            </button>
          </div>
        </form>
      </div>
    </dialog>
  );
}
```

`frontend/src/components/songs/ConfirmModal.tsx`:

```tsx
export default function ConfirmModal({
  open,
  title,
  message,
  confirmLabel,
  onConfirm,
  onCancel,
}: {
  open: boolean;
  title: string;
  message: string;
  confirmLabel: string;
  onConfirm: () => void;
  onCancel: () => void;
}) {
  return (
    <dialog className={`modal ${open ? 'modal-open' : ''}`}>
      <div className="modal-box">
        <h3 className="text-lg font-bold">{title}</h3>
        <p className="py-2">{message}</p>
        <div className="modal-action">
          <button className="btn" onClick={onCancel}>
            Cancel
          </button>
          <button className="btn btn-error" onClick={onConfirm}>
            {confirmLabel}
          </button>
        </div>
      </div>
    </dialog>
  );
}
```

- [ ] **Step 3: Rewrite `frontend/src/pages/HomePage.tsx`**

```tsx
import Fuse from 'fuse.js';
import {useEffect, useMemo, useState} from 'react';
import {useLogPractice, useSongs, useUndoPractice} from '../hooks/songs';
import AddSongModal from '../components/songs/AddSongModal';
import SongRow from '../components/songs/SongRow';

interface UndoState {
  songId: number;
  date: string;
  title: string;
}

export default function HomePage() {
  const {data: songs = []} = useSongs();
  const logPractice = useLogPractice();
  const undoPractice = useUndoPractice();
  const [search, setSearch] = useState('');
  const [adding, setAdding] = useState(false);
  const [undo, setUndo] = useState<UndoState | null>(null);

  useEffect(() => {
    if (!undo) return;
    const timer = setTimeout(() => setUndo(null), 6000);
    return () => clearTimeout(timer);
  }, [undo]);

  const fuse = useMemo(
    () => new Fuse(songs, {keys: ['title', 'artist'], threshold: 0.35}),
    [songs],
  );
  const visible = search.trim()
    ? fuse.search(search.trim()).map(r => r.item)
    : songs;

  const practiced = (songId: number, date: string) => {
    const song = songs.find(s => s.id === songId);
    logPractice.mutate(
      {id: songId, date},
      {onSuccess: () => setUndo({songId, date, title: song?.title ?? 'song'})},
    );
  };

  return (
    <div className="flex flex-col gap-4">
      <div className="flex items-center gap-3">
        <input
          className="input flex-1"
          placeholder="Search songs…"
          value={search}
          onChange={e => setSearch(e.target.value)}
        />
        <button className="btn btn-primary" onClick={() => setAdding(true)}>
          Add song
        </button>
      </div>

      {visible.length === 0 ? (
        <p className="text-base-content/60 py-12 text-center">
          {songs.length === 0
            ? 'No songs yet — add your first one.'
            : 'No songs match your search.'}
        </p>
      ) : (
        <ul className="flex flex-col gap-2">
          {visible.map(song => (
            <SongRow key={song.id} song={song} onPracticed={practiced} />
          ))}
        </ul>
      )}

      <AddSongModal open={adding} onClose={() => setAdding(false)} />

      {undo && (
        <div className="toast toast-center">
          <div className="alert alert-success">
            <span>Practiced “{undo.title}”</span>
            <button
              className="btn btn-ghost btn-sm"
              onClick={() => {
                undoPractice.mutate({id: undo.songId, date: undo.date});
                setUndo(null);
              }}
            >
              Undo
            </button>
          </div>
        </div>
      )}
    </div>
  );
}
```

- [ ] **Step 4: Run checks, commit**

All four frontend just checks (timeout 600000; `just format` to fix). Expected: PASS.

```bash
git add frontend
git commit -m "feat: song library page with search, practice logging, and add modal"
```

---

### Task 12: Song detail page

**Files:**
- Create: `frontend/src/pages/SongPage.tsx`, `frontend/src/pages/SongPage.test.tsx`
- Create: `frontend/src/components/songs/ResourceList.tsx`
- Modify: `frontend/src/App.tsx` (add `/songs/:id` route)

- [ ] **Step 1: Write the failing tests** — `frontend/src/pages/SongPage.test.tsx`:

```tsx
import {screen, waitFor} from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import {beforeEach, describe, expect, it, vi} from 'vitest';
import {Route, Routes} from 'react-router';
import {renderWithProviders} from '../test/utils';
import SongPage from './SongPage';

const detail = {
  id: 1,
  title: 'Wonderwall',
  artist: 'Oasis',
  status: 'learning',
  notes: 'capo 2',
  resources: [{id: 5, url: 'https://example.com/tab', label: 'tab'}],
  lastPracticedAt: '2026-06-10',
  practiceCount: 3,
};

function jsonResponse(status: number, body: unknown) {
  return new Response(JSON.stringify(body), {status});
}

function renderSongPage() {
  return renderWithProviders(
    <Routes>
      <Route path="/songs/:id" element={<SongPage />} />
      <Route path="/" element={<p>home</p>} />
    </Routes>,
    {route: '/songs/1'},
  );
}

describe('SongPage', () => {
  beforeEach(() => {
    vi.stubGlobal(
      'fetch',
      vi.fn().mockImplementation((input: RequestInfo | URL, init?: RequestInit) => {
        const url = String(input);
        if (init?.method === 'PATCH') {
          return Promise.resolve(jsonResponse(200, {...detail, status: 'learned'}));
        }
        if (init?.method === 'DELETE') {
          return Promise.resolve(new Response(null, {status: 204}));
        }
        if (url.includes('/api/folders')) {
          return Promise.resolve(jsonResponse(200, []));
        }
        return Promise.resolve(jsonResponse(200, detail));
      }),
    );
  });

  it('renders identity, notes, resources, and practice stats', async () => {
    renderSongPage();
    expect(await screen.findByDisplayValue('Wonderwall')).toBeInTheDocument();
    expect(screen.getByDisplayValue('capo 2')).toBeInTheDocument();
    expect(screen.getByText(/example\.com/)).toBeInTheDocument();
    expect(screen.getByText(/3 days practiced/i)).toBeInTheDocument();
    expect(screen.getByText(/2026-06-10/)).toBeInTheDocument();
  });

  it('changes status via the select', async () => {
    renderSongPage();
    await screen.findByDisplayValue('Wonderwall');
    await userEvent.selectOptions(screen.getByLabelText(/status/i), 'learned');
    await waitFor(() => {
      const calls = vi.mocked(fetch).mock.calls;
      expect(
        calls.some(
          ([, init]) =>
            init?.method === 'PATCH' && String(init.body).includes('learned'),
        ),
      ).toBe(true);
    });
  });

  it('deletes with confirmation and navigates home', async () => {
    renderSongPage();
    await screen.findByDisplayValue('Wonderwall');
    await userEvent.click(screen.getByRole('button', {name: /delete song/i}));
    await userEvent.click(screen.getByRole('button', {name: /^delete$/i}));
    await waitFor(() => expect(screen.getByText('home')).toBeInTheDocument());
  });
});
```

Run: `just test-frontend`. Expected: FAIL — SongPage missing.

- [ ] **Step 2: Write `frontend/src/components/songs/ResourceList.tsx`**

```tsx
import {useState} from 'react';
import type {FormEvent} from 'react';
import {useCreateResource, useDeleteResource} from '../../hooks/songs';
import type {Resource} from '../../lib/types';

export default function ResourceList({
  songId,
  resources,
}: {
  songId: number;
  resources: Resource[];
}) {
  const createResource = useCreateResource(songId);
  const deleteResource = useDeleteResource(songId);
  const [url, setUrl] = useState('');
  const [label, setLabel] = useState('');

  const submit = (e: FormEvent) => {
    e.preventDefault();
    createResource.mutate(
      {url, label},
      {
        onSuccess: () => {
          setUrl('');
          setLabel('');
        },
      },
    );
  };

  return (
    <div className="flex flex-col gap-2">
      {resources.length === 0 && (
        <p className="text-base-content/60 text-sm">
          No links yet — add tabs, videos, or tutorials.
        </p>
      )}
      <ul className="flex flex-col gap-1">
        {resources.map(r => (
          <li key={r.id} className="flex items-center gap-2">
            <a
              className="link min-w-0 flex-1 truncate"
              href={r.url}
              target="_blank"
              rel="noreferrer"
            >
              {r.label || r.url}
            </a>
            <button
              className="btn btn-ghost btn-xs"
              aria-label={`Remove ${r.label || r.url}`}
              onClick={() => deleteResource.mutate(r.id)}
            >
              ✕
            </button>
          </li>
        ))}
      </ul>
      <form className="flex flex-wrap gap-2" onSubmit={submit}>
        <input
          className="input input-sm min-w-0 flex-1"
          placeholder="https://…"
          value={url}
          onChange={e => setUrl(e.target.value)}
          required
        />
        <input
          className="input input-sm w-32"
          placeholder="Label"
          value={label}
          onChange={e => setLabel(e.target.value)}
        />
        <button className="btn btn-sm" disabled={createResource.isPending}>
          Add link
        </button>
      </form>
      {createResource.error && (
        <div role="alert" className="alert alert-error">
          {createResource.error.message}
        </div>
      )}
    </div>
  );
}
```

- [ ] **Step 3: Write `frontend/src/pages/SongPage.tsx`**

```tsx
import {useEffect, useState} from 'react';
import type {FormEvent} from 'react';
import {useNavigate, useParams} from 'react-router';
import ConfirmModal from '../components/songs/ConfirmModal';
import ResourceList from '../components/songs/ResourceList';
import FolderPicker from '../components/folders/FolderPicker';
import {
  useDeleteSong,
  useLogPractice,
  useSong,
  useUpdateSong,
} from '../hooks/songs';
import {localToday} from '../lib/dates';
import type {SongStatus} from '../lib/types';

const statusOptions: {value: SongStatus; label: string}[] = [
  {value: 'not_learned', label: 'Not learned'},
  {value: 'learning', label: 'Learning'},
  {value: 'learned', label: 'Learned'},
  {value: 'nailed', label: 'Nailed!'},
];

export default function SongPage() {
  const {id: idParam} = useParams();
  const id = Number(idParam);
  const navigate = useNavigate();
  const {data: song} = useSong(id);
  const updateSong = useUpdateSong(id);
  const deleteSong = useDeleteSong();
  const logPractice = useLogPractice();

  const [title, setTitle] = useState('');
  const [artist, setArtist] = useState('');
  const [notes, setNotes] = useState('');
  const [dirty, setDirty] = useState(false);
  const [backfill, setBackfill] = useState('');
  const [confirming, setConfirming] = useState(false);

  useEffect(() => {
    if (song && !dirty) {
      setTitle(song.title);
      setArtist(song.artist);
      setNotes(song.notes);
    }
  }, [song, dirty]);

  if (!song) {
    return (
      <div className="flex justify-center py-12">
        <span className="loading loading-spinner" aria-label="Loading" />
      </div>
    );
  }

  const save = (e: FormEvent) => {
    e.preventDefault();
    updateSong.mutate(
      {title, artist, notes},
      {onSuccess: () => setDirty(false)},
    );
  };

  return (
    <div className="flex flex-col gap-6">
      <form className="card bg-base-100 shadow" onSubmit={save}>
        <div className="card-body">
          <label className="label" htmlFor="title">
            Title
          </label>
          <input
            id="title"
            className="input w-full"
            value={title}
            onChange={e => {
              setDirty(true);
              setTitle(e.target.value);
            }}
            required
          />
          <label className="label" htmlFor="artist">
            Artist
          </label>
          <input
            id="artist"
            className="input w-full"
            value={artist}
            onChange={e => {
              setDirty(true);
              setArtist(e.target.value);
            }}
          />
          <label className="label" htmlFor="status">
            Status
          </label>
          <select
            id="status"
            className="select w-full"
            value={song.status}
            onChange={e => updateSong.mutate({status: e.target.value})}
          >
            {statusOptions.map(o => (
              <option key={o.value} value={o.value}>
                {o.label}
              </option>
            ))}
          </select>
          <label className="label" htmlFor="notes">
            Notes
          </label>
          <textarea
            id="notes"
            className="textarea min-h-32 w-full"
            value={notes}
            onChange={e => {
              setDirty(true);
              setNotes(e.target.value);
            }}
          />
          {updateSong.error && (
            <div role="alert" className="alert alert-error">
              {updateSong.error.message}
            </div>
          )}
          <div className="card-actions justify-end">
            <button className="btn btn-primary" disabled={updateSong.isPending}>
              Save
            </button>
          </div>
        </div>
      </form>

      <section className="card bg-base-100 shadow">
        <div className="card-body">
          <h2 className="card-title">Practice</h2>
          <p>
            {song.practiceCount} days practiced
            {song.lastPracticedAt && <> · last on {song.lastPracticedAt}</>}
          </p>
          <div className="flex flex-wrap items-center gap-2">
            <button
              className="btn btn-outline"
              onClick={() => logPractice.mutate({id, date: localToday()})}
            >
              Practiced today
            </button>
            <input
              type="date"
              className="input w-44"
              aria-label="Backfill date"
              value={backfill}
              onChange={e => setBackfill(e.target.value)}
            />
            <button
              className="btn btn-ghost"
              disabled={!backfill}
              onClick={() => {
                logPractice.mutate({id, date: backfill});
                setBackfill('');
              }}
            >
              Log past day
            </button>
          </div>
        </div>
      </section>

      <section className="card bg-base-100 shadow">
        <div className="card-body">
          <h2 className="card-title">Links</h2>
          <ResourceList songId={id} resources={song.resources} />
        </div>
      </section>

      <section className="card bg-base-100 shadow">
        <div className="card-body">
          <h2 className="card-title">Folders</h2>
          <FolderPicker songId={id} />
        </div>
      </section>

      <section className="card bg-base-100 shadow">
        <div className="card-body">
          <h2 className="card-title">Danger zone</h2>
          <div className="card-actions">
            <button className="btn btn-error btn-outline" onClick={() => setConfirming(true)}>
              Delete song
            </button>
          </div>
        </div>
      </section>

      <ConfirmModal
        open={confirming}
        title="Delete song"
        message={`Delete “${song.title}” and all of its notes, links, and practice history?`}
        confirmLabel="Delete"
        onConfirm={() =>
          deleteSong.mutate(id, {onSuccess: () => void navigate('/')})
        }
        onCancel={() => setConfirming(false)}
      />
    </div>
  );
}
```

NOTE: `FolderPicker` arrives in Task 13. For THIS task, create a minimal placeholder `frontend/src/components/folders/FolderPicker.tsx` that Task 13 replaces:

```tsx
export default function FolderPicker({songId}: {songId: number}) {
  void songId;
  return <p className="text-base-content/60 text-sm">Folder assignment coming next.</p>;
}
```

- [ ] **Step 4: Add the route** — in `frontend/src/App.tsx` inside the Layout route group:

```tsx
          <Route path="/songs/:id" element={<SongPage />} />
```

(plus the import).

- [ ] **Step 5: Run checks, commit**

All four frontend just checks. Expected: PASS.

```bash
git add frontend
git commit -m "feat: song detail page with practice, resources, and delete"
```

---

### Task 13: Folders UI — sidebar with drag reorder, folder filtering, song reorder, folder picker

**Files:**
- Create: `frontend/src/components/folders/FolderSidebar.tsx`, `SortableSongList.tsx`, `FolderSidebar.test.tsx`
- Replace: `frontend/src/components/folders/FolderPicker.tsx` (real implementation + test in `FolderPicker.test.tsx`)
- Modify: `frontend/src/pages/HomePage.tsx` (sidebar + folder filtering + reorder mode)

- [ ] **Step 1: Write the failing tests**

`frontend/src/components/folders/FolderPicker.test.tsx`:

```tsx
import {screen, waitFor} from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import {beforeEach, describe, expect, it, vi} from 'vitest';
import {renderWithProviders} from '../../test/utils';
import FolderPicker from './FolderPicker';

const folders = [
  {id: 1, name: 'Setlist', position: 1, songIds: [7]},
  {id: 2, name: 'Queue', position: 2, songIds: []},
];

describe('FolderPicker', () => {
  beforeEach(() => {
    vi.stubGlobal(
      'fetch',
      vi.fn().mockImplementation((input: RequestInfo | URL, init?: RequestInit) => {
        if (init?.method === 'PUT') {
          return Promise.resolve(new Response(null, {status: 204}));
        }
        return Promise.resolve(
          new Response(JSON.stringify(folders), {status: 200}),
        );
      }),
    );
  });

  it('checks folders containing the song and toggles membership', async () => {
    renderWithProviders(<FolderPicker songId={7} />);
    const setlist = await screen.findByLabelText('Setlist');
    const queue = screen.getByLabelText('Queue');
    expect(setlist).toBeChecked();
    expect(queue).not.toBeChecked();

    await userEvent.click(queue);
    await waitFor(() => {
      const calls = vi.mocked(fetch).mock.calls;
      expect(
        calls.some(
          ([input, init]) =>
            String(input).includes('/api/folders/2/entries') &&
            init?.method === 'PUT' &&
            String(init.body).includes('7'),
        ),
      ).toBe(true);
    });
  });
});
```

`frontend/src/components/folders/FolderSidebar.test.tsx`:

```tsx
import {screen, waitFor} from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import {beforeEach, describe, expect, it, vi} from 'vitest';
import {renderWithProviders} from '../../test/utils';
import FolderSidebar from './FolderSidebar';

const folders = [
  {id: 1, name: 'Setlist', position: 1, songIds: []},
];

describe('FolderSidebar', () => {
  beforeEach(() => {
    vi.stubGlobal(
      'fetch',
      vi.fn().mockImplementation((input: RequestInfo | URL, init?: RequestInit) => {
        if (init?.method === 'POST') {
          return Promise.resolve(
            new Response(
              JSON.stringify({id: 9, name: 'New folder', position: 2, songIds: []}),
              {status: 201},
            ),
          );
        }
        return Promise.resolve(
          new Response(JSON.stringify(folders), {status: 200}),
        );
      }),
    );
  });

  it('lists folders, selects one, and creates new ones', async () => {
    const onSelect = vi.fn();
    renderWithProviders(<FolderSidebar selectedId={null} onSelect={onSelect} />);

    await userEvent.click(await screen.findByText('Setlist'));
    expect(onSelect).toHaveBeenCalledWith(1);

    await userEvent.type(screen.getByPlaceholderText(/new folder/i), 'Gigs');
    await userEvent.click(screen.getByRole('button', {name: /create/i}));
    await waitFor(() => {
      const calls = vi.mocked(fetch).mock.calls;
      expect(calls.some(([, init]) => init?.method === 'POST')).toBe(true);
    });
  });
});
```

Run: `just test-frontend`. Expected: FAIL.

- [ ] **Step 2: Replace `frontend/src/components/folders/FolderPicker.tsx`**

```tsx
import {useFolders, useSetFolderEntries} from '../../hooks/folders';

export default function FolderPicker({songId}: {songId: number}) {
  const {data: folders = []} = useFolders();
  const setEntries = useSetFolderEntries();

  if (folders.length === 0) {
    return (
      <p className="text-base-content/60 text-sm">
        No folders yet — create one from the library page.
      </p>
    );
  }

  const toggle = (folderId: number, member: boolean) => {
    const folder = folders.find(f => f.id === folderId);
    if (!folder) return;
    const songIds = member
      ? [...folder.songIds, songId]
      : folder.songIds.filter(id => id !== songId);
    setEntries.mutate({id: folderId, songIds});
  };

  return (
    <ul className="flex flex-col gap-1">
      {folders.map(f => {
        const member = f.songIds.includes(songId);
        return (
          <li key={f.id}>
            <label className="label cursor-pointer justify-start gap-3">
              <input
                type="checkbox"
                className="checkbox checkbox-sm"
                checked={member}
                onChange={() => toggle(f.id, !member)}
                aria-label={f.name}
              />
              <span>{f.name}</span>
            </label>
          </li>
        );
      })}
    </ul>
  );
}
```

- [ ] **Step 3: Write `frontend/src/components/folders/FolderSidebar.tsx`**

```tsx
import {DndContext, closestCenter} from '@dnd-kit/core';
import type {DragEndEvent} from '@dnd-kit/core';
import {
  SortableContext,
  useSortable,
  verticalListSortingStrategy,
} from '@dnd-kit/sortable';
import {CSS} from '@dnd-kit/utilities';
import {useState} from 'react';
import type {FormEvent} from 'react';
import ConfirmModal from '../songs/ConfirmModal';
import {
  useCreateFolder,
  useDeleteFolder,
  useFolders,
  useRenameFolder,
  useReorderFolders,
} from '../../hooks/folders';
import type {Folder} from '../../lib/types';

function SortableFolderRow({
  folder,
  selected,
  onSelect,
  onRename,
  onDelete,
}: {
  folder: Folder;
  selected: boolean;
  onSelect: () => void;
  onRename: (name: string) => void;
  onDelete: () => void;
}) {
  const {attributes, listeners, setNodeRef, transform, transition} =
    useSortable({id: folder.id});
  const [editing, setEditing] = useState(false);
  const [name, setName] = useState(folder.name);

  const submitRename = (e: FormEvent) => {
    e.preventDefault();
    if (name.trim()) {
      onRename(name.trim());
    }
    setEditing(false);
  };

  return (
    <li
      ref={setNodeRef}
      style={{transform: CSS.Transform.toString(transform), transition}}
      className={`flex items-center gap-1 rounded-box px-2 py-1 ${
        selected ? 'bg-base-300' : ''
      }`}
    >
      <button
        className="cursor-grab touch-none"
        aria-label={`Reorder ${folder.name}`}
        {...attributes}
        {...listeners}
      >
        ⠿
      </button>
      {editing ? (
        <form onSubmit={submitRename} className="flex-1">
          <input
            className="input input-xs w-full"
            value={name}
            onChange={e => setName(e.target.value)}
            autoFocus
            onBlur={submitRename}
          />
        </form>
      ) : (
        <button className="min-w-0 flex-1 truncate text-left" onClick={onSelect}>
          {folder.name}
        </button>
      )}
      <button
        className="btn btn-ghost btn-xs"
        aria-label={`Rename ${folder.name}`}
        onClick={() => {
          setName(folder.name);
          setEditing(true);
        }}
      >
        ✎
      </button>
      <button
        className="btn btn-ghost btn-xs"
        aria-label={`Delete ${folder.name}`}
        onClick={onDelete}
      >
        ✕
      </button>
    </li>
  );
}

export default function FolderSidebar({
  selectedId,
  onSelect,
}: {
  selectedId: number | null;
  onSelect: (id: number | null) => void;
}) {
  const {data: folders = []} = useFolders();
  const createFolder = useCreateFolder();
  const renameFolder = useRenameFolder();
  const deleteFolder = useDeleteFolder();
  const reorderFolders = useReorderFolders();
  const [newName, setNewName] = useState('');
  const [deleting, setDeleting] = useState<Folder | null>(null);

  const create = (e: FormEvent) => {
    e.preventDefault();
    if (!newName.trim()) return;
    createFolder.mutate({name: newName.trim()}, {onSuccess: () => setNewName('')});
  };

  const dragEnd = (event: DragEndEvent) => {
    const {active, over} = event;
    if (!over || active.id === over.id) return;
    const ids = folders.map(f => f.id);
    const from = ids.indexOf(Number(active.id));
    const to = ids.indexOf(Number(over.id));
    ids.splice(to, 0, ...ids.splice(from, 1));
    reorderFolders.mutate(ids);
  };

  return (
    <aside className="w-full sm:w-56">
      <ul className="menu bg-base-100 rounded-box w-full p-2">
        <li>
          <button
            className={selectedId === null ? 'active' : ''}
            onClick={() => onSelect(null)}
          >
            All songs
          </button>
        </li>
      </ul>
      <DndContext collisionDetection={closestCenter} onDragEnd={dragEnd}>
        <SortableContext
          items={folders.map(f => f.id)}
          strategy={verticalListSortingStrategy}
        >
          <ul className="mt-2 flex flex-col gap-1">
            {folders.map(f => (
              <SortableFolderRow
                key={f.id}
                folder={f}
                selected={selectedId === f.id}
                onSelect={() => onSelect(f.id)}
                onRename={name => renameFolder.mutate({id: f.id, name})}
                onDelete={() => setDeleting(f)}
              />
            ))}
          </ul>
        </SortableContext>
      </DndContext>
      <form className="mt-3 flex gap-1" onSubmit={create}>
        <input
          className="input input-sm min-w-0 flex-1"
          placeholder="New folder…"
          value={newName}
          onChange={e => setNewName(e.target.value)}
        />
        <button className="btn btn-sm" disabled={createFolder.isPending}>
          Create
        </button>
      </form>
      <ConfirmModal
        open={deleting !== null}
        title="Delete folder"
        message={`Delete “${deleting?.name ?? ''}”? Songs in it are not deleted.`}
        confirmLabel="Delete"
        onConfirm={() => {
          if (deleting) {
            deleteFolder.mutate(deleting.id, {
              onSuccess: () => {
                if (selectedId === deleting.id) onSelect(null);
              },
            });
          }
          setDeleting(null);
        }}
        onCancel={() => setDeleting(null)}
      />
    </aside>
  );
}
```

- [ ] **Step 4: Write `frontend/src/components/folders/SortableSongList.tsx`**

```tsx
import {DndContext, closestCenter} from '@dnd-kit/core';
import type {DragEndEvent} from '@dnd-kit/core';
import {
  SortableContext,
  useSortable,
  verticalListSortingStrategy,
} from '@dnd-kit/sortable';
import {CSS} from '@dnd-kit/utilities';
import SongRow from '../songs/SongRow';
import type {SongListItem} from '../../lib/types';

function SortableSongRow({
  song,
  onPracticed,
}: {
  song: SongListItem;
  onPracticed: (id: number, date: string) => void;
}) {
  const {attributes, listeners, setNodeRef, transform, transition} =
    useSortable({id: song.id});
  return (
    <div
      ref={setNodeRef}
      style={{transform: CSS.Transform.toString(transform), transition}}
      className="flex items-center gap-1"
    >
      <button
        className="cursor-grab touch-none px-1"
        aria-label={`Reorder ${song.title}`}
        {...attributes}
        {...listeners}
      >
        ⠿
      </button>
      <div className="min-w-0 flex-1">
        <ul>
          <SongRow song={song} onPracticed={onPracticed} />
        </ul>
      </div>
    </div>
  );
}

// SortableSongList renders folder songs in folder order with drag reorder.
export default function SortableSongList({
  songs,
  onPracticed,
  onReorder,
}: {
  songs: SongListItem[];
  onPracticed: (id: number, date: string) => void;
  onReorder: (songIds: number[]) => void;
}) {
  const dragEnd = (event: DragEndEvent) => {
    const {active, over} = event;
    if (!over || active.id === over.id) return;
    const ids = songs.map(s => s.id);
    const from = ids.indexOf(Number(active.id));
    const to = ids.indexOf(Number(over.id));
    ids.splice(to, 0, ...ids.splice(from, 1));
    onReorder(ids);
  };

  return (
    <DndContext collisionDetection={closestCenter} onDragEnd={dragEnd}>
      <SortableContext
        items={songs.map(s => s.id)}
        strategy={verticalListSortingStrategy}
      >
        <div className="flex flex-col gap-2">
          {songs.map(song => (
            <SortableSongRow key={song.id} song={song} onPracticed={onPracticed} />
          ))}
        </div>
      </SortableContext>
    </DndContext>
  );
}
```

- [ ] **Step 5: Integrate into `frontend/src/pages/HomePage.tsx`**

Update HomePage: add folder state + sidebar + filtered/sortable views. The full revised component:

```tsx
import Fuse from 'fuse.js';
import {useEffect, useMemo, useState} from 'react';
import FolderSidebar from '../components/folders/FolderSidebar';
import SortableSongList from '../components/folders/SortableSongList';
import AddSongModal from '../components/songs/AddSongModal';
import SongRow from '../components/songs/SongRow';
import {useFolders, useSetFolderEntries} from '../hooks/folders';
import {useLogPractice, useSongs, useUndoPractice} from '../hooks/songs';

interface UndoState {
  songId: number;
  date: string;
  title: string;
}

export default function HomePage() {
  const {data: songs = []} = useSongs();
  const {data: folders = []} = useFolders();
  const logPractice = useLogPractice();
  const undoPractice = useUndoPractice();
  const setEntries = useSetFolderEntries();
  const [search, setSearch] = useState('');
  const [adding, setAdding] = useState(false);
  const [undo, setUndo] = useState<UndoState | null>(null);
  const [folderId, setFolderId] = useState<number | null>(null);

  useEffect(() => {
    if (!undo) return;
    const timer = setTimeout(() => setUndo(null), 6000);
    return () => clearTimeout(timer);
  }, [undo]);

  const selectedFolder = folders.find(f => f.id === folderId) ?? null;

  // Folder view shows the folder's songs in folder order.
  const folderSongs = useMemo(() => {
    if (!selectedFolder) return songs;
    const byID = new Map(songs.map(s => [s.id, s]));
    return selectedFolder.songIds
      .map(id => byID.get(id))
      .filter((s): s is NonNullable<typeof s> => s !== undefined);
  }, [songs, selectedFolder]);

  const fuse = useMemo(
    () => new Fuse(folderSongs, {keys: ['title', 'artist'], threshold: 0.35}),
    [folderSongs],
  );
  const searching = search.trim() !== '';
  const visible = searching
    ? fuse.search(search.trim()).map(r => r.item)
    : folderSongs;

  const practiced = (songId: number, date: string) => {
    const song = songs.find(s => s.id === songId);
    logPractice.mutate(
      {id: songId, date},
      {onSuccess: () => setUndo({songId, date, title: song?.title ?? 'song'})},
    );
  };

  return (
    <div className="flex flex-col gap-4 sm:flex-row">
      <FolderSidebar selectedId={folderId} onSelect={setFolderId} />

      <div className="flex min-w-0 flex-1 flex-col gap-4">
        <div className="flex items-center gap-3">
          <input
            className="input flex-1"
            placeholder="Search songs…"
            value={search}
            onChange={e => setSearch(e.target.value)}
          />
          <button className="btn btn-primary" onClick={() => setAdding(true)}>
            Add song
          </button>
        </div>

        {visible.length === 0 ? (
          <p className="text-base-content/60 py-12 text-center">
            {songs.length === 0
              ? 'No songs yet — add your first one.'
              : 'No songs here.'}
          </p>
        ) : selectedFolder && !searching ? (
          <SortableSongList
            songs={visible}
            onPracticed={practiced}
            onReorder={songIds =>
              setEntries.mutate({id: selectedFolder.id, songIds})
            }
          />
        ) : (
          <ul className="flex flex-col gap-2">
            {visible.map(song => (
              <SongRow key={song.id} song={song} onPracticed={practiced} />
            ))}
          </ul>
        )}

        <AddSongModal open={adding} onClose={() => setAdding(false)} />

        {undo && (
          <div className="toast toast-center">
            <div className="alert alert-success">
              <span>Practiced “{undo.title}”</span>
              <button
                className="btn btn-ghost btn-sm"
                onClick={() => {
                  undoPractice.mutate({id: undo.songId, date: undo.date});
                  setUndo(null);
                }}
              >
                Undo
              </button>
            </div>
          </div>
        )}
      </div>
    </div>
  );
}
```

(The existing HomePage tests must still pass — they stub /api/folders with []. If a test needs adjustment because of the sidebar markup, adjust assertions minimally and report.)

- [ ] **Step 6: Run checks, commit**

All four frontend just checks. Expected: PASS.

```bash
git add frontend
git commit -m "feat: folders ui with drag reordering and folder picker"
```

---

### Task 14: Docs and final verification

**Files:**
- Modify: `AGENTS.md`, `README.md`

- [ ] **Step 1: Update `AGENTS.md`** (verify each claim as you edit):

1. Architecture: extend the `internal/handlers/` bullet's parenthetical to include `songs, practice, resources, folders`. Extend the `internal/model/` bullet to mention `Song, SongAnnotation, Resource, PracticeEvent, Folder, FolderEntry`.
2. After the Configuration section, add:

```markdown
## Domain model

Songs are identity-only (title/artist + owner); ALL metadata — status
(`not_learned|learning|learned|nailed`), notes, resources, practice events —
lives in subject-keyed rows (subject = a user now; band columns exist but
are unwritten until the bands plan). A missing annotation row reads as
not_learned/empty and is created lazily on first edit. Practice events are
unique per (song, subject, date) with YYYY-MM-DD string dates; the client
sends its local date. Folders are playlist-style (one song may be in many
folders) with integer positions reindexed on reorder; deleting a folder
never deletes songs.
```

3. README: update the Stack paragraph: replace "Planned per the design doc: song and band tracking, installable PWA, single container on fly.io." with "Personal song tracking (status, notes, links, practice days, folders) is implemented. Planned per the design doc: bands, installable PWA, single container on fly.io."

- [ ] **Step 2: Final verification**

Run: `just check` (timeout 600000). Expected: `all checks passed`.
Run: `git status --porcelain` — only the two doc files.

- [ ] **Step 3: Commit**

```bash
git add AGENTS.md README.md
git commit -m "docs: document song domain model"
```

---

## Done criteria

- `just check` green (all six gates).
- Through the dev loop: add songs, search them, set status/notes, attach links, log practice (today + backfill) with undo, create/rename/delete/reorder folders, assign songs to multiple folders, drag-reorder songs inside a folder, delete a song with confirmation.
- All metadata stored in subject-keyed annotation tables; no band rows written anywhere.
- Next: Plan 4 (Bands) gets written against this codebase.

