# BandWidth Band Songs & Interleaving Implementation Plan (Plan 4b of 7)

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Band songs with a full band metadata layer (status, notes, resources, rehearsals), personal-view interleaving (band songs appear in each member's library with their own editable personal layer plus a read-only band section), and the conversion engine that preserves a member's personal work when they leave a band, are removed, or the band/song is deleted.

**Architecture:** The schema is already subject-keyed (every metadata row carries nullable `user_id`/`band_id`). This plan introduces a private `subj` value (a user XOR a band) and refactors the Plan-3 metadata cores to be subject-parameterized — the existing public user-keyed methods keep their exact signatures (so no merged call sites or tests change) and delegate to the shared cores; new band-keyed public methods are thin additions. Band songs are `owner_band_id`-owned; they surface in `SongsForUser` (member's personal layer) and `SongsForBand` (band layer). The conversion engine re-points a member's personal rows onto a freshly-created personal-copy song. Band folders and personal-folders-accepting-band-songs are Plan 4c.

**Tech Stack:** Existing Go/Echo/GORM backend and React/TanStack Query frontend. No new dependencies.

---

## Conventions for the executor

- Repo root `/Users/john/code/git/BandWidth`, branch off `main` (e.g. `band-songs`).
- All verification through `just` recipes (Dagger, Bash timeout 600000 ms): `just test`, `just lint-go`, `just test-frontend`, `just typecheck`, `just lint-js`, `just format-check` (`just format` to fix), full gate `just check` (Tasks 9 and 14). Host commands only for dependency management and `go doc`.
- Echo v5 pointer contexts. Existing helpers reused: handlers `songID` (parses `:id`), `notFoundOr`, `bandAccess` (parses `:id` as a band, enforces a role floor, 404 non-member / 403 under-role — from Plan 4a); repository `testRepo`, `CreateUser`, `CreateBand`, `AddMember`, `MemberRole`, `MembersForBand`; handler test helpers `newTestAPI`, `signupAndCookie`, `jsonReq`, `createBandFor`, `mustUserID`. Handlers nil-guard `appmw.CurrentUser(c)`.
- Spec semantics fixed for this plan: band songs carry a full band layer (status/notes/resources + a rehearsal log = band-keyed practice events), editable only from the band view by Editors/Admins; Viewers read only. In the personal view a member has their OWN editable layer on a band song (their personal status/notes/resources/practice) AND sees the band layer read-only. Identity (title/artist) of a band song is band-owned — a member cannot edit it or delete the song from their personal view. The conversion rule: when a band song stops being available to a member (they leave/are removed, the band deletes the song, or the band is deleted), if the member has ANY personal rows on it (annotation, resource, or practice) those rows are re-pointed onto a new personal-copy song owned by them (band layer is not copied); untouched band songs simply vanish from their library. All conversion runs inside the same transaction as the removal.
- Band-layer JSON uses `lastRehearsedAt`/`rehearsalCount`; the personal layer keeps `lastPracticedAt`/`practiceCount`.

## File structure being built

```
internal/repository/subject.go        # subj value + scope() helper (new)
internal/repository/practice.go       # extract subject cores; add band practice methods
internal/repository/resources.go      # extract subject cores; add band resource methods
internal/repository/songs.go          # extract annotation core; add band-song ownership,
                                      # SongsForBand, SongForBand, SongVisibleToUser,
                                      # band annotation; expand SongsForUser; add bandId/bandName
internal/repository/bandsongs.go      # conversion engine + DeleteBandSong (new)
internal/repository/bands.go          # RemoveMember/DeleteBand call the conversion engine
internal/handlers/bandsongs.go        # band-view song CRUD + bandSongDetailResponse (new)
internal/handlers/bandpractice.go     # band-view practice + resource handlers (new)
internal/handlers/songs.go            # personal-view interleaving (band object, visibility,
                                      # block identity-edit/delete on band songs)
cmd/bandwidth/server.go               # band-song routes
frontend/src/lib/types.ts             # BandLayer, extend SongDetail/SongListItem
frontend/src/hooks/bandsongs.ts       # band-song + band-practice + band-resource hooks (new)
frontend/src/hooks/songs.ts           # SongDetail gains band layer (type only)
frontend/src/components/bands/BandSongList.tsx + AddBandSongModal.tsx (new)
frontend/src/pages/BandSongPage.tsx   # band-view song detail (new)
frontend/src/pages/BandPage.tsx       # mount the band song list
frontend/src/pages/SongPage.tsx       # read-only band section for band songs
frontend/src/components/songs/SongRow.tsx + StatusBadge surfaces band tag
frontend/src/App.tsx                  # /bands/:id/songs/:songId route
```

---

### Task 1: Subject value + band practice methods

**Files:**
- Create: `internal/repository/subject.go`
- Modify: `internal/repository/practice.go`
- Test: `internal/repository/practice_test.go` (append band cases)

- [ ] **Step 1: Write the failing tests** — append to `internal/repository/practice_test.go`:

```go
func TestBandPracticeIsolatedFromUser(t *testing.T) {
	repo := testRepo(t)
	user, _ := repo.CreateUser("alice", "alice@example.com", "h")
	band, _ := repo.CreateBand(user.ID, "Band")
	song, _ := repo.CreateBandSong(band.ID, "Wonderwall", "Oasis")

	// A band rehearsal and a personal practice on the SAME song/day are
	// distinct rows in distinct layers.
	if err := repo.LogBandPractice(song.ID, band.ID, "2026-06-10"); err != nil {
		t.Fatalf("LogBandPractice: %v", err)
	}
	if err := repo.LogPractice(song.ID, user.ID, "2026-06-10"); err != nil {
		t.Fatalf("LogPractice: %v", err)
	}
	// Logging the same band day twice is idempotent.
	if err := repo.LogBandPractice(song.ID, band.ID, "2026-06-10"); err != nil {
		t.Fatal(err)
	}

	bandLast, bandCount, _ := repo.BandPracticeStats(song.ID, band.ID)
	if bandLast != "2026-06-10" || bandCount != 1 {
		t.Errorf("band stats = %q/%d, want 2026-06-10/1", bandLast, bandCount)
	}
	userLast, userCount, _ := repo.PracticeStats(song.ID, user.ID)
	if userLast != "2026-06-10" || userCount != 1 {
		t.Errorf("user stats = %q/%d (band rows must not leak into user layer)", userLast, userCount)
	}

	if err := repo.DeleteBandPractice(song.ID, band.ID, "2026-06-10"); err != nil {
		t.Fatalf("DeleteBandPractice: %v", err)
	}
	bandLast, bandCount, _ = repo.BandPracticeStats(song.ID, band.ID)
	if bandCount != 0 {
		t.Errorf("band count after delete = %d", bandCount)
	}
	// The user's row is untouched by deleting the band's.
	_, userCount, _ = repo.PracticeStats(song.ID, user.ID)
	if userCount != 1 {
		t.Errorf("user count after band delete = %d, want 1", userCount)
	}
}
```

(This references `CreateBandSong` from Task 3. To keep Task 1 self-contained, create the band song inline via the model instead: replace the `repo.CreateBandSong(...)` line with:

```go
	song := &model.Song{Title: "Wonderwall", Artist: "Oasis", OwnerBandID: &band.ID}
	if err := repo.db.Create(song).Error; err != nil {
		t.Fatal(err)
	}
```

and add `"github.com/jwhumphries/bandwidth/internal/model"` to the test imports if not present. Use this inline form; do NOT depend on Task 3.)

Run: `just test`. Expected: FAIL — undefined: LogBandPractice (etc.).

- [ ] **Step 2: Write `internal/repository/subject.go`**

```go
package repository

// subj identifies the owner of a metadata row: a user XOR a band. Exactly
// one of the two ids is set. The schema keys annotations, resources, and
// practice events by subject, so the metadata operations are written once
// against subj and exposed as user- and band-specific public methods.
type subj struct {
	userID *uint
	bandID *uint
}

func userSubj(id uint) subj { return subj{userID: &id} }
func bandSubj(id uint) subj { return subj{bandID: &id} }

// scope returns a column filter selecting only this subject's rows (the
// other subject column is required to be NULL so the two layers never mix)
// together with its bind value.
func (s subj) scope() (string, uint) {
	if s.userID != nil {
		return "user_id = ? AND band_id IS NULL", *s.userID
	}
	return "band_id = ? AND user_id IS NULL", *s.bandID
}
```

- [ ] **Step 3: Refactor `internal/repository/practice.go`** to subject cores plus user and band wrappers. The full file becomes:

```go
package repository

import (
	"gorm.io/gorm/clause"

	"github.com/jwhumphries/bandwidth/internal/model"
)

// logPractice records a practiced day for a subject. The day is deduped by
// the per-subject partial unique index, so repeats are no-ops.
func (r *Repo) logPractice(songID uint, s subj, date string) error {
	event := &model.PracticeEvent{SongID: songID, UserID: s.userID, BandID: s.bandID, Date: date}
	return r.db.Clauses(clause.OnConflict{DoNothing: true}).Create(event).Error
}

func (r *Repo) deletePractice(songID uint, s subj, date string) error {
	cond, id := s.scope()
	return r.db.
		Where("song_id = ? AND "+cond+" AND date = ?", songID, id, date).
		Delete(&model.PracticeEvent{}).Error
}

func (r *Repo) practiceStats(songID uint, s subj) (string, int, error) {
	cond, id := s.scope()
	var row struct {
		Last  string
		Count int
	}
	err := r.db.Model(&model.PracticeEvent{}).
		Select("COALESCE(MAX(date), '') AS last, COUNT(*) AS count").
		Where("song_id = ? AND "+cond, songID, id).
		Scan(&row).Error
	if err != nil {
		return "", 0, err
	}
	return row.Last, row.Count, nil
}

// LogPractice records that the user practiced the song on date (YYYY-MM-DD).
func (r *Repo) LogPractice(songID, userID uint, date string) error {
	return r.logPractice(songID, userSubj(userID), date)
}

// DeletePractice removes the user's practice event for one date (no-op when absent).
func (r *Repo) DeletePractice(songID, userID uint, date string) error {
	return r.deletePractice(songID, userSubj(userID), date)
}

// PracticeStats returns the user's last practiced date ("" when never) and count.
func (r *Repo) PracticeStats(songID, userID uint) (string, int, error) {
	return r.practiceStats(songID, userSubj(userID))
}

// LogBandPractice records a band rehearsal on date (idempotent per day).
func (r *Repo) LogBandPractice(songID, bandID uint, date string) error {
	return r.logPractice(songID, bandSubj(bandID), date)
}

// DeleteBandPractice removes the band's rehearsal for one date (no-op when absent).
func (r *Repo) DeleteBandPractice(songID, bandID uint, date string) error {
	return r.deletePractice(songID, bandSubj(bandID), date)
}

// BandPracticeStats returns the band's last rehearsal date and count.
func (r *Repo) BandPracticeStats(songID, bandID uint) (string, int, error) {
	return r.practiceStats(songID, bandSubj(bandID))
}
```

- [ ] **Step 4: Run `just test` + `just lint-go`.** Expected: all green/clean — the existing user-layer practice tests still pass (user rows always have `band_id` NULL, so the added `AND band_id IS NULL` is a no-op for them), plus the new band test.

- [ ] **Step 5: Commit**

```bash
git add internal/repository/
git commit -m "feat: subject value and band practice methods"
```

---

### Task 2: Band resource methods

**Files:**
- Modify: `internal/repository/resources.go`
- Test: `internal/repository/resources_test.go` (append band cases)

- [ ] **Step 1: Write the failing test** — append to `internal/repository/resources_test.go`:

```go
func TestBandResourcesIsolatedFromUser(t *testing.T) {
	repo := testRepo(t)
	user, _ := repo.CreateUser("alice", "alice@example.com", "h")
	band, _ := repo.CreateBand(user.ID, "Band")
	song := &model.Song{Title: "Wonderwall", Artist: "Oasis", OwnerBandID: &band.ID}
	if err := repo.db.Create(song).Error; err != nil {
		t.Fatal(err)
	}

	bandRes, err := repo.CreateBandResource(song.ID, band.ID, "https://example.com/band", "band tab")
	if err != nil {
		t.Fatalf("CreateBandResource: %v", err)
	}
	if _, err := repo.CreateResource(song.ID, user.ID, "https://example.com/mine", "my tab"); err != nil {
		t.Fatal(err)
	}

	// Each layer sees only its own resources.
	bandList, _ := repo.ResourcesForSongBand(song.ID, band.ID)
	if len(bandList) != 1 || bandList[0].Label != "band tab" {
		t.Errorf("band resources = %+v", bandList)
	}
	userList, _ := repo.ResourcesForSongUser(song.ID, user.ID)
	if len(userList) != 1 || userList[0].Label != "my tab" {
		t.Errorf("user resources = %+v", userList)
	}

	url := "https://example.com/band2"
	if _, err := repo.UpdateBandResource(bandRes.ID, song.ID, band.ID, &url, nil); err != nil {
		t.Fatalf("UpdateBandResource: %v", err)
	}
	if err := repo.DeleteBandResource(bandRes.ID, song.ID, band.ID); err != nil {
		t.Fatalf("DeleteBandResource: %v", err)
	}
	if bandList, _ = repo.ResourcesForSongBand(song.ID, band.ID); len(bandList) != 0 {
		t.Errorf("band resources after delete = %d", len(bandList))
	}
	if userList, _ = repo.ResourcesForSongUser(song.ID, user.ID); len(userList) != 1 {
		t.Errorf("user resources after band delete = %d, want 1", len(userList))
	}
}
```

Run: `just test`. Expected: FAIL — undefined: CreateBandResource (etc.).

- [ ] **Step 2: Refactor `internal/repository/resources.go`** to subject cores plus user and band wrappers. The full file becomes:

```go
package repository

import (
	"github.com/jwhumphries/bandwidth/internal/model"
)

func (r *Repo) resourcesForSong(songID uint, s subj) ([]model.Resource, error) {
	resources := []model.Resource{}
	cond, id := s.scope()
	err := r.db.Where("song_id = ? AND "+cond, songID, id).
		Order("position, id").Find(&resources).Error
	if err != nil {
		return nil, err
	}
	return resources, nil
}

func (r *Repo) createResource(songID uint, s subj, url, label string) (*model.Resource, error) {
	cond, id := s.scope()
	var maxPos int
	err := r.db.Model(&model.Resource{}).
		Select("COALESCE(MAX(position), 0)").
		Where("song_id = ? AND "+cond, songID, id).
		Scan(&maxPos).Error
	if err != nil {
		return nil, err
	}
	res := &model.Resource{
		SongID: songID, UserID: s.userID, BandID: s.bandID,
		URL: url, Label: label, Position: maxPos + 1,
	}
	if err := r.db.Create(res).Error; err != nil {
		return nil, err
	}
	return res, nil
}

func (r *Repo) resourceForSubject(resourceID, songID uint, s subj) (*model.Resource, error) {
	var res model.Resource
	cond, id := s.scope()
	err := r.db.Where("id = ? AND song_id = ? AND "+cond, resourceID, songID, id).
		First(&res).Error
	if err != nil {
		return nil, err
	}
	return &res, nil
}

func (r *Repo) updateResource(resourceID, songID uint, s subj, url, label *string) (*model.Resource, error) {
	res, err := r.resourceForSubject(resourceID, songID, s)
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

func (r *Repo) deleteResource(resourceID, songID uint, s subj) error {
	res, err := r.resourceForSubject(resourceID, songID, s)
	if err != nil {
		return err
	}
	return r.db.Delete(res).Error
}

// ResourcesForSongUser returns the user's resources for a song, in position order.
func (r *Repo) ResourcesForSongUser(songID, userID uint) ([]model.Resource, error) {
	return r.resourcesForSong(songID, userSubj(userID))
}

// CreateResource appends a resource to the user's list for a song.
func (r *Repo) CreateResource(songID, userID uint, url, label string) (*model.Resource, error) {
	return r.createResource(songID, userSubj(userID), url, label)
}

// UpdateResource applies any provided fields to the user's resource.
func (r *Repo) UpdateResource(resourceID, songID, userID uint, url, label *string) (*model.Resource, error) {
	return r.updateResource(resourceID, songID, userSubj(userID), url, label)
}

// DeleteResource removes the user's resource.
func (r *Repo) DeleteResource(resourceID, songID, userID uint) error {
	return r.deleteResource(resourceID, songID, userSubj(userID))
}

// ResourcesForSongBand returns the band's resources for a song, in position order.
func (r *Repo) ResourcesForSongBand(songID, bandID uint) ([]model.Resource, error) {
	return r.resourcesForSong(songID, bandSubj(bandID))
}

// CreateBandResource appends a resource to the band's list for a song.
func (r *Repo) CreateBandResource(songID, bandID uint, url, label string) (*model.Resource, error) {
	return r.createResource(songID, bandSubj(bandID), url, label)
}

// UpdateBandResource applies any provided fields to the band's resource.
func (r *Repo) UpdateBandResource(resourceID, songID, bandID uint, url, label *string) (*model.Resource, error) {
	return r.updateResource(resourceID, songID, bandSubj(bandID), url, label)
}

// DeleteBandResource removes the band's resource.
func (r *Repo) DeleteBandResource(resourceID, songID, bandID uint) error {
	return r.deleteResource(resourceID, songID, bandSubj(bandID))
}
```

- [ ] **Step 3: Run `just test` + `just lint-go`.** Expected: green/clean (existing resource tests unchanged behavior + the new band test).

- [ ] **Step 4: Commit**

```bash
git add internal/repository/
git commit -m "feat: band resource methods"
```

---

### Task 3: Band song ownership, listing, visibility, and band annotation

**Files:**
- Modify: `internal/repository/songs.go`
- Test: `internal/repository/bandsongs_test.go` (new)

- [ ] **Step 1: Write the failing tests** — `internal/repository/bandsongs_test.go`:

```go
package repository

import (
	"testing"

	"github.com/jwhumphries/bandwidth/internal/model"
)

func TestCreateAndListBandSongs(t *testing.T) {
	repo := testRepo(t)
	alice, _ := repo.CreateUser("alice", "alice@example.com", "h")
	band, _ := repo.CreateBand(alice.ID, "Band")

	song, err := repo.CreateBandSong(band.ID, "Wonderwall", "Oasis")
	if err != nil || song.OwnerBandID == nil || *song.OwnerBandID != band.ID {
		t.Fatalf("CreateBandSong: %+v (%v)", song, err)
	}

	items, err := repo.SongsForBand(band.ID)
	if err != nil || len(items) != 1 {
		t.Fatalf("SongsForBand: %v (%v)", items, err)
	}
	if items[0].Title != "Wonderwall" || items[0].Status != model.StatusNotLearned {
		t.Errorf("band song item = %+v", items[0])
	}

	// The band layer status is independent of any member's personal layer.
	status := model.StatusLearned
	if err := repo.UpsertBandAnnotation(song.ID, band.ID, &status, nil); err != nil {
		t.Fatalf("UpsertBandAnnotation: %v", err)
	}
	notes := "key of F#m"
	if err := repo.UpsertBandAnnotation(song.ID, band.ID, nil, &notes); err != nil {
		t.Fatal(err)
	}
	ann, err := repo.BandAnnotationForSong(song.ID, band.ID)
	if err != nil || ann.Status != model.StatusLearned || ann.Notes != "key of F#m" {
		t.Errorf("band annotation = %+v (%v)", ann, err)
	}
	// A member's personal annotation on the same band song is separate.
	personal := model.StatusLearning
	if err := repo.UpsertAnnotation(song.ID, alice.ID, &personal, nil); err != nil {
		t.Fatal(err)
	}
	items, _ = repo.SongsForBand(band.ID)
	if items[0].Status != model.StatusLearned {
		t.Errorf("band layer status leaked to/from user: %q", items[0].Status)
	}
}

func TestSongVisibleToUser(t *testing.T) {
	repo := testRepo(t)
	alice, _ := repo.CreateUser("alice", "alice@example.com", "h")
	bob, _ := repo.CreateUser("bob", "bob@example.com", "h")
	band, _ := repo.CreateBand(alice.ID, "Band")
	_ = repo.AddMember(band.ID, bob.ID, model.RoleViewer)
	bandSong, _ := repo.CreateBandSong(band.ID, "Shared", "Artist")
	ownSong, _ := repo.CreateSong(alice.ID, "Mine", "Artist")

	// Members (alice creator, bob viewer) can see the band song.
	for _, u := range []uint{alice.ID, bob.ID} {
		if _, err := repo.SongVisibleToUser(bandSong.ID, u); err != nil {
			t.Errorf("member %d cannot see band song: %v", u, err)
		}
	}
	// A non-member cannot.
	carol, _ := repo.CreateUser("carol", "carol@example.com", "h")
	if _, err := repo.SongVisibleToUser(bandSong.ID, carol.ID); err == nil {
		t.Error("non-member sees band song")
	}
	// Owner sees their own song; others don't.
	if _, err := repo.SongVisibleToUser(ownSong.ID, alice.ID); err != nil {
		t.Errorf("owner cannot see own song: %v", err)
	}
	if _, err := repo.SongVisibleToUser(ownSong.ID, bob.ID); err == nil {
		t.Error("non-owner sees personal song")
	}

	// SongForBand is band-scoped.
	if _, err := repo.SongForBand(bandSong.ID, band.ID); err != nil {
		t.Errorf("SongForBand(own band): %v", err)
	}
	if _, err := repo.SongForBand(ownSong.ID, band.ID); err == nil {
		t.Error("SongForBand returned a personal song")
	}
}
```

Run: `just test`. Expected: FAIL — undefined: CreateBandSong (etc.).

- [ ] **Step 2: Add to `internal/repository/songs.go`** — refactor the annotation pair to a subject core and add the band-song functions. First, replace the existing `AnnotationForSongUser` and `UpsertAnnotation` with subject-core-backed versions:

```go
func (r *Repo) annotationForSong(songID uint, s subj) (*model.SongAnnotation, error) {
	var ann model.SongAnnotation
	cond, id := s.scope()
	err := r.db.Where("song_id = ? AND "+cond, songID, id).First(&ann).Error
	if err != nil {
		return nil, err
	}
	return &ann, nil
}

func (r *Repo) upsertAnnotation(songID uint, s subj, status *model.SongStatus, notes *string) error {
	ann, err := r.annotationForSong(songID, s)
	switch {
	case err == nil:
		// existing row
	case errors.Is(err, gorm.ErrRecordNotFound):
		ann = &model.SongAnnotation{
			SongID: songID, UserID: s.userID, BandID: s.bandID,
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

// AnnotationForSongUser returns the user's annotation row, or
// gorm.ErrRecordNotFound when none exists yet.
func (r *Repo) AnnotationForSongUser(songID, userID uint) (*model.SongAnnotation, error) {
	return r.annotationForSong(songID, userSubj(userID))
}

// UpsertAnnotation lazily creates the user's annotation row and applies any
// provided fields (nil pointers leave the existing value untouched).
func (r *Repo) UpsertAnnotation(songID, userID uint, status *model.SongStatus, notes *string) error {
	return r.upsertAnnotation(songID, userSubj(userID), status, notes)
}

// BandAnnotationForSong returns the band's annotation row, or
// gorm.ErrRecordNotFound when none exists yet.
func (r *Repo) BandAnnotationForSong(songID, bandID uint) (*model.SongAnnotation, error) {
	return r.annotationForSong(songID, bandSubj(bandID))
}

// UpsertBandAnnotation lazily creates the band's annotation row and applies
// any provided fields.
func (r *Repo) UpsertBandAnnotation(songID, bandID uint, status *model.SongStatus, notes *string) error {
	return r.upsertAnnotation(songID, bandSubj(bandID), status, notes)
}
```

Then add the band-song ownership/listing/visibility functions to the same file:

```go
// CreateBandSong inserts a band-owned song.
func (r *Repo) CreateBandSong(bandID uint, title, artist string) (*model.Song, error) {
	song := &model.Song{Title: title, Artist: artist, OwnerBandID: &bandID}
	if err := r.db.Create(song).Error; err != nil {
		return nil, err
	}
	return song, nil
}

// SongsForBand returns a band's songs with the BAND's metadata layer and
// rehearsal aggregates joined in.
func (r *Repo) SongsForBand(bandID uint) ([]SongListItem, error) {
	items := []SongListItem{}
	err := r.db.Table("songs").
		Select(`songs.id, songs.title, songs.artist,
			COALESCE(sa.status, 'not_learned') AS status,
			COALESCE(pe.last_practiced_at, '') AS last_practiced_at,
			COALESCE(pe.practice_count, 0) AS practice_count`).
		Joins(`LEFT JOIN song_annotations sa
			ON sa.song_id = songs.id AND sa.band_id = ?`, bandID).
		Joins(`LEFT JOIN (
			SELECT song_id, MAX(date) AS last_practiced_at, COUNT(*) AS practice_count
			FROM practice_events WHERE band_id = ? GROUP BY song_id
		) pe ON pe.song_id = songs.id`, bandID).
		Where("songs.owner_band_id = ?", bandID).
		Order("songs.title COLLATE NOCASE, songs.id").
		Scan(&items).Error
	if err != nil {
		return nil, err
	}
	return items, nil
}

// SongForBand returns a band-owned song.
func (r *Repo) SongForBand(songID, bandID uint) (*model.Song, error) {
	var song model.Song
	err := r.db.Where("id = ? AND owner_band_id = ?", songID, bandID).First(&song).Error
	if err != nil {
		return nil, err
	}
	return &song, nil
}

// SongVisibleToUser returns a song the user can see: one they own, or one
// owned by a band they belong to.
func (r *Repo) SongVisibleToUser(songID, userID uint) (*model.Song, error) {
	var song model.Song
	err := r.db.
		Where(`id = ? AND (owner_user_id = ? OR owner_band_id IN
			(SELECT band_id FROM band_members WHERE user_id = ?))`,
			songID, userID, userID).
		First(&song).Error
	if err != nil {
		return nil, err
	}
	return &song, nil
}
```

- [ ] **Step 3: Run `just test` + `just lint-go`.** Expected: green/clean (existing annotation tests still pass; new band-song tests pass).

- [ ] **Step 4: Commit**

```bash
git add internal/repository/
git commit -m "feat: band song ownership, listing, visibility, and band annotation"
```

---

### Task 4: Personal library interleaving (band songs in the member's list)

**Files:**
- Modify: `internal/repository/songs.go` (`SongListItem`, `SongsForUser`)
- Test: `internal/repository/songs_test.go` (append)

- [ ] **Step 1: Write the failing test** — append to `internal/repository/songs_test.go`:

```go
func TestSongsForUserIncludesBandSongs(t *testing.T) {
	repo := testRepo(t)
	alice, _ := repo.CreateUser("alice", "alice@example.com", "h")
	bob, _ := repo.CreateUser("bob", "bob@example.com", "h")
	band, _ := repo.CreateBand(alice.ID, "The Quietones")
	_ = repo.AddMember(band.ID, bob.ID, model.RoleEditor)

	_, _ = repo.CreateSong(bob.ID, "My Own Song", "Me")
	bandSong, _ := repo.CreateBandSong(band.ID, "Band Song", "Them")

	// Bob's library shows both his own song and the band song.
	items, err := repo.SongsForUser(bob.ID)
	if err != nil || len(items) != 2 {
		t.Fatalf("SongsForUser: %d items (%v)", len(items), err)
	}
	var own, shared *SongListItem
	for i := range items {
		switch items[i].Title {
		case "My Own Song":
			own = &items[i]
		case "Band Song":
			shared = &items[i]
		}
	}
	if own == nil || own.BandID != nil {
		t.Errorf("own song tagged with band: %+v", own)
	}
	if shared == nil || shared.BandID == nil || *shared.BandID != band.ID || shared.BandName != "The Quietones" {
		t.Errorf("band song not tagged: %+v", shared)
	}

	// The status shown for the band song is BOB'S personal layer, not the
	// band's. Set a band status and a different personal status.
	bandStatus := model.StatusNailed
	_ = repo.UpsertBandAnnotation(bandSong.ID, band.ID, &bandStatus, nil)
	personal := model.StatusLearning
	_ = repo.UpsertAnnotation(bandSong.ID, bob.ID, &personal, nil)
	items, _ = repo.SongsForUser(bob.ID)
	for i := range items {
		if items[i].Title == "Band Song" && items[i].Status != model.StatusLearning {
			t.Errorf("band song list status = %q, want bob's personal 'learning'", items[i].Status)
		}
	}

	// A non-member's library never shows the band song.
	carol, _ := repo.CreateUser("carol", "carol@example.com", "h")
	if items, _ := repo.SongsForUser(carol.ID); len(items) != 0 {
		t.Errorf("non-member library = %d, want 0", len(items))
	}
}
```

Run: `just test`. Expected: FAIL — `SongListItem` has no `BandID` field / band song missing from list.

- [ ] **Step 2: Extend `SongListItem` and `SongsForUser`** in `internal/repository/songs.go`. Replace the struct:

```go
// SongListItem is one row of a user's library: identity plus the user's own
// metadata layer, with practice stats pre-aggregated. BandID/BandName are
// set only for band-owned songs (shared into the member's library).
type SongListItem struct {
	ID              uint             `json:"id"`
	Title           string           `json:"title"`
	Artist          string           `json:"artist"`
	Status          model.SongStatus `json:"status"`
	LastPracticedAt string           `json:"lastPracticedAt"`
	PracticeCount   int              `json:"practiceCount"`
	BandID          *uint            `json:"bandId,omitempty"`
	BandName        string           `json:"bandName,omitempty"`
}
```

and replace `SongsForUser`:

```go
// SongsForUser returns the member's library: songs they own plus songs owned
// by bands they belong to, each with the user's own annotation/practice
// layer (so a band song shows the member's personal status, not the band's).
func (r *Repo) SongsForUser(userID uint) ([]SongListItem, error) {
	items := []SongListItem{}
	err := r.db.Table("songs").
		Select(`songs.id, songs.title, songs.artist,
			COALESCE(sa.status, 'not_learned') AS status,
			COALESCE(pe.last_practiced_at, '') AS last_practiced_at,
			COALESCE(pe.practice_count, 0) AS practice_count,
			songs.owner_band_id AS band_id,
			COALESCE(b.name, '') AS band_name`).
		Joins(`LEFT JOIN song_annotations sa
			ON sa.song_id = songs.id AND sa.user_id = ?`, userID).
		Joins(`LEFT JOIN (
			SELECT song_id, MAX(date) AS last_practiced_at, COUNT(*) AS practice_count
			FROM practice_events WHERE user_id = ? GROUP BY song_id
		) pe ON pe.song_id = songs.id`, userID).
		Joins(`LEFT JOIN bands b ON b.id = songs.owner_band_id`).
		Where(`songs.owner_user_id = ? OR songs.owner_band_id IN
			(SELECT band_id FROM band_members WHERE user_id = ?)`, userID, userID).
		Order("songs.title COLLATE NOCASE, songs.id").
		Scan(&items).Error
	if err != nil {
		return nil, err
	}
	return items, nil
}
```

(`band_id` scans into `*uint`: NULL for owned songs → nil → omitted by `omitempty`. `band_name` is `COALESCE`d to `""` for owned songs → omitted.)

- [ ] **Step 3: Run `just test` + `just lint-go`.** Expected: green/clean. The Plan-3 `TestCreateAndListSongs` still passes (owned songs have `band_id` NULL → nil, `band_name` "" → both omitted).

- [ ] **Step 4: Commit**

```bash
git add internal/repository/
git commit -m "feat: interleave band songs into the member library"
```

---

### Task 5: Conversion engine + DeleteBandSong + leave/delete integration

**Files:**
- Create: `internal/repository/bandsongs.go`
- Modify: `internal/repository/bands.go` (`RemoveMember`, `DeleteBand`)
- Test: `internal/repository/conversion_test.go` (new)

- [ ] **Step 1: Write the failing tests** — `internal/repository/conversion_test.go`:

```go
package repository

import (
	"testing"

	"github.com/jwhumphries/bandwidth/internal/model"
)

// touchPersonalLayer gives the user some personal data on a band song.
func touchPersonalLayer(t *testing.T, repo *Repo, songID, userID uint) {
	t.Helper()
	s := model.StatusLearning
	if err := repo.UpsertAnnotation(songID, userID, &s, nil); err != nil {
		t.Fatal(err)
	}
	if _, err := repo.CreateResource(songID, userID, "https://example.com/mine", "mine"); err != nil {
		t.Fatal(err)
	}
	if err := repo.LogPractice(songID, userID, "2026-06-10"); err != nil {
		t.Fatal(err)
	}
}

func TestLeaveConvertsTouchedSongs(t *testing.T) {
	repo := testRepo(t)
	alice, _ := repo.CreateUser("alice", "alice@example.com", "h")
	bob, _ := repo.CreateUser("bob", "bob@example.com", "h")
	band, _ := repo.CreateBand(alice.ID, "Band")
	_ = repo.AddMember(band.ID, bob.ID, model.RoleEditor)
	touched, _ := repo.CreateBandSong(band.ID, "Touched", "X")
	untouched, _ := repo.CreateBandSong(band.ID, "Untouched", "Y")
	touchPersonalLayer(t, repo, touched.ID, bob.ID)

	if err := repo.RemoveMember(band.ID, bob.ID); err != nil {
		t.Fatalf("RemoveMember: %v", err)
	}

	// Bob's library now has exactly one song: a personal copy of "Touched".
	items, _ := repo.SongsForUser(bob.ID)
	if len(items) != 1 || items[0].Title != "Touched" || items[0].BandID != nil {
		t.Fatalf("bob library after leave = %+v", items)
	}
	// His personal layer survived on the copy.
	if items[0].Status != model.StatusLearning || items[0].PracticeCount != 1 {
		t.Errorf("converted song lost personal layer: %+v", items[0])
	}
	// The band still has both songs, untouched by bob's departure.
	bandItems, _ := repo.SongsForBand(band.ID)
	if len(bandItems) != 2 {
		t.Errorf("band lost songs: %d", len(bandItems))
	}
	// The band song "Touched" no longer carries bob's personal rows.
	var n int64
	repo.db.Model(&model.SongAnnotation{}).
		Where("song_id = ? AND user_id = ?", touched.ID, bob.ID).Count(&n)
	if n != 0 {
		t.Errorf("bob's annotation still on the band song: %d", n)
	}
	_ = untouched
}

func TestDeleteBandSongConvertsPerMember(t *testing.T) {
	repo := testRepo(t)
	alice, _ := repo.CreateUser("alice", "alice@example.com", "h")
	bob, _ := repo.CreateUser("bob", "bob@example.com", "h")
	band, _ := repo.CreateBand(alice.ID, "Band")
	_ = repo.AddMember(band.ID, bob.ID, model.RoleEditor)
	song, _ := repo.CreateBandSong(band.ID, "Doomed", "X")
	bandStatus := model.StatusNailed
	_ = repo.UpsertBandAnnotation(song.ID, band.ID, &bandStatus, nil)
	touchPersonalLayer(t, repo, song.ID, bob.ID) // bob touched it; alice did not

	if err := repo.DeleteBandSong(song.ID, band.ID); err != nil {
		t.Fatalf("DeleteBandSong: %v", err)
	}

	// The band song and its band layer are gone.
	if _, err := repo.SongForBand(song.ID, band.ID); err == nil {
		t.Error("band song survived delete")
	}
	var bandAnns int64
	repo.db.Model(&model.SongAnnotation{}).Where("song_id = ? AND band_id = ?", song.ID, band.ID).Count(&bandAnns)
	if bandAnns != 0 {
		t.Errorf("band annotation survived: %d", bandAnns)
	}
	// Bob (touched) keeps a personal copy; alice (untouched) gets nothing.
	if items, _ := repo.SongsForUser(bob.ID); len(items) != 1 || items[0].Title != "Doomed" {
		t.Errorf("bob library = %+v, want one converted copy", items)
	}
	if items, _ := repo.SongsForUser(alice.ID); len(items) != 0 {
		t.Errorf("alice library = %+v, want empty", items)
	}
}

func TestDeleteBandConvertsForAllMembers(t *testing.T) {
	repo := testRepo(t)
	alice, _ := repo.CreateUser("alice", "alice@example.com", "h")
	bob, _ := repo.CreateUser("bob", "bob@example.com", "h")
	band, _ := repo.CreateBand(alice.ID, "Band")
	_ = repo.AddMember(band.ID, bob.ID, model.RoleEditor)
	song, _ := repo.CreateBandSong(band.ID, "Shared", "X")
	touchPersonalLayer(t, repo, song.ID, bob.ID)

	if err := repo.DeleteBand(band.ID); err != nil {
		t.Fatalf("DeleteBand: %v", err)
	}
	if _, err := repo.BandByID(band.ID); err == nil {
		t.Error("band survived delete")
	}
	// Bob's touched song converted; the band song row is gone.
	items, _ := repo.SongsForUser(bob.ID)
	if len(items) != 1 || items[0].Title != "Shared" || items[0].BandID != nil {
		t.Errorf("bob library after band delete = %+v", items)
	}
	if items, _ := repo.SongsForUser(alice.ID); len(items) != 0 {
		t.Errorf("alice library after band delete = %+v", items)
	}
}
```

Run: `just test`. Expected: FAIL — undefined: `DeleteBandSong`; `RemoveMember`/`DeleteBand` don't convert.

- [ ] **Step 2: Write `internal/repository/bandsongs.go`**

```go
package repository

import (
	"gorm.io/gorm"

	"github.com/jwhumphries/bandwidth/internal/model"
)

// convertBandSongForUser preserves one member's personal work on a band song
// that is becoming unavailable to them. If the member has any personal rows
// (annotation, resource, or practice) on the song, a personal-copy song is
// created and those rows are re-pointed onto it; the band layer is not
// copied. Members who never touched the song get nothing. Runs inside tx.
func convertBandSongForUser(tx *gorm.DB, song *model.Song, userID uint) error {
	hasData, err := userTouchedSong(tx, song.ID, userID)
	if err != nil {
		return err
	}
	if !hasData {
		return nil
	}
	personal := &model.Song{Title: song.Title, Artist: song.Artist, OwnerUserID: &userID}
	if err := tx.Create(personal).Error; err != nil {
		return err
	}
	for _, m := range []any{&model.SongAnnotation{}, &model.Resource{}, &model.PracticeEvent{}} {
		err := tx.Model(m).
			Where("song_id = ? AND user_id = ?", song.ID, userID).
			Update("song_id", personal.ID).Error
		if err != nil {
			return err
		}
	}
	return nil
}

// userTouchedSong reports whether the user has any personal metadata row on
// the song.
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
	return false, nil
}

// deleteBandSongRows removes the band layer for one band song and the song
// row itself (callers convert member layers first). Runs inside tx.
func deleteBandSongRows(tx *gorm.DB, songID uint) error {
	for _, m := range []any{&model.PracticeEvent{}, &model.Resource{}, &model.SongAnnotation{}} {
		// Only band-layer rows remain after conversion (member rows were
		// re-pointed away), but scope by song_id to clear any band rows.
		if err := tx.Where("song_id = ? AND band_id IS NOT NULL", songID).Delete(m).Error; err != nil {
			return err
		}
	}
	return tx.Delete(&model.Song{}, songID).Error
}

// DeleteBandSong removes a band-owned song: every member with personal work
// on it keeps a personal copy, then the band song and its band layer are
// deleted. Atomic.
func (r *Repo) DeleteBandSong(songID, bandID uint) error {
	return r.db.Transaction(func(tx *gorm.DB) error {
		var song model.Song
		if err := tx.Where("id = ? AND owner_band_id = ?", songID, bandID).
			First(&song).Error; err != nil {
			return err
		}
		var members []model.BandMember
		if err := tx.Where("band_id = ?", bandID).Find(&members).Error; err != nil {
			return err
		}
		for _, m := range members {
			if err := convertBandSongForUser(tx, &song, m.UserID); err != nil {
				return err
			}
		}
		return deleteBandSongRows(tx, songID)
	})
}
```

- [ ] **Step 3: Rewrite `RemoveMember` and `DeleteBand`** in `internal/repository/bands.go` to run conversion. Replace those two methods:

```go
// RemoveMember removes a user from a band. Before the membership is dropped,
// each band song the member personally touched is converted to a personal
// copy (preserving their notes/status/resources/practice); the band's own
// songs are untouched.
func (r *Repo) RemoveMember(bandID, userID uint) error {
	return r.db.Transaction(func(tx *gorm.DB) error {
		var member model.BandMember
		if err := tx.Where("band_id = ? AND user_id = ?", bandID, userID).
			First(&member).Error; err != nil {
			return err // gorm.ErrRecordNotFound for non-members
		}
		var songs []model.Song
		if err := tx.Where("owner_band_id = ?", bandID).Find(&songs).Error; err != nil {
			return err
		}
		for i := range songs {
			if err := convertBandSongForUser(tx, &songs[i], userID); err != nil {
				return err
			}
		}
		return tx.Delete(&member).Error
	})
}

// DeleteBand removes the band, its songs, memberships, and invites. Each
// member's personal work on the band's songs is converted to personal copies
// first. Atomic.
func (r *Repo) DeleteBand(bandID uint) error {
	return r.db.Transaction(func(tx *gorm.DB) error {
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
	})
}
```

IMPORTANT: the Plan-4a `RemoveMember`/`DeleteBand` tests must still pass. The new `RemoveMember` still returns `gorm.ErrRecordNotFound` for a non-member (the `First` fails), and still removes a real member. The new `DeleteBand` on a band with no songs skips the song loop and deletes members/invites/band as before. Run the full repository suite to confirm.

- [ ] **Step 4: Run `just test` + `just lint-go`.** Expected: green/clean (Plan-4a band tests + the new conversion tests).

- [ ] **Step 5: Commit**

```bash
git add internal/repository/
git commit -m "feat: band song conversion engine for leave and delete"
```

---

### Task 6: Band song handlers (band view)

**Files:**
- Create: `internal/handlers/bandsongs.go`
- Test: `internal/handlers/bandsongs_test.go`

- [ ] **Step 1: Write the failing tests** — `internal/handlers/bandsongs_test.go`:

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

func newBandSongsAPI(t *testing.T) (*echo.Echo, *API) {
	t.Helper()
	e, api := newBandsAPI(t) // from bands_test.go: creates band CRUD routes
	g := e.Group("/api/bands/:id/songs", appmw.RequireAuth(api.Repo))
	g.GET("", api.BandSongs)
	g.POST("", api.CreateBandSong)
	g.GET("/:songId", api.BandSong)
	g.PATCH("/:songId", api.UpdateBandSong)
	g.DELETE("/:songId", api.DeleteBandSong)
	return e, api
}

func createBandSongFor(t *testing.T, e *echo.Echo, cookie *http.Cookie, bandID uint, title string) uint {
	t.Helper()
	rec := jsonReq(e, http.MethodPost, fmt.Sprintf("/api/bands/%d/songs", bandID),
		fmt.Sprintf(`{"title":%q}`, title), cookie)
	if rec.Code != http.StatusCreated {
		t.Fatalf("create band song: %d %s", rec.Code, rec.Body.String())
	}
	var created struct {
		ID uint `json:"id"`
	}
	_ = json.Unmarshal(rec.Body.Bytes(), &created)
	return created.ID
}

func TestBandSongCRUD(t *testing.T) {
	e, api := newBandSongsAPI(t)
	alice := signupAndCookie(t, e, "alice") // creator/admin
	bob := signupAndCookie(t, e, "bob")
	band := createBandFor(t, e, alice, "Band")
	_ = api.Repo.AddMember(band, mustUserID(t, api, "bob"), model.RoleViewer)

	songID := createBandSongFor(t, e, alice, band, "Wonderwall")

	// List (any member, incl. viewer).
	rec := jsonReq(e, http.MethodGet, fmt.Sprintf("/api/bands/%d/songs", band), "", bob)
	var list []map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &list); err != nil || len(list) != 1 {
		t.Fatalf("band songs list: %s (%v)", rec.Body.String(), err)
	}

	// Detail shows band-layer fields.
	rec = jsonReq(e, http.MethodGet, fmt.Sprintf("/api/bands/%d/songs/%d", band, songID), "", bob)
	var detail struct {
		Title          string `json:"title"`
		Status         string `json:"status"`
		RehearsalCount int    `json:"rehearsalCount"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &detail); err != nil {
		t.Fatal(err)
	}
	if detail.Title != "Wonderwall" || detail.Status != "not_learned" {
		t.Errorf("band song detail = %+v", detail)
	}

	// Update band identity + band annotation (Editor+; alice is admin).
	rec = jsonReq(e, http.MethodPatch, fmt.Sprintf("/api/bands/%d/songs/%d", band, songID),
		`{"artist":"Oasis","status":"learned","notes":"capo 2"}`, alice)
	if rec.Code != http.StatusOK {
		t.Fatalf("update: %d %s", rec.Code, rec.Body.String())
	}
	rec = jsonReq(e, http.MethodGet, fmt.Sprintf("/api/bands/%d/songs/%d", band, songID), "", alice)
	var after struct {
		Artist string `json:"artist"`
		Status string `json:"status"`
		Notes  string `json:"notes"`
	}
	_ = json.Unmarshal(rec.Body.Bytes(), &after)
	if after.Artist != "Oasis" || after.Status != "learned" || after.Notes != "capo 2" {
		t.Errorf("after update = %+v", after)
	}

	// Viewer cannot create, update, or delete (403).
	if rec := jsonReq(e, http.MethodPost, fmt.Sprintf("/api/bands/%d/songs", band), `{"title":"X"}`, bob); rec.Code != http.StatusForbidden {
		t.Fatalf("viewer create: %d, want 403", rec.Code)
	}
	if rec := jsonReq(e, http.MethodPatch, fmt.Sprintf("/api/bands/%d/songs/%d", band, songID), `{"status":"nailed"}`, bob); rec.Code != http.StatusForbidden {
		t.Fatalf("viewer update: %d, want 403", rec.Code)
	}
	if rec := jsonReq(e, http.MethodDelete, fmt.Sprintf("/api/bands/%d/songs/%d", band, songID), "", bob); rec.Code != http.StatusForbidden {
		t.Fatalf("viewer delete: %d, want 403", rec.Code)
	}

	// Non-members get 404 for the band (band invisible).
	carol := signupAndCookie(t, e, "carol")
	if rec := jsonReq(e, http.MethodGet, fmt.Sprintf("/api/bands/%d/songs", band), "", carol); rec.Code != http.StatusNotFound {
		t.Fatalf("non-member list: %d, want 404", rec.Code)
	}

	// A song from another band is not reachable via this band's path.
	otherBand := createBandFor(t, e, alice, "Other")
	otherSong := createBandSongFor(t, e, alice, otherBand, "Elsewhere")
	if rec := jsonReq(e, http.MethodGet, fmt.Sprintf("/api/bands/%d/songs/%d", band, otherSong), "", alice); rec.Code != http.StatusNotFound {
		t.Fatalf("cross-band song: %d, want 404", rec.Code)
	}

	// Admin deletes.
	if rec := jsonReq(e, http.MethodDelete, fmt.Sprintf("/api/bands/%d/songs/%d", band, songID), "", alice); rec.Code != http.StatusNoContent {
		t.Fatalf("delete: %d", rec.Code)
	}
	if rec := jsonReq(e, http.MethodGet, fmt.Sprintf("/api/bands/%d/songs/%d", band, songID), "", alice); rec.Code != http.StatusNotFound {
		t.Fatalf("detail after delete: %d, want 404", rec.Code)
	}
}
```

Run: `just test`. Expected: FAIL — undefined: (*API).BandSongs (etc.).

- [ ] **Step 2: Write `internal/handlers/bandsongs.go`**

```go
package handlers

import (
	"errors"
	"net/http"
	"strconv"
	"strings"

	"github.com/labstack/echo/v5"
	"gorm.io/gorm"

	"github.com/jwhumphries/bandwidth/internal/model"
)

// bandSongID parses the :songId path parameter.
func bandSongID(c *echo.Context) (uint, error) {
	id, err := strconv.ParseUint(c.Param("songId"), 10, 32)
	if err != nil || id == 0 {
		return 0, echo.NewHTTPError(http.StatusNotFound, "song not found")
	}
	return uint(id), nil
}

// bandSongForRequest resolves :songId and confirms it belongs to bandID.
func (a *API) bandSongForRequest(c *echo.Context, bandID uint) (*model.Song, error) {
	sid, err := bandSongID(c)
	if err != nil {
		return nil, err
	}
	song, err := a.Repo.SongForBand(sid, bandID)
	if err != nil {
		return nil, notFoundOr(err, "song")
	}
	return song, nil
}

// bandSongDetailResponse builds the band-layer detail for one band song.
func (a *API) bandSongDetailResponse(song *model.Song, bandID uint) (map[string]any, error) {
	status := model.StatusNotLearned
	notes := ""
	if ann, err := a.Repo.BandAnnotationForSong(song.ID, bandID); err == nil {
		status = ann.Status
		notes = ann.Notes
	} else if !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, err
	}
	resources, err := a.Repo.ResourcesForSongBand(song.ID, bandID)
	if err != nil {
		return nil, err
	}
	last, count, err := a.Repo.BandPracticeStats(song.ID, bandID)
	if err != nil {
		return nil, err
	}
	resList := make([]map[string]any, 0, len(resources))
	for _, r := range resources {
		resList = append(resList, map[string]any{"id": r.ID, "url": r.URL, "label": r.Label})
	}
	return map[string]any{
		"id":              song.ID,
		"title":           song.Title,
		"artist":          song.Artist,
		"status":          status,
		"notes":           notes,
		"resources":       resList,
		"lastRehearsedAt": last,
		"rehearsalCount":  count,
	}, nil
}

// BandSongs lists a band's songs with the band layer (any member).
func (a *API) BandSongs(c *echo.Context) error {
	bandID, _, err := a.bandAccess(c, model.RoleViewer)
	if err != nil {
		return err
	}
	items, err := a.Repo.SongsForBand(bandID)
	if err != nil {
		return err
	}
	return c.JSON(http.StatusOK, items)
}

type bandSongCreateRequest struct {
	Title  string `json:"title"`
	Artist string `json:"artist"`
}

// CreateBandSong adds a band song (Editor+).
func (a *API) CreateBandSong(c *echo.Context) error {
	bandID, _, err := a.bandAccess(c, model.RoleEditor)
	if err != nil {
		return err
	}
	var req bandSongCreateRequest
	if err := c.Bind(&req); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid request body")
	}
	req.Title = strings.TrimSpace(req.Title)
	req.Artist = strings.TrimSpace(req.Artist)
	if req.Title == "" || len(req.Title) > maxTitleLen || len(req.Artist) > maxTitleLen {
		return echo.NewHTTPError(http.StatusBadRequest,
			"a title (at most 200 characters) is required")
	}
	song, err := a.Repo.CreateBandSong(bandID, req.Title, req.Artist)
	if err != nil {
		return err
	}
	detail, err := a.bandSongDetailResponse(song, bandID)
	if err != nil {
		return err
	}
	return c.JSON(http.StatusCreated, detail)
}

// BandSong returns a band song's band-layer detail (any member).
func (a *API) BandSong(c *echo.Context) error {
	bandID, _, err := a.bandAccess(c, model.RoleViewer)
	if err != nil {
		return err
	}
	song, err := a.bandSongForRequest(c, bandID)
	if err != nil {
		return err
	}
	detail, err := a.bandSongDetailResponse(song, bandID)
	if err != nil {
		return err
	}
	return c.JSON(http.StatusOK, detail)
}

type bandSongUpdateRequest struct {
	Title  *string `json:"title"`
	Artist *string `json:"artist"`
	Status *string `json:"status"`
	Notes  *string `json:"notes"`
}

// UpdateBandSong patches a band song's identity and band annotation (Editor+),
// validating all fields before any write.
func (a *API) UpdateBandSong(c *echo.Context) error {
	bandID, _, err := a.bandAccess(c, model.RoleEditor)
	if err != nil {
		return err
	}
	song, err := a.bandSongForRequest(c, bandID)
	if err != nil {
		return err
	}
	var req bandSongUpdateRequest
	if err := c.Bind(&req); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid request body")
	}

	var stagedTitle *string
	if req.Title != nil {
		title := strings.TrimSpace(*req.Title)
		if title == "" || len(title) > maxTitleLen {
			return echo.NewHTTPError(http.StatusBadRequest, "title must be 1-200 characters")
		}
		stagedTitle = &title
	}
	var stagedArtist *string
	if req.Artist != nil {
		artist := strings.TrimSpace(*req.Artist)
		if len(artist) > maxTitleLen {
			return echo.NewHTTPError(http.StatusBadRequest, "artist must be at most 200 characters")
		}
		stagedArtist = &artist
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

	if stagedTitle != nil || stagedArtist != nil {
		if stagedTitle != nil {
			song.Title = *stagedTitle
		}
		if stagedArtist != nil {
			song.Artist = *stagedArtist
		}
		if err := a.Repo.SaveSong(song); err != nil {
			return err
		}
	}
	if status != nil || req.Notes != nil {
		if err := a.Repo.UpsertBandAnnotation(song.ID, bandID, status, req.Notes); err != nil {
			return err
		}
	}

	detail, err := a.bandSongDetailResponse(song, bandID)
	if err != nil {
		return err
	}
	return c.JSON(http.StatusOK, detail)
}

// DeleteBandSong removes a band song (Editor+); each member's personal work
// is converted to a personal copy by the repository.
func (a *API) DeleteBandSong(c *echo.Context) error {
	bandID, _, err := a.bandAccess(c, model.RoleEditor)
	if err != nil {
		return err
	}
	song, err := a.bandSongForRequest(c, bandID)
	if err != nil {
		return err
	}
	if err := a.Repo.DeleteBandSong(song.ID, bandID); err != nil {
		return err
	}
	return c.NoContent(http.StatusNoContent)
}
```

- [ ] **Step 3: Run `just test` + `just lint-go`.** Expected: green/clean.

- [ ] **Step 4: Commit**

```bash
git add internal/handlers/
git commit -m "feat: band song handlers (band view)"
```

---

### Task 7: Band rehearsal + band resource handlers

**Files:**
- Create: `internal/handlers/bandpractice.go`
- Test: `internal/handlers/bandpractice_test.go`

- [ ] **Step 1: Write the failing tests** — `internal/handlers/bandpractice_test.go`:

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
	"github.com/jwhumphries/bandwidth/internal/model"
)

func newBandPracticeAPI(t *testing.T) (*echo.Echo, *API) {
	t.Helper()
	e, api := newBandSongsAPI(t)
	g := e.Group("/api/bands/:id/songs/:songId", appmw.RequireAuth(api.Repo))
	g.PUT("/rehearsal", api.LogBandRehearsal)
	g.DELETE("/rehearsal/:date", api.DeleteBandRehearsal)
	g.POST("/resources", api.CreateBandResource)
	g.PATCH("/resources/:resourceId", api.UpdateBandResource)
	g.DELETE("/resources/:resourceId", api.DeleteBandResource)
	return e, api
}

func TestBandRehearsalAndResources(t *testing.T) {
	e, api := newBandPracticeAPI(t)
	alice := signupAndCookie(t, e, "alice")
	bob := signupAndCookie(t, e, "bob")
	band := createBandFor(t, e, alice, "Band")
	_ = api.Repo.AddMember(band, mustUserID(t, api, "bob"), model.RoleViewer)
	song := createBandSongFor(t, e, alice, band, "Wonderwall")

	base := fmt.Sprintf("/api/bands/%d/songs/%d", band, song)

	// Log a rehearsal (Editor+); response is band rehearsal stats.
	rec := jsonReq(e, http.MethodPut, base+"/rehearsal", `{"date":"2026-06-10"}`, alice)
	if rec.Code != http.StatusOK {
		t.Fatalf("rehearsal: %d %s", rec.Code, rec.Body.String())
	}
	var stats struct {
		LastRehearsedAt string `json:"lastRehearsedAt"`
		RehearsalCount  int    `json:"rehearsalCount"`
	}
	_ = json.Unmarshal(rec.Body.Bytes(), &stats)
	if stats.LastRehearsedAt != "2026-06-10" || stats.RehearsalCount != 1 {
		t.Errorf("rehearsal stats = %+v", stats)
	}

	// Empty body defaults to today.
	rec = jsonReq(e, http.MethodPut, base+"/rehearsal", "{}", alice)
	_ = json.Unmarshal(rec.Body.Bytes(), &stats)
	if stats.LastRehearsedAt != time.Now().UTC().Format("2006-01-02") {
		t.Errorf("default date = %q", stats.LastRehearsedAt)
	}

	// Viewer cannot log rehearsals.
	if rec := jsonReq(e, http.MethodPut, base+"/rehearsal", "{}", bob); rec.Code != http.StatusForbidden {
		t.Fatalf("viewer rehearsal: %d, want 403", rec.Code)
	}

	// Undo a rehearsal.
	if rec := jsonReq(e, http.MethodDelete, base+"/rehearsal/2026-06-10", "", alice); rec.Code != http.StatusOK {
		t.Fatalf("delete rehearsal: %d", rec.Code)
	}

	// Band resource lifecycle (Editor+).
	rec = jsonReq(e, http.MethodPost, base+"/resources",
		`{"url":"https://example.com/tab","label":"tab"}`, alice)
	if rec.Code != http.StatusCreated {
		t.Fatalf("create band resource: %d %s", rec.Code, rec.Body.String())
	}
	var res struct {
		ID uint `json:"id"`
	}
	_ = json.Unmarshal(rec.Body.Bytes(), &res)
	if rec := jsonReq(e, http.MethodPatch, fmt.Sprintf("%s/resources/%d", base, res.ID), `{"label":"chords"}`, alice); rec.Code != http.StatusOK {
		t.Fatalf("update band resource: %d", rec.Code)
	}
	if rec := jsonReq(e, http.MethodDelete, fmt.Sprintf("%s/resources/%d", base, res.ID), "", alice); rec.Code != http.StatusNoContent {
		t.Fatalf("delete band resource: %d", rec.Code)
	}
	// Viewer cannot create band resources.
	if rec := jsonReq(e, http.MethodPost, base+"/resources", `{"url":"https://example.com"}`, bob); rec.Code != http.StatusForbidden {
		t.Fatalf("viewer band resource: %d, want 403", rec.Code)
	}
}
```

Run: `just test`. Expected: FAIL — undefined: (*API).LogBandRehearsal (etc.).

- [ ] **Step 2: Write `internal/handlers/bandpractice.go`**

```go
package handlers

import (
	"net/http"
	"strings"
	"time"

	"github.com/labstack/echo/v5"

	"github.com/jwhumphries/bandwidth/internal/model"
)

func (a *API) bandRehearsalStatsResponse(c *echo.Context, songID, bandID uint) error {
	last, count, err := a.Repo.BandPracticeStats(songID, bandID)
	if err != nil {
		return err
	}
	return c.JSON(http.StatusOK, map[string]any{
		"lastRehearsedAt": last,
		"rehearsalCount":  count,
	})
}

type logRehearsalRequest struct {
	Date string `json:"date"`
}

// LogBandRehearsal records a band rehearsal day (Editor+). Default: today UTC.
func (a *API) LogBandRehearsal(c *echo.Context) error {
	bandID, _, err := a.bandAccess(c, model.RoleEditor)
	if err != nil {
		return err
	}
	song, err := a.bandSongForRequest(c, bandID)
	if err != nil {
		return err
	}
	var req logRehearsalRequest
	if err := c.Bind(&req); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid request body")
	}
	if req.Date == "" {
		req.Date = time.Now().UTC().Format("2006-01-02")
	}
	if !validPracticeDate(req.Date) {
		return echo.NewHTTPError(http.StatusBadRequest, "date must be YYYY-MM-DD and not in the future")
	}
	if err := a.Repo.LogBandPractice(song.ID, bandID, req.Date); err != nil {
		return err
	}
	return a.bandRehearsalStatsResponse(c, song.ID, bandID)
}

// DeleteBandRehearsal removes a band rehearsal day (Editor+).
func (a *API) DeleteBandRehearsal(c *echo.Context) error {
	bandID, _, err := a.bandAccess(c, model.RoleEditor)
	if err != nil {
		return err
	}
	song, err := a.bandSongForRequest(c, bandID)
	if err != nil {
		return err
	}
	date := c.Param("date")
	if !validPracticeDate(date) {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid date")
	}
	if err := a.Repo.DeleteBandPractice(song.ID, bandID, date); err != nil {
		return err
	}
	return a.bandRehearsalStatsResponse(c, song.ID, bandID)
}

func bandResourceResponse(r *model.Resource) map[string]any {
	return map[string]any{"id": r.ID, "url": r.URL, "label": r.Label}
}

// CreateBandResource appends a band resource (Editor+).
func (a *API) CreateBandResource(c *echo.Context) error {
	bandID, _, err := a.bandAccess(c, model.RoleEditor)
	if err != nil {
		return err
	}
	song, err := a.bandSongForRequest(c, bandID)
	if err != nil {
		return err
	}
	var req resourceRequest // {URL *string; Label *string} from resources.go
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
	res, err := a.Repo.CreateBandResource(song.ID, bandID, *req.URL, label)
	if err != nil {
		return err
	}
	return c.JSON(http.StatusCreated, bandResourceResponse(res))
}

// UpdateBandResource patches a band resource (Editor+).
func (a *API) UpdateBandResource(c *echo.Context) error {
	bandID, _, err := a.bandAccess(c, model.RoleEditor)
	if err != nil {
		return err
	}
	song, err := a.bandSongForRequest(c, bandID)
	if err != nil {
		return err
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
	res, err := a.Repo.UpdateBandResource(rid, song.ID, bandID, req.URL, req.Label)
	if err != nil {
		return notFoundOr(err, "resource")
	}
	return c.JSON(http.StatusOK, bandResourceResponse(res))
}

// DeleteBandResource removes a band resource (Editor+).
func (a *API) DeleteBandResource(c *echo.Context) error {
	bandID, _, err := a.bandAccess(c, model.RoleEditor)
	if err != nil {
		return err
	}
	song, err := a.bandSongForRequest(c, bandID)
	if err != nil {
		return err
	}
	rid, err := resourceID(c)
	if err != nil {
		return err
	}
	if err := a.Repo.DeleteBandResource(rid, song.ID, bandID); err != nil {
		return notFoundOr(err, "resource")
	}
	return c.NoContent(http.StatusNoContent)
}
```

- [ ] **Step 3: Run `just test` + `just lint-go`.** Expected: green/clean.

- [ ] **Step 4: Commit**

```bash
git add internal/handlers/
git commit -m "feat: band rehearsal and band resource handlers"
```

---

### Task 8: Personal-view interleaving (band songs in the member's library)

**Files:**
- Modify: `internal/handlers/songs.go` (`songDetailResponse`, `Song`, `UpdateSong`, `DeleteSong`)
- Modify: `internal/handlers/practice.go` (visibility guards), `internal/handlers/resources.go` (visibility guard)
- Test: `internal/handlers/interleave_test.go` (new)

- [ ] **Step 1: Write the failing tests** — `internal/handlers/interleave_test.go`:

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

// newInterleaveAPI wires the personal song routes plus band-song creation,
// so a band song can be set up and then viewed from the personal side.
func newInterleaveAPI(t *testing.T) (*echo.Echo, *API) {
	t.Helper()
	e, api := newBandSongsAPI(t) // band CRUD + band song routes
	g := e.Group("/api/songs", appmw.RequireAuth(api.Repo))
	g.GET("", api.Songs)
	g.GET("/:id", api.Song)
	g.PATCH("/:id", api.UpdateSong)
	g.DELETE("/:id", api.DeleteSong)
	g.PUT("/:id/practice", api.LogPractice)
	g.POST("/:id/resources", api.CreateResource)
	return e, api
}

func TestBandSongInPersonalView(t *testing.T) {
	e, api := newInterleaveAPI(t)
	alice := signupAndCookie(t, e, "alice")
	band := createBandFor(t, e, alice, "The Quietones")
	songID := createBandSongFor(t, e, alice, band, "Wonderwall")
	// Give the band layer a status + a band resource.
	_ = api.Repo.UpsertBandAnnotation(songID, band, ptrStatus(model.StatusNailed), ptrString("band notes"))
	_, _ = api.Repo.CreateBandResource(songID, band, "https://example.com/band", "band tab")

	// The band song appears in alice's personal library, tagged.
	rec := jsonReq(e, http.MethodGet, "/api/songs", "", alice)
	var list []map[string]any
	_ = json.Unmarshal(rec.Body.Bytes(), &list)
	if len(list) != 1 || list[0]["bandId"] == nil {
		t.Fatalf("personal list missing tagged band song: %s", rec.Body.String())
	}

	// Personal detail shows the user's own (default) layer plus a read-only
	// band section.
	rec = jsonReq(e, http.MethodGet, fmt.Sprintf("/api/songs/%d", songID), "", alice)
	if rec.Code != http.StatusOK {
		t.Fatalf("personal detail: %d %s", rec.Code, rec.Body.String())
	}
	var detail struct {
		Status string `json:"status"`
		Band   *struct {
			BandName       string `json:"bandName"`
			Status         string `json:"status"`
			Notes          string `json:"notes"`
			RehearsalCount int    `json:"rehearsalCount"`
			Resources      []any  `json:"resources"`
		} `json:"band"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &detail); err != nil {
		t.Fatal(err)
	}
	if detail.Status != "not_learned" {
		t.Errorf("personal status = %q, want the member's own default", detail.Status)
	}
	if detail.Band == nil || detail.Band.BandName != "The Quietones" || detail.Band.Status != "nailed" || detail.Band.Notes != "band notes" {
		t.Fatalf("band section = %+v", detail.Band)
	}
	if len(detail.Band.Resources) != 1 {
		t.Errorf("band resources in section = %d", len(detail.Band.Resources))
	}

	// The member CAN set their personal status on the band song...
	if rec := jsonReq(e, http.MethodPatch, fmt.Sprintf("/api/songs/%d", songID), `{"status":"learning","notes":"my take"}`, alice); rec.Code != http.StatusOK {
		t.Fatalf("personal status patch: %d %s", rec.Code, rec.Body.String())
	}
	// ...and log personal practice on it...
	if rec := jsonReq(e, http.MethodPut, fmt.Sprintf("/api/songs/%d/practice", songID), `{"date":"2026-06-10"}`, alice); rec.Code != http.StatusOK {
		t.Fatalf("personal practice: %d", rec.Code)
	}
	// ...but NOT edit the band-owned identity from the personal view...
	if rec := jsonReq(e, http.MethodPatch, fmt.Sprintf("/api/songs/%d", songID), `{"title":"Hijacked"}`, alice); rec.Code != http.StatusForbidden {
		t.Fatalf("personal identity edit: %d, want 403", rec.Code)
	}
	// ...nor delete the band song from their library.
	if rec := jsonReq(e, http.MethodDelete, fmt.Sprintf("/api/songs/%d", songID), "", alice); rec.Code != http.StatusForbidden {
		t.Fatalf("personal delete of band song: %d, want 403", rec.Code)
	}

	// The band layer is unchanged by the member's personal edits.
	bandAnn, _ := api.Repo.BandAnnotationForSong(songID, band)
	if bandAnn.Status != model.StatusNailed {
		t.Errorf("band status changed by member: %q", bandAnn.Status)
	}
}

func ptrStatus(s model.SongStatus) *model.SongStatus { return &s }
func ptrString(s string) *string                     { return &s }
```

Run: `just test`. Expected: FAIL — band song not reachable from personal routes / no band section.

- [ ] **Step 2: Add the band section to `songDetailResponse`** in `internal/handlers/songs.go`. The function currently returns the flat map directly; capture it in a variable and append the band section. Replace the final `return map[string]any{...}, nil` block with:

```go
	detail := map[string]any{
		"id":              song.ID,
		"title":           song.Title,
		"artist":          song.Artist,
		"status":          status,
		"notes":           notes,
		"resources":       resList,
		"lastPracticedAt": last,
		"practiceCount":   count,
	}
	if song.OwnerBandID != nil {
		band, err := a.Repo.BandByID(*song.OwnerBandID)
		if err != nil {
			return nil, err
		}
		bandLayer, err := a.bandSongDetailResponse(song, *song.OwnerBandID)
		if err != nil {
			return nil, err
		}
		detail["band"] = map[string]any{
			"bandId":          band.ID,
			"bandName":        band.Name,
			"status":          bandLayer["status"],
			"notes":           bandLayer["notes"],
			"resources":       bandLayer["resources"],
			"lastRehearsedAt": bandLayer["lastRehearsedAt"],
			"rehearsalCount":  bandLayer["rehearsalCount"],
		}
	}
	return detail, nil
```

- [ ] **Step 3: Use visibility + identity guards in the personal handlers.**

In `Song` (GET), replace `song, err := a.Repo.SongForUser(id, user.ID)` with `song, err := a.Repo.SongVisibleToUser(id, user.ID)`.

In `UpdateSong`, replace `song, err := a.Repo.SongForUser(id, user.ID)` with `song, err := a.Repo.SongVisibleToUser(id, user.ID)`, and immediately after the `c.Bind(&req)` block insert the identity guard:

```go
	if song.OwnerBandID != nil && (req.Title != nil || req.Artist != nil) {
		return echo.NewHTTPError(http.StatusForbidden,
			"a band song's title and artist are managed in the band view")
	}
```

(Status/notes still flow to `UpsertAnnotation(song.ID, user.ID, ...)` — the member's personal layer — which is correct for band songs.)

In `DeleteSong`, replace the body's delete section so band songs are rejected:

```go
	song, err := a.Repo.SongVisibleToUser(id, user.ID)
	if err != nil {
		return notFoundOr(err, "song")
	}
	if song.OwnerBandID != nil {
		return echo.NewHTTPError(http.StatusForbidden,
			"band songs cannot be deleted from your library")
	}
	if err := a.Repo.DeleteSong(song.ID, user.ID); err != nil {
		return notFoundOr(err, "song")
	}
	return c.NoContent(http.StatusNoContent)
```

In `internal/handlers/practice.go`, both `SongForUser` guard lines (LogPractice and DeletePractice) become `SongVisibleToUser`. In `internal/handlers/resources.go`, the `SongForUser` guard in CreateResource becomes `SongVisibleToUser`. (UpdateResource/DeleteResource are guarded by the user-keyed resource lookup, which only finds the member's own resources, so they need no change.)

- [ ] **Step 4: Run `just test` + `just lint-go`.** Expected: green/clean. Plan-3 personal song tests still pass (owned songs are visible via `SongVisibleToUser`; `OwnerBandID` is nil so no guards fire).

- [ ] **Step 5: Commit**

```bash
git add internal/handlers/
git commit -m "feat: interleave band songs into the personal song view"
```

---

### Task 9: Route wiring + integration test

**Files:**
- Modify: `cmd/bandwidth/server.go`, `cmd/bandwidth/server_test.go`

- [ ] **Step 1: Failing integration test** — append to `cmd/bandwidth/server_test.go`:

```go
func TestBandSongInterleaveFlow(t *testing.T) {
	e := testServer(t)

	rec := do(e, http.MethodPost, "/api/auth/signup",
		`{"username":"alice","email":"alice@example.com","password":"hunter2hunter2"}`, nil)
	alice := rec.Result().Cookies()

	rec = do(e, http.MethodPost, "/api/bands", `{"name":"The Quietones"}`, alice)
	var band struct {
		ID uint `json:"id"`
	}
	_ = json.Unmarshal(rec.Body.Bytes(), &band)

	rec = do(e, http.MethodPost, fmt.Sprintf("/api/bands/%d/songs", band.ID),
		`{"title":"Wonderwall","artist":"Oasis"}`, alice)
	if rec.Code != http.StatusCreated {
		t.Fatalf("create band song: %d %s", rec.Code, rec.Body.String())
	}

	// The band song shows up in alice's personal library.
	rec = do(e, http.MethodGet, "/api/songs", "", alice)
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), "Wonderwall") {
		t.Fatalf("personal library: %d %s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "bandId") {
		t.Fatalf("band song not tagged in personal library: %s", rec.Body.String())
	}
}
```

(Add `"encoding/json"` and `"fmt"` to the test imports if not already present.)

Run: `just test`. Expected: FAIL — band song routes not wired (404 on POST /api/bands/:id/songs).

- [ ] **Step 2: Wire the routes** in `cmd/bandwidth/server.go` `newEcho`, inside the existing `bands` group (after the invite routes from Plan 4a), add:

```go
	bands.GET("/:id/songs", api.BandSongs)
	bands.POST("/:id/songs", api.CreateBandSong)
	bands.GET("/:id/songs/:songId", api.BandSong)
	bands.PATCH("/:id/songs/:songId", api.UpdateBandSong)
	bands.DELETE("/:id/songs/:songId", api.DeleteBandSong)
	bands.PUT("/:id/songs/:songId/rehearsal", api.LogBandRehearsal)
	bands.DELETE("/:id/songs/:songId/rehearsal/:date", api.DeleteBandRehearsal)
	bands.POST("/:id/songs/:songId/resources", api.CreateBandResource)
	bands.PATCH("/:id/songs/:songId/resources/:resourceId", api.UpdateBandResource)
	bands.DELETE("/:id/songs/:songId/resources/:resourceId", api.DeleteBandResource)
```

- [ ] **Step 3: `just test` green, then `just check` → "all checks passed". Commit:**

```bash
git add cmd/
git commit -m "feat: wire band song routes"
```

---

### Task 10: Frontend types + band-song hooks

**Files:**
- Modify: `frontend/src/lib/types.ts`
- Create: `frontend/src/hooks/bandsongs.ts`

- [ ] **Step 1: Extend `frontend/src/lib/types.ts`** — add `bandId`/`bandName` to `SongListItem`, a `BandLayer` and `BandSongDetail`, and a `band` field on `SongDetail`. Replace those interfaces:

```ts
export interface SongListItem {
  id: number;
  title: string;
  artist: string;
  status: SongStatus;
  lastPracticedAt: string;
  practiceCount: number;
  bandId?: number;
  bandName?: string;
}

export interface BandLayer {
  bandId: number;
  bandName: string;
  status: SongStatus;
  notes: string;
  resources: Resource[];
  lastRehearsedAt: string;
  rehearsalCount: number;
}

export interface SongDetail extends SongListItem {
  notes: string;
  resources: Resource[];
  band?: BandLayer;
}

export interface BandSongDetail {
  id: number;
  title: string;
  artist: string;
  status: SongStatus;
  notes: string;
  resources: Resource[];
  lastRehearsedAt: string;
  rehearsalCount: number;
}
```

(`Resource` and `SongStatus` already exist in this file from Plan 3 — leave them.)

- [ ] **Step 2: Write `frontend/src/hooks/bandsongs.ts`**

```ts
import {useMutation, useQuery, useQueryClient} from '@tanstack/react-query';
import {api, ApiError} from '../lib/api';
import type {BandSongDetail, Resource, SongListItem} from '../lib/types';

interface RehearsalStats {
  lastRehearsedAt: string;
  rehearsalCount: number;
}

export function useBandSongs(bandId: number) {
  return useQuery<SongListItem[], ApiError>({
    queryKey: ['bands', bandId, 'songs'],
    queryFn: () => api.get<SongListItem[]>(`/api/bands/${bandId}/songs`),
  });
}

export function useBandSong(bandId: number, songId: number) {
  return useQuery<BandSongDetail, ApiError>({
    queryKey: ['bands', bandId, 'songs', songId],
    queryFn: () => api.get<BandSongDetail>(`/api/bands/${bandId}/songs/${songId}`),
  });
}

// invalidateBandSong refreshes the band song list, the band song detail, and
// the member library (band songs surface there too).
function invalidateBandSong(
  queryClient: ReturnType<typeof useQueryClient>,
  bandId: number,
  songId?: number,
) {
  void queryClient.invalidateQueries({queryKey: ['bands', bandId, 'songs']});
  if (songId !== undefined) {
    void queryClient.invalidateQueries({queryKey: ['bands', bandId, 'songs', songId]});
  }
  void queryClient.invalidateQueries({queryKey: ['songs']});
}

export function useCreateBandSong(bandId: number) {
  const queryClient = useQueryClient();
  return useMutation<BandSongDetail, ApiError, {title: string; artist: string}>({
    mutationFn: data => api.post<BandSongDetail>(`/api/bands/${bandId}/songs`, data),
    onSuccess: () => invalidateBandSong(queryClient, bandId),
  });
}

export function useUpdateBandSong(bandId: number, songId: number) {
  const queryClient = useQueryClient();
  return useMutation<
    BandSongDetail,
    ApiError,
    {title?: string; artist?: string; status?: string; notes?: string}
  >({
    mutationFn: data => api.patch<BandSongDetail>(`/api/bands/${bandId}/songs/${songId}`, data),
    onSuccess: detail => {
      queryClient.setQueryData(['bands', bandId, 'songs', songId], detail);
      invalidateBandSong(queryClient, bandId);
    },
  });
}

export function useDeleteBandSong(bandId: number) {
  const queryClient = useQueryClient();
  return useMutation<void, ApiError, number>({
    mutationFn: songId => api.delete(`/api/bands/${bandId}/songs/${songId}`),
    onSuccess: () => invalidateBandSong(queryClient, bandId),
  });
}

export function useLogBandRehearsal(bandId: number, songId: number) {
  const queryClient = useQueryClient();
  return useMutation<RehearsalStats, ApiError, {date: string}>({
    mutationFn: data =>
      api.put<RehearsalStats>(`/api/bands/${bandId}/songs/${songId}/rehearsal`, data),
    onSuccess: () => invalidateBandSong(queryClient, bandId, songId),
  });
}

export function useCreateBandResource(bandId: number, songId: number) {
  const queryClient = useQueryClient();
  return useMutation<Resource, ApiError, {url: string; label: string}>({
    mutationFn: data =>
      api.post<Resource>(`/api/bands/${bandId}/songs/${songId}/resources`, data),
    onSuccess: () => invalidateBandSong(queryClient, bandId, songId),
  });
}

export function useDeleteBandResource(bandId: number, songId: number) {
  const queryClient = useQueryClient();
  return useMutation<void, ApiError, number>({
    mutationFn: resourceId =>
      api.delete(`/api/bands/${bandId}/songs/${songId}/resources/${resourceId}`),
    onSuccess: () => invalidateBandSong(queryClient, bandId, songId),
  });
}
```

- [ ] **Step 3: Checks** — `just typecheck && just lint-js && just format-check` (no new tests yet; `just test-frontend` still green). Commit:

```bash
git add frontend
git commit -m "feat: band song types and hooks"
```

---

### Task 11: Band song list in the band page

**Files:**
- Create: `frontend/src/components/bands/BandSongList.tsx`, `frontend/src/components/bands/AddBandSongModal.tsx`
- Modify: `frontend/src/pages/BandPage.tsx`
- Test: `frontend/src/components/bands/BandSongList.test.tsx`

- [ ] **Step 1: Failing test** — `frontend/src/components/bands/BandSongList.test.tsx`:

```tsx
import {screen, waitFor} from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import {beforeEach, describe, expect, it, vi} from 'vitest';
import {MemoryRouter} from 'react-router';
import {renderWithProviders} from '../../test/utils';
import BandSongList from './BandSongList';

const songs = [
  {id: 1, title: 'Wonderwall', artist: 'Oasis', status: 'learned', lastPracticedAt: '', practiceCount: 0},
];

function jsonResponse(status: number, body: unknown) {
  return new Response(JSON.stringify(body), {status});
}

describe('BandSongList', () => {
  beforeEach(() => {
    vi.stubGlobal(
      'fetch',
      vi.fn().mockImplementation((input: RequestInfo | URL, init?: RequestInit) => {
        if (init?.method === 'POST') {
          return Promise.resolve(jsonResponse(201, {id: 9}));
        }
        return Promise.resolve(jsonResponse(200, songs));
      }),
    );
  });

  it('lists band songs linking to the band-song detail', async () => {
    renderWithProviders(<BandSongList bandId={3} canEdit={false} />);
    const link = await screen.findByRole('link', {name: /wonderwall/i});
    expect(link).toHaveAttribute('href', '/bands/3/songs/1');
    // Viewers get no add control.
    expect(screen.queryByRole('button', {name: /add song/i})).not.toBeInTheDocument();
  });

  it('lets editors add a band song', async () => {
    renderWithProviders(<BandSongList bandId={3} canEdit={true} />);
    await screen.findByRole('link', {name: /wonderwall/i});
    await userEvent.click(screen.getByRole('button', {name: /add song/i}));
    await userEvent.type(screen.getByLabelText(/title/i), 'Creep');
    await userEvent.click(screen.getByRole('button', {name: /^add$/i}));
    await waitFor(() => {
      const calls = vi.mocked(fetch).mock.calls;
      expect(
        calls.some(
          ([input, init]) =>
            String(input).endsWith('/api/bands/3/songs') &&
            init?.method === 'POST' &&
            String(init.body).includes('Creep'),
        ),
      ).toBe(true);
    });
  });
});

// renderWithProviders already wraps in MemoryRouter; the explicit import keeps
// it available if a future test needs a custom route.
void MemoryRouter;
```

(If `renderWithProviders` already supplies a router, drop the `MemoryRouter` import and the trailing `void MemoryRouter;` line — adapt minimally and report.)

Run: `just test-frontend`. Expected: FAIL — cannot resolve `./BandSongList`.

- [ ] **Step 2: Write `frontend/src/components/bands/AddBandSongModal.tsx`**

```tsx
import {useState} from 'react';
import type {FormEvent} from 'react';
import {useCreateBandSong} from '../../hooks/bandsongs';

export default function AddBandSongModal({
  bandId,
  open,
  onClose,
}: {
  bandId: number;
  open: boolean;
  onClose: () => void;
}) {
  const createSong = useCreateBandSong(bandId);
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
    <dialog className={`modal ${open ? 'modal-open' : ''}`} open={open}>
      <div className="modal-box">
        <h3 className="text-lg font-bold">Add band song</h3>
        <form onSubmit={submit}>
          <label className="label" htmlFor="band-song-title">
            Title
          </label>
          <input
            id="band-song-title"
            className="input w-full"
            value={title}
            onChange={e => setTitle(e.target.value)}
            required
          />
          <label className="label" htmlFor="band-song-artist">
            Artist
          </label>
          <input
            id="band-song-artist"
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

- [ ] **Step 3: Write `frontend/src/components/bands/BandSongList.tsx`**

```tsx
import {useState} from 'react';
import {Link} from 'react-router';
import StatusBadge from '../songs/StatusBadge';
import AddBandSongModal from './AddBandSongModal';
import {useBandSongs} from '../../hooks/bandsongs';

export default function BandSongList({
  bandId,
  canEdit,
}: {
  bandId: number;
  canEdit: boolean;
}) {
  const {data: songs = []} = useBandSongs(bandId);
  const [adding, setAdding] = useState(false);

  return (
    <section className="card bg-base-100 shadow">
      <div className="card-body">
        <div className="flex items-center justify-between">
          <h2 className="card-title">Songs</h2>
          {canEdit && (
            <button className="btn btn-primary btn-sm" onClick={() => setAdding(true)}>
              Add song
            </button>
          )}
        </div>
        {songs.length === 0 ? (
          <p className="text-base-content/60 text-sm">No band songs yet.</p>
        ) : (
          <ul className="flex flex-col gap-2">
            {songs.map(song => (
              <li key={song.id}>
                <Link
                  to={`/bands/${bandId}/songs/${song.id}`}
                  className="bg-base-200 flex items-center gap-3 rounded-box p-3"
                >
                  <span className="min-w-0 flex-1">
                    <span className="block truncate font-semibold">{song.title}</span>
                    <span className="text-base-content/60 block truncate text-sm">
                      {song.artist || '—'}
                    </span>
                  </span>
                  <StatusBadge status={song.status} />
                </Link>
              </li>
            ))}
          </ul>
        )}
        <AddBandSongModal bandId={bandId} open={adding} onClose={() => setAdding(false)} />
      </div>
    </section>
  );
}
```

- [ ] **Step 4: Mount it in `frontend/src/pages/BandPage.tsx`** — add the import and render the list for every member (after `MemberList`, before the admin sections), passing `canEdit` for Editors/Admins:

```tsx
import BandSongList from '../components/bands/BandSongList';
```

and in the returned JSX:

```tsx
      <h1 className="text-3xl font-bold">{band.name}</h1>
      <BandSongList bandId={band.id} canEdit={band.myRole !== 'viewer'} />
      <MemberList band={band} />
      {isAdmin && <InviteManager bandId={band.id} />}
      {isAdmin && <BandSettings band={band} />}
```

- [ ] **Step 5: All four frontend checks green, commit**

```bash
git add frontend
git commit -m "feat: band song list and add modal in the band page"
```

---

### Task 12: Band song detail page (band view)

**Files:**
- Create: `frontend/src/pages/BandSongPage.tsx`, `frontend/src/pages/BandSongPage.test.tsx`
- Modify: `frontend/src/App.tsx` (route)

- [ ] **Step 1: Failing test** — `frontend/src/pages/BandSongPage.test.tsx`:

```tsx
import {screen, waitFor} from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import {beforeEach, describe, expect, it, vi} from 'vitest';
import {Route, Routes} from 'react-router';
import {renderWithProviders} from '../test/utils';
import BandSongPage from './BandSongPage';

const detail = {
  id: 1,
  title: 'Wonderwall',
  artist: 'Oasis',
  status: 'learning',
  notes: 'capo 2',
  resources: [{id: 5, url: 'https://example.com/tab', label: 'tab'}],
  lastRehearsedAt: '2026-06-10',
  rehearsalCount: 4,
};

function jsonResponse(status: number, body: unknown) {
  return new Response(JSON.stringify(body), {status});
}

// The band detail (for myRole) plus the band-song detail share one stub.
function stubFetch(myRole: string) {
  vi.stubGlobal(
    'fetch',
    vi.fn().mockImplementation((input: RequestInfo | URL, init?: RequestInit) => {
      const url = String(input);
      if (init?.method === 'PATCH') {
        return Promise.resolve(jsonResponse(200, {...detail, status: 'nailed'}));
      }
      if (init?.method === 'PUT' || init?.method === 'POST') {
        return Promise.resolve(jsonResponse(200, {lastRehearsedAt: '2026-06-11', rehearsalCount: 5}));
      }
      if (url.includes('/songs/1')) {
        return Promise.resolve(jsonResponse(200, detail));
      }
      // band detail
      return Promise.resolve(
        jsonResponse(200, {id: 3, name: 'The Quietones', creatorId: 1, myRole, members: []}),
      );
    }),
  );
}

function renderPage() {
  return renderWithProviders(
    <Routes>
      <Route path="/bands/:id/songs/:songId" element={<BandSongPage />} />
      <Route path="/bands/:id" element={<p>band page</p>} />
    </Routes>,
    {route: '/bands/3/songs/1'},
  );
}

describe('BandSongPage', () => {
  beforeEach(() => vi.unstubAllGlobals());

  it('shows the band layer and lets editors change status', async () => {
    stubFetch('admin');
    renderPage();
    expect(await screen.findByDisplayValue('Wonderwall')).toBeInTheDocument();
    expect(screen.getByText(/4 rehearsals/i)).toBeInTheDocument();
    await userEvent.selectOptions(screen.getByLabelText(/status/i), 'nailed');
    await waitFor(() => {
      const calls = vi.mocked(fetch).mock.calls;
      expect(
        calls.some(
          ([input, init]) =>
            String(input).includes('/bands/3/songs/1') &&
            init?.method === 'PATCH' &&
            String(init.body).includes('nailed'),
        ),
      ).toBe(true);
    });
  });

  it('renders read-only for viewers (no status control, no delete)', async () => {
    stubFetch('viewer');
    renderPage();
    await screen.findByText('Wonderwall');
    expect(screen.queryByLabelText(/status/i)).not.toBeInTheDocument();
    expect(screen.queryByRole('button', {name: /delete song/i})).not.toBeInTheDocument();
  });
});
```

Run: `just test-frontend`. Expected: FAIL — cannot resolve `./BandSongPage`.

- [ ] **Step 2: Write `frontend/src/pages/BandSongPage.tsx`**

```tsx
import {useEffect, useState} from 'react';
import type {FormEvent} from 'react';
import {Link, useNavigate, useParams} from 'react-router';
import ConfirmModal from '../components/songs/ConfirmModal';
import {useBand} from '../hooks/bands';
import {
  useBandSong,
  useCreateBandResource,
  useDeleteBandResource,
  useDeleteBandSong,
  useLogBandRehearsal,
  useUpdateBandSong,
} from '../hooks/bandsongs';
import {localToday} from '../lib/dates';
import type {SongStatus} from '../lib/types';

const statusOptions: {value: SongStatus; label: string}[] = [
  {value: 'not_learned', label: 'Not learned'},
  {value: 'learning', label: 'Learning'},
  {value: 'learned', label: 'Learned'},
  {value: 'nailed', label: 'Nailed!'},
];

export default function BandSongPage() {
  const {id: idParam, songId: songParam} = useParams();
  const bandId = Number(idParam);
  const songId = Number(songParam);
  const navigate = useNavigate();
  const {data: band} = useBand(bandId);
  const {data: song, isPending, isError, error, refetch} = useBandSong(bandId, songId);
  const updateSong = useUpdateBandSong(bandId, songId);
  const deleteSong = useDeleteBandSong(bandId);
  const logRehearsal = useLogBandRehearsal(bandId, songId);
  const createResource = useCreateBandResource(bandId, songId);
  const deleteResource = useDeleteBandResource(bandId, songId);

  const [title, setTitle] = useState('');
  const [artist, setArtist] = useState('');
  const [notes, setNotes] = useState('');
  const [dirty, setDirty] = useState(false);
  const [resUrl, setResUrl] = useState('');
  const [resLabel, setResLabel] = useState('');
  const [confirming, setConfirming] = useState(false);

  useEffect(() => {
    if (song && !dirty) {
      setTitle(song.title);
      setArtist(song.artist);
      setNotes(song.notes);
    }
  }, [song, dirty]);

  if (isPending) {
    return (
      <div className="flex justify-center py-12">
        <span className="loading loading-spinner" aria-label="Loading" />
      </div>
    );
  }
  if (isError || !song) {
    return (
      <div className="flex flex-col items-center gap-4 py-12">
        <p>{error?.message ?? 'Could not load this song.'}</p>
        <Link className="btn btn-ghost" to={`/bands/${bandId}`}>
          Back to band
        </Link>
      </div>
    );
  }

  const canEdit = band ? band.myRole !== 'viewer' : false;

  const save = (e: FormEvent) => {
    e.preventDefault();
    updateSong.mutate({title, artist, notes}, {onSuccess: () => setDirty(false)});
  };
  const addResource = (e: FormEvent) => {
    e.preventDefault();
    createResource.mutate(
      {url: resUrl, label: resLabel},
      {
        onSuccess: () => {
          setResUrl('');
          setResLabel('');
        },
      },
    );
  };

  return (
    <div className="flex flex-col gap-6">
      <Link className="link text-sm" to={`/bands/${bandId}`}>
        ← {band?.name ?? 'Band'}
      </Link>

      {canEdit ? (
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
              onChange={e => updateSong.mutate({status: e.target.value as SongStatus})}
            >
              {statusOptions.map(o => (
                <option key={o.value} value={o.value}>
                  {o.label}
                </option>
              ))}
            </select>
            <label className="label" htmlFor="notes">
              Band notes
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
      ) : (
        <div className="card bg-base-100 shadow">
          <div className="card-body">
            <h1 className="text-2xl font-bold">{song.title}</h1>
            <p className="text-base-content/60">{song.artist || '—'}</p>
            <p>
              Status: <span className="badge">{song.status}</span>
            </p>
            {song.notes && <p className="whitespace-pre-wrap">{song.notes}</p>}
          </div>
        </div>
      )}

      <section className="card bg-base-100 shadow">
        <div className="card-body">
          <h2 className="card-title">Rehearsals</h2>
          <p>
            {song.rehearsalCount} rehearsals
            {song.lastRehearsedAt && <> · last on {song.lastRehearsedAt}</>}
          </p>
          {canEdit && (
            <div className="card-actions">
              <button
                className="btn btn-outline"
                onClick={() => logRehearsal.mutate({date: localToday()})}
              >
                Rehearsed today
              </button>
            </div>
          )}
        </div>
      </section>

      <section className="card bg-base-100 shadow">
        <div className="card-body">
          <h2 className="card-title">Band links</h2>
          <ul className="flex flex-col gap-1">
            {song.resources.map(r => (
              <li key={r.id} className="flex items-center gap-2">
                <a
                  className="link min-w-0 flex-1 truncate"
                  href={r.url}
                  target="_blank"
                  rel="noreferrer noopener"
                >
                  {r.label || r.url}
                </a>
                {canEdit && (
                  <button
                    className="btn btn-ghost btn-xs"
                    aria-label={`Remove ${r.label || r.url}`}
                    onClick={() => deleteResource.mutate(r.id)}
                  >
                    ✕
                  </button>
                )}
              </li>
            ))}
          </ul>
          {canEdit && (
            <form className="flex flex-wrap gap-2" onSubmit={addResource}>
              <input
                className="input input-sm min-w-0 flex-1"
                placeholder="https://…"
                aria-label="Band resource URL"
                value={resUrl}
                onChange={e => setResUrl(e.target.value)}
                required
              />
              <input
                className="input input-sm w-32"
                placeholder="Label"
                aria-label="Band resource label"
                value={resLabel}
                onChange={e => setResLabel(e.target.value)}
              />
              <button className="btn btn-sm" disabled={createResource.isPending}>
                Add link
              </button>
            </form>
          )}
        </div>
      </section>

      {canEdit && (
        <section className="card bg-base-100 shadow">
          <div className="card-body">
            <h2 className="card-title">Danger zone</h2>
            <div className="card-actions">
              <button
                className="btn btn-error btn-outline"
                onClick={() => setConfirming(true)}
              >
                Delete song
              </button>
            </div>
          </div>
        </section>
      )}

      <ConfirmModal
        open={confirming}
        title="Delete band song"
        message={`Delete “${song.title}” from the band? Each member who personally tracked it keeps a personal copy.`}
        confirmLabel="Delete"
        onConfirm={() =>
          deleteSong.mutate(song.id, {onSuccess: () => void navigate(`/bands/${bandId}`)})
        }
        onCancel={() => setConfirming(false)}
      />
    </div>
  );
}
```

- [ ] **Step 3: Add the route** in `frontend/src/App.tsx` inside the Layout group (with the import):

```tsx
          <Route path="/bands/:id/songs/:songId" element={<BandSongPage />} />
```

- [ ] **Step 4: All four frontend checks green, commit**

```bash
git add frontend
git commit -m "feat: band song detail page (band view)"
```

---

### Task 13: Personal-view interleaving UI (band tag + read-only band section)

**Files:**
- Modify: `frontend/src/components/songs/SongRow.tsx` (band tag)
- Create: `frontend/src/components/songs/BandSection.tsx`
- Modify: `frontend/src/pages/SongPage.tsx` (mount band section; lock identity + hide delete for band songs)
- Test: `frontend/src/pages/SongPage.test.tsx` (append a band-song case)

- [ ] **Step 1: Failing test** — append to `frontend/src/pages/SongPage.test.tsx` a band-song case. Add this `describe` block (it stubs a band song detail with a `band` layer):

```tsx
describe('SongPage band song', () => {
  const bandDetail = {
    id: 2,
    title: 'Shared Tune',
    artist: 'The Band',
    status: 'not_learned',
    notes: '',
    resources: [],
    lastPracticedAt: '',
    practiceCount: 0,
    band: {
      bandId: 7,
      bandName: 'The Quietones',
      status: 'nailed',
      notes: 'band notes here',
      resources: [{id: 1, url: 'https://example.com/band', label: 'band tab'}],
      lastRehearsedAt: '2026-06-10',
      rehearsalCount: 9,
    },
  };

  beforeEach(() => {
    vi.stubGlobal(
      'fetch',
      vi.fn().mockResolvedValue(
        new Response(JSON.stringify(bandDetail), {status: 200}),
      ),
    );
  });

  function renderBandSong() {
    return renderWithProviders(
      <Routes>
        <Route path="/songs/:id" element={<SongPage />} />
        <Route path="/" element={<p>home</p>} />
      </Routes>,
      {route: '/songs/2'},
    );
  }

  it('shows the read-only band section and locks identity', async () => {
    renderBandSong();
    expect(await screen.findByText(/The Quietones/)).toBeInTheDocument();
    expect(screen.getByText(/band notes here/)).toBeInTheDocument();
    expect(screen.getByText(/9 rehearsals/i)).toBeInTheDocument();
    // The title input is disabled (band owns identity).
    expect(screen.getByLabelText(/title/i)).toBeDisabled();
    // No delete control for band songs in the personal view.
    expect(screen.queryByRole('button', {name: /delete song/i})).not.toBeInTheDocument();
  });
});
```

(This block reuses `SongPage`, `renderWithProviders`, `Routes`, `Route`, `screen`, `vi`, `beforeEach`, `describe`, `it`, `expect` already imported at the top of the existing test file. If any import is missing, add it.)

Run: `just test-frontend`. Expected: FAIL — no band section / title not disabled / delete still present.

- [ ] **Step 2: Add the band tag to `frontend/src/components/songs/SongRow.tsx`** — show the band name when present. Insert, right after the `<StatusBadge .../>` line:

```tsx
      {song.bandName && (
        <span className="badge badge-outline badge-sm">{song.bandName}</span>
      )}
```

- [ ] **Step 3: Write `frontend/src/components/songs/BandSection.tsx`**

```tsx
import type {BandLayer} from '../../lib/types';
import StatusBadge from './StatusBadge';

export default function BandSection({band}: {band: BandLayer}) {
  return (
    <section className="card bg-base-200 shadow">
      <div className="card-body">
        <h2 className="card-title">Band: {band.bandName}</h2>
        <p className="text-base-content/70 text-sm">
          Shared with your band — read-only here; edit it in the band view.
        </p>
        <div className="flex items-center gap-2">
          <span>Band status:</span>
          <StatusBadge status={band.status} />
        </div>
        {band.notes && <p className="whitespace-pre-wrap">{band.notes}</p>}
        <p className="text-base-content/70 text-sm">
          {band.rehearsalCount} rehearsals
          {band.lastRehearsedAt && <> · last on {band.lastRehearsedAt}</>}
        </p>
        {band.resources.length > 0 && (
          <ul className="flex flex-col gap-1">
            {band.resources.map(r => (
              <li key={r.id}>
                <a
                  className="link truncate"
                  href={r.url}
                  target="_blank"
                  rel="noreferrer noopener"
                >
                  {r.label || r.url}
                </a>
              </li>
            ))}
          </ul>
        )}
      </div>
    </section>
  );
}
```

- [ ] **Step 4: Wire it into `frontend/src/pages/SongPage.tsx`.** Make three changes (the file already destructures `song` from `useSong` and renders an identity form, practice card, links card, folders card, and a danger-zone delete):

a) Add the import: `import BandSection from '../components/songs/BandSection';`

b) Derive a flag after `song` is known to be loaded: `const isBandSong = song.band !== undefined;`

c) Disable the identity inputs for band songs and render the band section. On the title and artist `<input>` elements add `disabled={isBandSong}`. Render `{song.band && <BandSection band={song.band} />}` near the top of the returned layout (e.g. immediately after the identity card). Wrap the danger-zone delete section so it only renders for non-band songs: `{!isBandSong && ( …existing danger zone… )}`. (Status, notes, practice, and the personal Links card stay editable — they are the member's personal layer.)

The status `<select>` and notes `<textarea>` must remain enabled (personal layer), so only add `disabled={isBandSong}` to the title and artist inputs — not to status/notes.

- [ ] **Step 5: All four frontend checks green** (the existing personal-song SongPage tests still pass — those details have no `band` field, so `isBandSong` is false and nothing is disabled/hidden). Commit:

```bash
git add frontend
git commit -m "feat: personal-view band tag and read-only band section"
```

---

### Task 14: Docs + final verification

**Files:**
- Modify: `AGENTS.md`, `README.md`

- [ ] **Step 1: Update `AGENTS.md`** (verify each claim against the code):

1. Architecture: extend the `internal/handlers/` parenthetical with `band songs, band rehearsals/resources`; note `internal/repository/bandsongs.go` (conversion engine) and `subject.go` (the `subj` value backing user/band metadata methods).
2. Append to the Domain model section:

```markdown
Band songs are `owner_band_id`-owned and carry a full band metadata layer
(status, notes, resources, and a rehearsal log = band-keyed practice
events), edited only from the band view by Editors/Admins. Metadata
operations are written once against an internal `subj` value (a user XOR a
band) and exposed as user- and band-keyed methods. A band song appears in
every member's personal library with the member's OWN editable layer
(personal status/notes/resources/practice) plus a read-only band section;
its title/artist are band-owned and it cannot be deleted from the personal
view. When a member loses access to a band song (they leave or are removed,
the band deletes the song, or the band is deleted), any personal rows they
have on it are re-pointed onto a freshly created personal-copy song (the
band layer is not copied); untouched band songs simply leave their library.
All conversion runs in the same transaction as the removal. Band folders
arrive in the bands-folders plan.
```

- [ ] **Step 2: Update `README.md`** — change the Stack paragraph's bands clause to: "Bands with roles, invites, and shared songs (band metadata layer, personal-view interleaving, and a conversion engine that preserves members' work on leave/delete) are implemented. Planned per the design doc: band folders, installable PWA, single container on fly.io."

- [ ] **Step 3: `just check` → "all checks passed"; only the two doc files dirty. Commit:**

```bash
git add AGENTS.md README.md
git commit -m "docs: document band songs and the conversion engine"
```

---

## Done criteria

- `just check` green (all six gates).
- Through the dev loop: in a band, an Editor adds a song, sets the band status/notes, logs a rehearsal, adds a band link; a Viewer sees it all read-only. The band song appears in each member's personal library tagged with the band name; opening it shows the member's own editable status/notes/practice/resources plus a read-only "Band: <name>" section; the title/artist are locked and there is no delete. A member who has personally tracked a band song and then leaves (or is removed, or the band/song is deleted) keeps a personal copy with their own layer intact; a member who never touched it keeps nothing.
- All band-layer mutations are band-view-only and role-gated (Editor+ to write, Viewer to read); non-members get 404s.
- Next: Plan 4c (Band Folders) gets written against this codebase.

