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
