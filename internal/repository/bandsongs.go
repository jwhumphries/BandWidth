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
