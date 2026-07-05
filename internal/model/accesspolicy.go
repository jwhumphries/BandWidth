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
