# Band Folders & Personal-Folder Cross-Inclusion Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Give bands playlist-style folders (the band-keyed mirror of personal folders), let a member's personal folders also hold band songs surfaced via interleaving, and extend the conversion engine so a member's folder placements survive losing access to a band song.

**Architecture:** Folders already carry both `owner_user_id` and `owner_band_id` (nullable) on one table; band folders reuse that ownership column exactly as band songs reused `owner_band_id`. The existing private `subj` value (a user XOR a band) gains an `ownerScope()` method so the folder repository methods are written once against an owner filter and exposed as unchanged user-keyed wrappers plus new band-keyed methods. Band folder writes are Editor+/reads Viewer, gated by the existing `bandAccess` helper; non-members get 404. Personal `SetFolderEntries` widens its visibility check from "songs the user owns" to "any song visible to the user" (owned or band-member), so a band song can be dragged into a personal folder. The conversion engine (`convertBandSongForUser`) gains folder-entry awareness: a member who has placed a band song in a personal folder counts as having "touched" it, and their personal folder entries are re-pointed onto the freshly created personal copy alongside their annotation/resource/practice rows.

**Tech Stack:** Go 1.26 + Echo v5.1.1, GORM + CGO-free SQLite (ncruces/gormlite), React 19 + TypeScript + Vite + Tailwind v4/DaisyUI 5 + TanStack Query 5 + @dnd-kit + react-router v7, just + Dagger CI. All verification runs through `just` recipes (never host `go`/`bun`).

---

## File Structure

**Backend**
- `internal/repository/subject.go` — add `ownerScope()` to the existing `subj` value.
- `internal/repository/folders.go` — refactor to owner-keyed private cores; keep the existing user-keyed public methods unchanged; add band-keyed methods. Widen the user-folder entry visibility check to "visible to the user".
- `internal/repository/bandsongs.go` — extend `userTouchedSong` and `convertBandSongForUser` to count and re-point a member's personal folder entries.
- `internal/handlers/bandfolders.go` (new) — band folder handlers, all `bandAccess`-gated.
- `cmd/bandwidth/server.go` — wire band folder routes into the `bands` group.

**Frontend**
- `frontend/src/lib/types.ts` — no new type needed (`Folder` is reused); confirm only.
- `frontend/src/hooks/bandfolders.ts` (new) — band folder query/mutation hooks.
- `frontend/src/components/bands/BandFolderSidebar.tsx` (new) — band folder list with create/rename/delete/reorder for Editors+, read-only for Viewers.
- `frontend/src/pages/BandPage.tsx` — mount the band folder sidebar and filter the band song list by the selected folder.
- `frontend/src/components/folders/FolderPicker.tsx` — unchanged; verify it lets a band song (now visible on the personal SongPage) be added to personal folders.

**Tests** live alongside source: `*_test.go` for Go, `*.test.tsx` for frontend.

**Docs**
- `AGENTS.md`, `README.md` — document band folders and the widened conversion rule.

---

### Task 1: Owner-keyed folder repository (band folder methods)

**Files:**
- Modify: `internal/repository/subject.go`
- Modify: `internal/repository/folders.go`
- Test: `internal/repository/bandfolders_test.go` (new)

- [ ] **Step 1: Write the failing test** — `internal/repository/bandfolders_test.go`:

```go
package repository

import (
	"errors"
	"testing"

	"gorm.io/gorm"
)

func TestBandFolderCRUDAndEntries(t *testing.T) {
	repo := testRepo(t)
	alice := signupUser(t, repo, "alice")
	band := createBandForRepo(t, repo, alice, "The Quietones")

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
	otherBand := createBandForRepo(t, repo, alice, "Other")
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
```

(`testRepo`, `signupUser`, and `createBandForRepo` are existing repository test helpers — confirm their names in `internal/repository/*_test.go` and adapt the calls if they differ. If no `createBandForRepo` helper exists, create the band inline with `repo.CreateBand(alice, "name")` matching the real signature.)

Run: `just test`. Expected: FAIL — `repo.CreateBandFolder` undefined.

- [ ] **Step 2: Add `ownerScope()` to the `subj` value** in `internal/repository/subject.go`. After the existing `scope()` method add:

```go
// ownerScope filters owner-keyed tables (folders) to this subject, requiring
// the other owner column to be NULL so a user filter never matches a band row
// and vice versa.
func (s subj) ownerScope() (string, uint) {
	if s.userID != nil {
		return "owner_user_id = ? AND owner_band_id IS NULL", *s.userID
	}
	return "owner_band_id = ? AND owner_user_id IS NULL", *s.bandID
}
```

- [ ] **Step 3: Refactor `internal/repository/folders.go` to owner-keyed cores.** Replace the body of the file (keep the `FolderWithSongs` type and imports) with private cores parameterized by `subj`, the existing public user methods delegating to them, and new band methods. Replace everything from `CreateFolder` through `SetFolderEntries` with:

```go
// CreateFolder appends a folder to the user's list.
func (r *Repo) CreateFolder(userID uint, name string) (*model.Folder, error) {
	return r.createFolder(userSubj(userID), name)
}

// CreateBandFolder appends a folder to the band's list.
func (r *Repo) CreateBandFolder(bandID uint, name string) (*model.Folder, error) {
	return r.createFolder(bandSubj(bandID), name)
}

func (r *Repo) createFolder(s subj, name string) (*model.Folder, error) {
	cond, id := s.ownerScope()
	var maxPos int
	err := r.db.Model(&model.Folder{}).
		Select("COALESCE(MAX(position), 0)").
		Where(cond, id).Scan(&maxPos).Error
	if err != nil {
		return nil, err
	}
	folder := &model.Folder{Name: name, Position: maxPos + 1, OwnerUserID: s.userID, OwnerBandID: s.bandID}
	if err := r.db.Create(folder).Error; err != nil {
		return nil, err
	}
	return folder, nil
}

// folderForOwner loads a folder only when this subject owns it.
func (r *Repo) folderForOwner(folderID uint, s subj) (*model.Folder, error) {
	var folder model.Folder
	cond, id := s.ownerScope()
	err := r.db.Where("id = ? AND "+cond, folderID, id).First(&folder).Error
	if err != nil {
		return nil, err
	}
	return &folder, nil
}

// FoldersForUser returns the user's folders in position order with song IDs.
func (r *Repo) FoldersForUser(userID uint) ([]FolderWithSongs, error) {
	return r.foldersForOwner(userSubj(userID))
}

// FoldersForBand returns the band's folders in position order with song IDs.
func (r *Repo) FoldersForBand(bandID uint) ([]FolderWithSongs, error) {
	return r.foldersForOwner(bandSubj(bandID))
}

func (r *Repo) foldersForOwner(s subj) ([]FolderWithSongs, error) {
	cond, id := s.ownerScope()
	var folders []model.Folder
	err := r.db.Where(cond, id).Order("position, id").Find(&folders).Error
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
	return r.renameFolder(folderID, userSubj(userID), name)
}

// RenameBandFolder renames the band's folder.
func (r *Repo) RenameBandFolder(folderID, bandID uint, name string) error {
	return r.renameFolder(folderID, bandSubj(bandID), name)
}

func (r *Repo) renameFolder(folderID uint, s subj, name string) error {
	folder, err := r.folderForOwner(folderID, s)
	if err != nil {
		return err
	}
	folder.Name = name
	return r.db.Save(folder).Error
}

// DeleteFolder removes the user's folder and its entries; songs are untouched.
func (r *Repo) DeleteFolder(folderID, userID uint) error {
	return r.deleteFolder(folderID, userSubj(userID))
}

// DeleteBandFolder removes the band's folder and its entries; songs untouched.
func (r *Repo) DeleteBandFolder(folderID, bandID uint) error {
	return r.deleteFolder(folderID, bandSubj(bandID))
}

func (r *Repo) deleteFolder(folderID uint, s subj) error {
	folder, err := r.folderForOwner(folderID, s)
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

// ReorderFolders applies the given order to the user's folders.
func (r *Repo) ReorderFolders(userID uint, folderIDs []uint) error {
	return r.reorderFolders(userSubj(userID), folderIDs)
}

// ReorderBandFolders applies the given order to the band's folders.
func (r *Repo) ReorderBandFolders(bandID uint, folderIDs []uint) error {
	return r.reorderFolders(bandSubj(bandID), folderIDs)
}

func (r *Repo) reorderFolders(s subj, folderIDs []uint) error {
	cond, id := s.ownerScope()
	return r.db.Transaction(func(tx *gorm.DB) error {
		for i, fid := range folderIDs {
			res := tx.Model(&model.Folder{}).
				Where("id = ? AND "+cond, fid, id).
				Update("position", i+1)
			if res.Error != nil {
				return res.Error
			}
			if res.RowsAffected != 1 {
				return fmt.Errorf("folder %d not found: %w", fid, gorm.ErrRecordNotFound)
			}
		}
		return nil
	})
}

// SetFolderEntries replaces a user folder's membership and order. Every song
// must be visible to the user (owned or shared by a band they belong to).
func (r *Repo) SetFolderEntries(folderID, userID uint, songIDs []uint) error {
	return r.setFolderEntries(folderID, userSubj(userID), songIDs)
}

// SetBandFolderEntries replaces a band folder's membership and order. Every
// song must be owned by this band.
func (r *Repo) SetBandFolderEntries(folderID, bandID uint, songIDs []uint) error {
	return r.setFolderEntries(folderID, bandSubj(bandID), songIDs)
}

func (r *Repo) setFolderEntries(folderID uint, s subj, songIDs []uint) error {
	if _, err := r.folderForOwner(folderID, s); err != nil {
		return err
	}
	if len(songIDs) > 0 {
		ok, err := r.songsSelectableFor(s, songIDs)
		if err != nil {
			return err
		}
		if !ok {
			return fmt.Errorf("one or more songs not found: %w", gorm.ErrRecordNotFound)
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

// songsSelectableFor reports whether every song may be placed in this
// subject's folder: any visible song for a user, only the band's own songs for
// a band.
func (r *Repo) songsSelectableFor(s subj, songIDs []uint) (bool, error) {
	var n int64
	q := r.db.Model(&model.Song{})
	if s.userID != nil {
		q = q.Where(`id IN ? AND (owner_user_id = ? OR owner_band_id IN
			(SELECT band_id FROM band_members WHERE user_id = ?))`,
			songIDs, *s.userID, *s.userID)
	} else {
		q = q.Where("id IN ? AND owner_band_id = ?", songIDs, *s.bandID)
	}
	if err := q.Count(&n).Error; err != nil {
		return false, err
	}
	return n == int64(len(songIDs)), nil
}
```

(`userSubj`/`bandSubj` are the existing constructors in `subject.go`. The `fmt` and `gorm` imports are already present in `folders.go`.)

- [ ] **Step 4: Run `just test` + `just lint-go`.** Expected: green/clean. The personal folder tests in `folders_test.go` still pass — the public user signatures are unchanged and now delegate to the cores.

- [ ] **Step 5: Commit**

```bash
git add internal/repository/
git commit -m "feat: band folders and owner-keyed folder repository"
```

---

### Task 2: Personal folders accept visible band songs (cross-inclusion)

**Files:**
- Modify: `internal/repository/folders.go` (already widened in Task 1 via `songsSelectableFor`)
- Test: `internal/repository/folders_test.go` (append a case)

The repository change shipped in Task 1; this task locks it in with a dedicated test so the cross-inclusion contract can't silently regress.

- [ ] **Step 1: Write the failing-then-passing test** — append to `internal/repository/folders_test.go`:

```go
func TestPersonalFolderHoldsBandSong(t *testing.T) {
	repo := testRepo(t)
	alice := signupUser(t, repo, "alice")
	band := createBandForRepo(t, repo, alice, "The Quietones")
	bandSong, err := repo.CreateBandSong(band, "Wonderwall", "Oasis")
	if err != nil {
		t.Fatalf("create band song: %v", err)
	}

	// Alice (a member) can put the band song in her personal folder.
	folder, err := repo.CreateFolder(alice, "Faves")
	if err != nil {
		t.Fatalf("create folder: %v", err)
	}
	if err := repo.SetFolderEntries(folder.ID, alice, []uint{bandSong.ID}); err != nil {
		t.Fatalf("set entries with band song: %v", err)
	}
	folders, _ := repo.FoldersForUser(alice)
	if len(folders) != 1 || len(folders[0].SongIDs) != 1 || folders[0].SongIDs[0] != bandSong.ID {
		t.Fatalf("personal folder = %+v", folders)
	}

	// A non-member cannot: bob is not in the band.
	bob := signupUser(t, repo, "bob")
	bobFolder, _ := repo.CreateFolder(bob, "Bob's")
	err = repo.SetFolderEntries(bobFolder.ID, bob, []uint{bandSong.ID})
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		t.Fatalf("non-member entry = %v, want ErrRecordNotFound", err)
	}
}
```

(Add `"errors"` and `gorm.io/gorm` to the test imports if not already present.)

Run: `just test`. Expected: PASS (the behavior landed in Task 1). If it FAILS, the Task 1 `songsSelectableFor` user branch is wrong — fix it there.

- [ ] **Step 2: Commit**

```bash
git add internal/repository/folders_test.go
git commit -m "test: personal folders accept visible band songs"
```

---

### Task 3: Conversion engine preserves folder placement

**Files:**
- Modify: `internal/repository/bandsongs.go`
- Test: `internal/repository/conversion_test.go` (append a case)

- [ ] **Step 1: Write the failing test** — append to `internal/repository/conversion_test.go`:

```go
func TestConversionPreservesPersonalFolderPlacement(t *testing.T) {
	repo := testRepo(t)
	alice := signupUser(t, repo, "alice")
	band := createBandForRepo(t, repo, alice, "The Quietones")
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
	copy, err := repo.SongForUser(newSongID, alice)
	if err != nil {
		t.Fatalf("personal copy not found: %v", err)
	}
	if copy.OwnerUserID == nil || *copy.OwnerUserID != alice || copy.Title != "Wonderwall" {
		t.Fatalf("personal copy = %+v", copy)
	}
}
```

Run: `just test`. Expected: FAIL — Alice's only "work" is a folder entry, which `userTouchedSong` does not yet count, so no personal copy is made and her folder ends up empty.

- [ ] **Step 2: Count folder entries in `userTouchedSong`** in `internal/repository/bandsongs.go`. Replace the function with:

```go
// userTouchedSong reports whether the user has any personal work on the song:
// a metadata row (annotation, resource, practice) or a placement in one of
// their own folders.
func userTouchedSong(tx *gorm.DB, songID, userID uint) (bool, error) {
	for _, m := range []any{&model.SongAnnotation{}, &model.Resource{}, &model.PracticeEvent{}} {
		var n int64
		if err := tx.Model(m).
			Where("song_id = ? AND user_id = ?", songID, userID).
			Count(&n).Error; err != nil {
			return false, err
		}
		if n > 0 {
			return true, nil
		}
	}
	var entries int64
	err := tx.Model(&model.FolderEntry{}).
		Where(`song_id = ? AND folder_id IN
			(SELECT id FROM folders WHERE owner_user_id = ?)`, songID, userID).
		Count(&entries).Error
	if err != nil {
		return false, err
	}
	return entries > 0, nil
}
```

- [ ] **Step 3: Re-point folder entries in `convertBandSongForUser`.** In the same file, after the loop that re-points the three metadata tables (before `return nil`), add:

```go
	// Re-point the member's personal folder placements onto the copy. Band
	// folder entries (folders owned by the band) are left for deleteBandSongRows.
	err = tx.Model(&model.FolderEntry{}).
		Where(`song_id = ? AND folder_id IN
			(SELECT id FROM folders WHERE owner_user_id = ?)`, song.ID, userID).
		Update("song_id", personal.ID).Error
	if err != nil {
		return err
	}
```

(No change to `deleteBandSongRows`: by the time it runs, every member's personal folder entries are re-pointed away, so the remaining `song_id = ?` entries are band-folder entries and are correctly deleted with the song.)

- [ ] **Step 4: Run `just test` + `just lint-go`.** Expected: green/clean. The existing conversion tests still pass (a member with only metadata rows is unaffected; the folder query simply returns zero).

- [ ] **Step 5: Commit**

```bash
git add internal/repository/bandsongs.go internal/repository/conversion_test.go
git commit -m "feat: conversion engine preserves a member's folder placement"
```

---

### Task 4: Band folder handlers

**Files:**
- Create: `internal/handlers/bandfolders.go`
- Test: `internal/handlers/bandfolders_test.go` (new)

- [ ] **Step 1: Write the failing test** — `internal/handlers/bandfolders_test.go`:

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

// newBandFoldersAPI wires band CRUD, band song creation, and band folder routes.
func newBandFoldersAPI(t *testing.T) (*echo.Echo, *API) {
	t.Helper()
	e, api := newBandSongsAPI(t)
	g := e.Group("/api/bands/:id/folders", appmw.RequireAuth(api.Repo))
	g.GET("", api.BandFolders)
	g.POST("", api.CreateBandFolder)
	g.PUT("/order", api.ReorderBandFolders)
	g.PATCH("/:folderId", api.UpdateBandFolder)
	g.DELETE("/:folderId", api.DeleteBandFolder)
	g.PUT("/:folderId/entries", api.SetBandFolderEntries)
	return e, api
}

func TestBandFolderHandlers(t *testing.T) {
	e, _ := newBandFoldersAPI(t)
	alice := signupAndCookie(t, e, "alice") // creator/admin
	bob := signupAndCookie(t, e, "bob")
	band := createBandFor(t, e, alice, "The Quietones")
	addMemberAs(t, e, alice, band, "bob", "viewer")
	songID := createBandSongFor(t, e, alice, band, "Wonderwall")
	base := fmt.Sprintf("/api/bands/%d/folders", band)

	// Editor/Admin creates a folder.
	rec := jsonReq(e, http.MethodPost, base, `{"name":"Set 1"}`, alice)
	if rec.Code != http.StatusCreated {
		t.Fatalf("create folder: %d %s", rec.Code, rec.Body.String())
	}
	var folder struct {
		ID uint `json:"id"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &folder); err != nil {
		t.Fatalf("decode folder: %v", err)
	}

	// Put the band song in it.
	if rec := jsonReq(e, http.MethodPut, fmt.Sprintf("%s/%d/entries", base, folder.ID),
		fmt.Sprintf(`{"songIds":[%d]}`, songID), alice); rec.Code != http.StatusNoContent {
		t.Fatalf("set entries: %d %s", rec.Code, rec.Body.String())
	}

	// Viewer can read the folder list...
	rec = jsonReq(e, http.MethodGet, base, "", bob)
	if rec.Code != http.StatusOK {
		t.Fatalf("viewer list: %d %s", rec.Code, rec.Body.String())
	}
	var list []map[string]any
	_ = json.Unmarshal(rec.Body.Bytes(), &list)
	if len(list) != 1 {
		t.Fatalf("folder list = %s", rec.Body.String())
	}

	// ...but cannot create, rename, reorder, set entries, or delete.
	if rec := jsonReq(e, http.MethodPost, base, `{"name":"Nope"}`, bob); rec.Code != http.StatusForbidden {
		t.Fatalf("viewer create: %d, want 403", rec.Code)
	}
	if rec := jsonReq(e, http.MethodPatch, fmt.Sprintf("%s/%d", base, folder.ID), `{"name":"Nope"}`, bob); rec.Code != http.StatusForbidden {
		t.Fatalf("viewer rename: %d, want 403", rec.Code)
	}
	if rec := jsonReq(e, http.MethodPut, fmt.Sprintf("%s/%d/entries", base, folder.ID), `{"songIds":[]}`, bob); rec.Code != http.StatusForbidden {
		t.Fatalf("viewer set entries: %d, want 403", rec.Code)
	}
	if rec := jsonReq(e, http.MethodDelete, fmt.Sprintf("%s/%d", base, folder.ID), "", bob); rec.Code != http.StatusForbidden {
		t.Fatalf("viewer delete: %d, want 403", rec.Code)
	}

	// Editor renames then deletes.
	if rec := jsonReq(e, http.MethodPatch, fmt.Sprintf("%s/%d", base, folder.ID), `{"name":"Opener"}`, alice); rec.Code != http.StatusNoContent {
		t.Fatalf("rename: %d %s", rec.Code, rec.Body.String())
	}
	if rec := jsonReq(e, http.MethodDelete, fmt.Sprintf("%s/%d", base, folder.ID), "", alice); rec.Code != http.StatusNoContent {
		t.Fatalf("delete: %d %s", rec.Code, rec.Body.String())
	}
}
```

(`newBandSongsAPI`, `signupAndCookie`, `createBandFor`, `createBandSongFor`, `addMemberAs`, and `jsonReq` are existing handler-test helpers from Plan 4a/4b. Confirm `addMemberAs`'s exact name/signature in `internal/handlers/bands_test.go`; if members are added by a different helper, use that.)

Run: `just test`. Expected: FAIL — `api.BandFolders` undefined.

- [ ] **Step 2: Write `internal/handlers/bandfolders.go`**

```go
package handlers

import (
	"net/http"
	"strconv"
	"strings"

	"github.com/labstack/echo/v5"

	"github.com/jwhumphries/bandwidth/internal/model"
)

// bandFolderID parses the :folderId path parameter.
func bandFolderID(c *echo.Context) (uint, error) {
	id, err := strconv.ParseUint(c.Param("folderId"), 10, 32)
	if err != nil || id == 0 {
		return 0, echo.NewHTTPError(http.StatusNotFound, "folder not found")
	}
	return uint(id), nil
}

// BandFolders lists a band's folders with ordered song IDs (Viewer+).
func (a *API) BandFolders(c *echo.Context) error {
	bandID, _, err := a.bandAccess(c, model.RoleViewer)
	if err != nil {
		return err
	}
	folders, err := a.Repo.FoldersForBand(bandID)
	if err != nil {
		return err
	}
	return c.JSON(http.StatusOK, folders)
}

// CreateBandFolder adds a folder to the band (Editor+).
func (a *API) CreateBandFolder(c *echo.Context) error {
	bandID, _, err := a.bandAccess(c, model.RoleEditor)
	if err != nil {
		return err
	}
	var req folderNameRequest
	if err := c.Bind(&req); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid request body")
	}
	req.Name = strings.TrimSpace(req.Name)
	if req.Name == "" || len(req.Name) > maxTitleLen {
		return echo.NewHTTPError(http.StatusBadRequest, "a folder name is required")
	}
	folder, err := a.Repo.CreateBandFolder(bandID, req.Name)
	if err != nil {
		return err
	}
	return c.JSON(http.StatusCreated, folderResponse(folder))
}

// UpdateBandFolder renames a band folder (Editor+).
func (a *API) UpdateBandFolder(c *echo.Context) error {
	bandID, _, err := a.bandAccess(c, model.RoleEditor)
	if err != nil {
		return err
	}
	fid, err := bandFolderID(c)
	if err != nil {
		return err
	}
	var req folderNameRequest
	if err := c.Bind(&req); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid request body")
	}
	req.Name = strings.TrimSpace(req.Name)
	if req.Name == "" || len(req.Name) > maxTitleLen {
		return echo.NewHTTPError(http.StatusBadRequest, "a folder name is required")
	}
	if err := a.Repo.RenameBandFolder(fid, bandID, req.Name); err != nil {
		return notFoundOr(err, "folder")
	}
	return c.NoContent(http.StatusNoContent)
}

// DeleteBandFolder removes a band folder; band songs are untouched (Editor+).
func (a *API) DeleteBandFolder(c *echo.Context) error {
	bandID, _, err := a.bandAccess(c, model.RoleEditor)
	if err != nil {
		return err
	}
	fid, err := bandFolderID(c)
	if err != nil {
		return err
	}
	if err := a.Repo.DeleteBandFolder(fid, bandID); err != nil {
		return notFoundOr(err, "folder")
	}
	return c.NoContent(http.StatusNoContent)
}

// ReorderBandFolders applies a new folder order (Editor+).
func (a *API) ReorderBandFolders(c *echo.Context) error {
	bandID, _, err := a.bandAccess(c, model.RoleEditor)
	if err != nil {
		return err
	}
	var req reorderFoldersRequest
	if err := c.Bind(&req); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid request body")
	}
	if err := a.Repo.ReorderBandFolders(bandID, req.FolderIDs); err != nil {
		return badRequestOrErr(err, "one or more folders not found")
	}
	return c.NoContent(http.StatusNoContent)
}

// SetBandFolderEntries replaces a band folder's membership and order (Editor+).
func (a *API) SetBandFolderEntries(c *echo.Context) error {
	bandID, _, err := a.bandAccess(c, model.RoleEditor)
	if err != nil {
		return err
	}
	fid, err := bandFolderID(c)
	if err != nil {
		return err
	}
	var req folderEntriesRequest
	if err := c.Bind(&req); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid request body")
	}
	seen := map[uint]bool{}
	songIDs := make([]uint, 0, len(req.SongIDs))
	for _, sid := range req.SongIDs {
		if !seen[sid] {
			seen[sid] = true
			songIDs = append(songIDs, sid)
		}
	}
	if err := a.Repo.SetBandFolderEntries(fid, bandID, songIDs); err != nil {
		return badRequestOrErr(err, "folder or songs not found")
	}
	return c.NoContent(http.StatusNoContent)
}
```

`folderNameRequest`, `folderResponse`, `reorderFoldersRequest`, `folderEntriesRequest`, `maxTitleLen`, `notFoundOr`, and `bandAccess` already exist in the package. Add one small shared helper in `internal/handlers/folders.go` (the personal `ReorderFolders`/`SetFolderEntries` map `gorm.ErrRecordNotFound` to 400 inline today — extract it so both call sites share it). At the top of `folders.go`, after the imports, add:

```go
// badRequestOrErr maps a not-found to a 400 with msg (used where bad input
// names missing rows) and passes other errors through.
func badRequestOrErr(err error, msg string) error {
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return echo.NewHTTPError(http.StatusBadRequest, msg)
	}
	return err
}
```

Then in `folders.go` replace the two inline `if errors.Is(err, gorm.ErrRecordNotFound) { return echo.NewHTTPError(http.StatusBadRequest, ...) } return err` blocks in `ReorderFolders` and `SetFolderEntries` with `return badRequestOrErr(err, "...")` using their existing messages ("one or more folders not found" and "folder or songs not found"). This keeps behavior identical and shares the mapping.

- [ ] **Step 3: Run `just test` + `just lint-go`.** Expected: green/clean.

- [ ] **Step 4: Commit**

```bash
git add internal/handlers/
git commit -m "feat: band folder handlers (band view)"
```

---

### Task 5: Wire band folder routes + integration test

**Files:**
- Modify: `cmd/bandwidth/server.go`, `cmd/bandwidth/server_test.go`

- [ ] **Step 1: Failing integration test** — append to `cmd/bandwidth/server_test.go`:

```go
func TestBandFolderFlow(t *testing.T) {
	e := testServer(t)

	rec := do(e, http.MethodPost, "/api/auth/signup",
		`{"username":"alice","email":"alice@example.com","password":"hunter2hunter2"}`, nil)
	if rec.Code != http.StatusCreated {
		t.Fatalf("signup: %d %s", rec.Code, rec.Body.String())
	}
	alice := rec.Result().Cookies()

	rec = do(e, http.MethodPost, "/api/bands", `{"name":"The Quietones"}`, alice)
	if rec.Code != http.StatusCreated {
		t.Fatalf("create band: %d %s", rec.Code, rec.Body.String())
	}
	var band struct {
		ID uint `json:"id"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &band); err != nil {
		t.Fatalf("decode band: %v", err)
	}

	rec = do(e, http.MethodPost, fmt.Sprintf("/api/bands/%d/folders", band.ID),
		`{"name":"Set 1"}`, alice)
	if rec.Code != http.StatusCreated {
		t.Fatalf("create band folder: %d %s", rec.Code, rec.Body.String())
	}

	rec = do(e, http.MethodGet, fmt.Sprintf("/api/bands/%d/folders", band.ID), "", alice)
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), "Set 1") {
		t.Fatalf("band folders: %d %s", rec.Code, rec.Body.String())
	}
}
```

Run: `just test`. Expected: FAIL — band folder routes not wired (404/405 on POST `/api/bands/:id/folders`).

- [ ] **Step 2: Wire the routes** in `cmd/bandwidth/server.go` `newEcho`, inside the existing `bands` group (after the band song routes), add:

```go
	bands.GET("/:id/folders", api.BandFolders)
	bands.POST("/:id/folders", api.CreateBandFolder)
	bands.PUT("/:id/folders/order", api.ReorderBandFolders)
	bands.PATCH("/:id/folders/:folderId", api.UpdateBandFolder)
	bands.DELETE("/:id/folders/:folderId", api.DeleteBandFolder)
	bands.PUT("/:id/folders/:folderId/entries", api.SetBandFolderEntries)
```

- [ ] **Step 3: `just test` green, then `just check` → "all checks passed". Commit:**

```bash
git add cmd/
git commit -m "feat: wire band folder routes"
```

---

### Task 6: Frontend band folder hooks

**Files:**
- Create: `frontend/src/hooks/bandfolders.ts`

The `Folder` type (`{id, name, position, songIds}`) is reused as-is — confirm it exists in `frontend/src/lib/types.ts` (it does from Plan 3) and add nothing.

- [ ] **Step 1: Write `frontend/src/hooks/bandfolders.ts`** — mirror `frontend/src/hooks/folders.ts` but keyed by `bandId`, hitting `/api/bands/${bandId}/folders`:

```ts
import {useMutation, useQuery, useQueryClient} from '@tanstack/react-query';
import {api, ApiError} from '../lib/api';
import type {Folder} from '../lib/types';

export function useBandFolders(bandId: number) {
  return useQuery<Folder[], ApiError>({
    queryKey: ['bands', bandId, 'folders'],
    queryFn: () => api.get<Folder[]>(`/api/bands/${bandId}/folders`),
  });
}

export function useCreateBandFolder(bandId: number) {
  const queryClient = useQueryClient();
  return useMutation<Folder, ApiError, {name: string}>({
    mutationFn: data => api.post<Folder>(`/api/bands/${bandId}/folders`, data),
    onSuccess: () =>
      void queryClient.invalidateQueries({queryKey: ['bands', bandId, 'folders']}),
  });
}

export function useRenameBandFolder(bandId: number) {
  const queryClient = useQueryClient();
  return useMutation<void, ApiError, {id: number; name: string}>({
    mutationFn: ({id, name}) =>
      api.patch<void>(`/api/bands/${bandId}/folders/${id}`, {name}),
    onSuccess: () =>
      void queryClient.invalidateQueries({queryKey: ['bands', bandId, 'folders']}),
  });
}

export function useDeleteBandFolder(bandId: number) {
  const queryClient = useQueryClient();
  return useMutation<void, ApiError, number>({
    mutationFn: id => api.delete(`/api/bands/${bandId}/folders/${id}`),
    onSuccess: () =>
      void queryClient.invalidateQueries({queryKey: ['bands', bandId, 'folders']}),
  });
}

export function useReorderBandFolders(bandId: number) {
  const queryClient = useQueryClient();
  return useMutation<void, ApiError, number[]>({
    mutationFn: folderIds =>
      api.put<void>(`/api/bands/${bandId}/folders/order`, {folderIds}),
    onMutate: folderIds => {
      queryClient.setQueryData<Folder[] | undefined>(
        ['bands', bandId, 'folders'],
        folders => {
          if (!folders) return folders;
          const byID = new Map(folders.map(f => [f.id, f]));
          return folderIds
            .map(id => byID.get(id))
            .filter((f): f is Folder => f !== undefined);
        },
      );
    },
    onError: () =>
      void queryClient.invalidateQueries({queryKey: ['bands', bandId, 'folders']}),
  });
}

export function useSetBandFolderEntries(bandId: number) {
  const queryClient = useQueryClient();
  return useMutation<void, ApiError, {id: number; songIds: number[]}>({
    mutationFn: ({id, songIds}) =>
      api.put<void>(`/api/bands/${bandId}/folders/${id}/entries`, {songIds}),
    onMutate: ({id, songIds}) => {
      queryClient.setQueryData<Folder[] | undefined>(
        ['bands', bandId, 'folders'],
        folders => folders?.map(f => (f.id === id ? {...f, songIds} : f)),
      );
    },
    onError: () =>
      void queryClient.invalidateQueries({queryKey: ['bands', bandId, 'folders']}),
    onSettled: () =>
      void queryClient.invalidateQueries({queryKey: ['bands', bandId, 'folders']}),
  });
}
```

- [ ] **Step 2: Checks** — `just typecheck && just lint-js && just format-check` (run `just format` first if format-check fails). `just test-frontend` stays green. Commit:

```bash
git add frontend
git commit -m "feat: band folder hooks"
```

---

### Task 7: Band folder sidebar + folder-filtered band song list

**Files:**
- Create: `frontend/src/components/bands/BandFolderSidebar.tsx`
- Modify: `frontend/src/components/bands/BandSongList.tsx` (accept a `folderId` filter)
- Modify: `frontend/src/pages/BandPage.tsx` (mount the sidebar, hold selected folder)
- Test: `frontend/src/components/bands/BandFolderSidebar.test.tsx` (new)

- [ ] **Step 1: Failing test** — `frontend/src/components/bands/BandFolderSidebar.test.tsx`:

```tsx
import {screen, waitFor} from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import {beforeEach, describe, expect, it, vi} from 'vitest';
import {renderWithProviders} from '../../test/utils';
import BandFolderSidebar from './BandFolderSidebar';

const folders = [{id: 1, name: 'Set 1', position: 1, songIds: []}];

function jsonResponse(status: number, body: unknown) {
  return new Response(JSON.stringify(body), {status});
}

describe('BandFolderSidebar', () => {
  beforeEach(() => {
    vi.stubGlobal(
      'fetch',
      vi.fn().mockImplementation((_input, init?: RequestInit) => {
        if (init?.method === 'POST') return Promise.resolve(jsonResponse(201, {id: 2}));
        return Promise.resolve(jsonResponse(200, folders));
      }),
    );
  });

  it('lets editors create a folder', async () => {
    renderWithProviders(
      <BandFolderSidebar
        bandId={3}
        canEdit={true}
        selectedId={null}
        onSelect={() => {}}
      />,
    );
    await screen.findByText('Set 1');
    await userEvent.type(screen.getByPlaceholderText(/new folder/i), 'Encore');
    await userEvent.click(screen.getByRole('button', {name: /^create$/i}));
    await waitFor(() => {
      const calls = vi.mocked(fetch).mock.calls;
      expect(
        calls.some(
          ([input, init]) =>
            String(input).endsWith('/api/bands/3/folders') &&
            init?.method === 'POST',
        ),
      ).toBe(true);
    });
  });

  it('hides editing controls for viewers', async () => {
    renderWithProviders(
      <BandFolderSidebar
        bandId={3}
        canEdit={false}
        selectedId={null}
        onSelect={() => {}}
      />,
    );
    await screen.findByText('Set 1');
    expect(screen.queryByPlaceholderText(/new folder/i)).not.toBeInTheDocument();
    expect(screen.queryByRole('button', {name: /delete set 1/i})).not.toBeInTheDocument();
  });
});
```

Run: `just test-frontend`. Expected: FAIL — cannot resolve `./BandFolderSidebar`.

- [ ] **Step 2: Write `frontend/src/components/bands/BandFolderSidebar.tsx`.** This mirrors `frontend/src/components/folders/FolderSidebar.tsx` (read it for the exact dnd-kit + sortable row structure) but is keyed by `bandId`, gates all editing behind `canEdit`, and uses the band folder hooks. Viewers see the "All songs" entry and the folder names (selectable) with no reorder handle, no rename/delete buttons, and no create form.

```tsx
import {DndContext, closestCenter} from '@dnd-kit/core';
import type {DragEndEvent} from '@dnd-kit/core';
import {
  SortableContext,
  useSortable,
  verticalListSortingStrategy,
} from '@dnd-kit/sortable';
import {CSS} from '@dnd-kit/utilities';
import {useState, useRef} from 'react';
import type {FormEvent} from 'react';
import ConfirmModal from '../songs/ConfirmModal';
import {
  useBandFolders,
  useCreateBandFolder,
  useDeleteBandFolder,
  useRenameBandFolder,
  useReorderBandFolders,
} from '../../hooks/bandfolders';
import type {Folder} from '../../lib/types';

function BandFolderRow({
  folder,
  canEdit,
  selected,
  onSelect,
  onRename,
  onDelete,
}: {
  folder: Folder;
  canEdit: boolean;
  selected: boolean;
  onSelect: () => void;
  onRename: (name: string) => void;
  onDelete: () => void;
}) {
  const {attributes, listeners, setNodeRef, transform, transition} =
    useSortable({id: folder.id, disabled: !canEdit});
  const [editing, setEditing] = useState(false);
  const [name, setName] = useState(folder.name);
  const submitted = useRef(false);

  const submitRename = (e: FormEvent) => {
    e.preventDefault();
    if (submitted.current) return;
    submitted.current = true;
    if (name.trim()) onRename(name.trim());
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
      {canEdit && (
        <button
          className="cursor-grab touch-none"
          aria-label={`Reorder ${folder.name}`}
          {...attributes}
          {...listeners}
        >
          ⠿
        </button>
      )}
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
      {canEdit && (
        <>
          <button
            className="btn btn-ghost btn-xs"
            aria-label={`Rename ${folder.name}`}
            onClick={() => {
              submitted.current = false;
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
        </>
      )}
    </li>
  );
}

export default function BandFolderSidebar({
  bandId,
  canEdit,
  selectedId,
  onSelect,
}: {
  bandId: number;
  canEdit: boolean;
  selectedId: number | null;
  onSelect: (id: number | null) => void;
}) {
  const {data: folders = []} = useBandFolders(bandId);
  const createFolder = useCreateBandFolder(bandId);
  const renameFolder = useRenameBandFolder(bandId);
  const deleteFolder = useDeleteBandFolder(bandId);
  const reorderFolders = useReorderBandFolders(bandId);
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
              <BandFolderRow
                key={f.id}
                folder={f}
                canEdit={canEdit}
                selected={selectedId === f.id}
                onSelect={() => onSelect(f.id)}
                onRename={name => renameFolder.mutate({id: f.id, name})}
                onDelete={() => setDeleting(f)}
              />
            ))}
          </ul>
        </SortableContext>
      </DndContext>
      {canEdit && (
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
      )}
      <ConfirmModal
        open={deleting !== null}
        title="Delete folder"
        message={`Delete "${deleting?.name ?? ''}"? Songs in it are not deleted.`}
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

- [ ] **Step 3: Add an optional folder filter to `BandSongList`.** In `frontend/src/components/bands/BandSongList.tsx`, accept an optional `folderId` and, when set, filter the list to that band folder's `songIds` (preserving the folder's order). Read the current file; add `folderId` to the props and, after `const {data: songs = []} = useBandSongs(bandId);`, derive the displayed list:

```tsx
import {useBandFolders} from '../../hooks/bandfolders';
// ...
export default function BandSongList({
  bandId,
  canEdit,
  folderId = null,
}: {
  bandId: number;
  canEdit: boolean;
  folderId?: number | null;
}) {
  const {data: songs = []} = useBandSongs(bandId);
  const {data: folders = []} = useBandFolders(bandId);
  const [adding, setAdding] = useState(false);

  const folder = folderId === null ? null : folders.find(f => f.id === folderId);
  const visible =
    folder === null || folder === undefined
      ? songs
      : folder.songIds
          .map(id => songs.find(s => s.id === id))
          .filter((s): s is (typeof songs)[number] => s !== undefined);
```

Then render `visible` instead of `songs` in the `.map(...)` and in the `length === 0` check. Keep the rest of the component unchanged.

- [ ] **Step 4: Mount the sidebar in `frontend/src/pages/BandPage.tsx`.** Read the file; hold the selected folder in state and lay the sidebar beside the song list. Add imports and replace the `<BandSongList .../>` line with a flex row:

```tsx
import {useState} from 'react';
import BandFolderSidebar from '../components/bands/BandFolderSidebar';
// ...
  const [folderId, setFolderId] = useState<number | null>(null);
  const canEdit = band.myRole !== 'viewer';
// ...
      <div className="flex flex-col gap-4 sm:flex-row">
        <BandFolderSidebar
          bandId={band.id}
          canEdit={canEdit}
          selectedId={folderId}
          onSelect={setFolderId}
        />
        <div className="flex-1">
          <BandSongList bandId={band.id} canEdit={canEdit} folderId={folderId} />
        </div>
      </div>
```

(Match the real variable names already in `BandPage.tsx` — it derives `band` and likely an `isAdmin` flag. Keep the existing `MemberList`, `InviteManager`, and `BandSettings` sections below this row. If `BandPage` already computes `canEdit`/`band.myRole !== 'viewer'` inline for `BandSongList`, reuse that single expression.)

- [ ] **Step 5: All four frontend checks green** (`just test-frontend`, `just typecheck`, `just lint-js`, `just format-check`; run `just format` if needed). Watch for collateral breakage in `BandPage.test.tsx` — mounting `BandFolderSidebar` adds a `GET /api/bands/:id/folders` fetch; if the existing BandPage test's fetch mock doesn't handle `/folders`, add a minimal guard returning `[]` (mirroring the `/songs` guard added in Plan 4b). Commit:

```bash
git add frontend
git commit -m "feat: band folder sidebar and folder-filtered band song list"
```

---

### Task 8: Personal-view cross-inclusion UI

**Files:**
- Modify: `frontend/src/pages/SongPage.tsx` (show the personal folder picker for band songs)
- Test: `frontend/src/pages/SongPage.test.tsx` (extend the band-song case)

The personal `FolderPicker` already lists the member's folders and toggles membership via the personal `useSetFolderEntries` hook, which now accepts band songs (Task 1/2). The only question is whether `SongPage` renders the folder picker for a band song. In Plan 4b the danger-zone delete was hidden for band songs, but the folder picker (the member's personal organization) should remain available.

- [ ] **Step 1: Failing test** — extend the existing `describe('SongPage band song', ...)` block in `frontend/src/pages/SongPage.test.tsx` with an assertion that the personal folder picker is present for a band song. Read the block (added in Plan 4b) and the stub; ensure the stub returns a folders list for `GET /api/folders` (add a URL branch returning `[{id: 1, name: 'Faves', position: 1, songIds: []}]` if the stub is generic), then add to the existing test:

```tsx
    // The member can still file a band song into their personal folders.
    expect(screen.getByText('Faves')).toBeInTheDocument();
    expect(screen.getByRole('checkbox', {name: 'Faves'})).not.toBeChecked();
```

Run: `just test-frontend`. Expected: FAIL if `SongPage` does not render `FolderPicker` for band songs.

- [ ] **Step 2: Ensure `SongPage` renders the personal folder picker for band songs.** Read `frontend/src/pages/SongPage.tsx`. The personal `FolderPicker` (`frontend/src/components/folders/FolderPicker.tsx`) is mounted in a "Folders" card for personal songs. Confirm that card is NOT inside the `{!isBandSong && (...)}` wrapper introduced in Plan 4b (only the danger-zone delete should be). If the folders card was accidentally hidden for band songs, move it outside that wrapper so it renders for all songs. The `FolderPicker` needs no change — it already uses the personal folders + personal `useSetFolderEntries`, which now accepts the (visible) band song.

- [ ] **Step 3: All four frontend checks green.** Commit:

```bash
git add frontend
git commit -m "feat: file band songs into personal folders from the song page"
```

---

### Task 9: Docs + final verification

**Files:**
- Modify: `AGENTS.md`, `README.md`

- [ ] **Step 1: Update `AGENTS.md`** (verify each claim against the code):

1. In the architecture section, note `internal/handlers/bandfolders.go` (band folder handlers) and that `internal/repository/folders.go` is owner-keyed via the `subj.ownerScope()` helper in `subject.go`.
2. Append to the Domain model section:

```markdown
Bands have playlist-style folders too: band-owned (`owner_band_id`) folders
that hold the band's songs, managed only from the band view by Editors/Admins
and read by Viewers. Folder methods are written once against the `subj`
value's `ownerScope()` and exposed as user- and band-keyed methods. A member's
personal folders may also hold any song visible to them, including band songs
surfaced by interleaving. When a member loses access to a band song, their
personal folder placements are re-pointed onto the personal copy alongside
their other personal rows (a personal folder entry alone now counts as
personal work worth preserving); band folder entries are removed with the
band song.
```

- [ ] **Step 2: Update `README.md`** — extend the bands clause to mention band folders and personal-folder cross-inclusion, e.g.: "Bands with roles, invites, shared songs, and band folders (with personal-folder cross-inclusion and a conversion engine that preserves members' work and folder placements on leave/delete) are implemented. Planned per the design doc: installable PWA, single container on fly.io." Adapt to the real surrounding sentence and report any change.

- [ ] **Step 3: `just check` → "all checks passed"; only the two doc files dirty. Commit:**

```bash
git add AGENTS.md README.md
git commit -m "docs: document band folders and the widened conversion rule"
```

---

## Done criteria

- `just check` green (all six gates).
- In a band, an Editor creates folders, drags band songs into them, reorders and renames them; a Viewer sees the folders and the folder-filtered song list but no editing controls; non-members get 404 on every band folder route.
- A member can file a band song (shown in their personal library via interleaving) into their personal folders from the song page.
- A member who has only foldered a band song still gets a personal copy when they leave / are removed / the band song or band is deleted, and that copy keeps its place in their personal folder; band folder entries vanish with the band song.
- All band folder writes are Editor+ and reads are Viewer; band folders never leak across bands, and personal folders never accept songs not visible to the member.
- Next: Plan 5 (installable PWA + single-container deploy) gets written against this codebase.
