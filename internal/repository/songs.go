package repository

import (
	"errors"
	"fmt"

	"gorm.io/gorm"

	"github.com/jwhumphries/bandwidth/internal/model"
)

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

// CreateSong inserts a user-owned song.
func (r *Repo) CreateSong(userID uint, title, artist string) (*model.Song, error) {
	song := &model.Song{Title: title, Artist: artist, OwnerUserID: &userID}
	if err := r.db.Create(song).Error; err != nil {
		return nil, err
	}
	return song, nil
}

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
	err := r.tryUpsertAnnotation(songID, s, status, notes)
	if IsDuplicate(err) {
		// Two first edits raced on the unique index; the row exists now, so
		// a single retry takes the update path.
		return r.tryUpsertAnnotation(songID, s, status, notes)
	}
	return err
}

func (r *Repo) tryUpsertAnnotation(songID uint, s subj, status *model.SongStatus, notes *string) error {
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
