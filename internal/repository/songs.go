package repository

import (
	"errors"
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
	case errors.Is(err, gorm.ErrRecordNotFound):
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
