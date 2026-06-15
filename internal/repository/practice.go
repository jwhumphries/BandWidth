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
