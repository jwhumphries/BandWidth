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
// A missing row reads as StatusNotLearned with empty notes.
type SongAnnotation struct {
	ID        uint       `gorm:"primarykey"`
	SongID    uint       `gorm:"uniqueIndex:idx_annotation_subject;not null"`
	UserID    *uint      `gorm:"uniqueIndex:idx_annotation_subject"`
	BandID    *uint      `gorm:"uniqueIndex:idx_annotation_subject"`
	Status    SongStatus `gorm:"not null;default:not_learned"`
	Notes     string
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
// subject. Date is YYYY-MM-DD; the unique index dedupes per day.
type PracticeEvent struct {
	ID     uint   `gorm:"primarykey"`
	SongID uint   `gorm:"uniqueIndex:idx_practice_day;not null"`
	UserID *uint  `gorm:"uniqueIndex:idx_practice_day"`
	BandID *uint  `gorm:"uniqueIndex:idx_practice_day"`
	Date   string `gorm:"uniqueIndex:idx_practice_day;not null"`
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
