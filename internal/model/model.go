// Package model holds the persisted domain types.
package model

import "time"

// User is an account holder.
type User struct {
	ID              uint   `gorm:"primarykey"`
	Username        string `gorm:"uniqueIndex;not null"`
	Email           string `gorm:"uniqueIndex;not null"`
	PasswordHash    string `gorm:"not null"`
	TOTPSecret      string
	TOTPConfirmedAt *time.Time
	CreatedAt       time.Time
	UpdatedAt       time.Time
}

// TOTPEnabled reports whether 2FA is fully enrolled (secret set and verified).
func (u *User) TOTPEnabled() bool {
	return u.TOTPSecret != "" && u.TOTPConfirmedAt != nil
}

// Session is a logged-in browser session; the cookie holds the raw token,
// the row holds its SHA-256.
type Session struct {
	ID        uint      `gorm:"primarykey"`
	TokenHash string    `gorm:"uniqueIndex;not null"`
	UserID    uint      `gorm:"index;not null"`
	ExpiresAt time.Time `gorm:"not null"`
	CreatedAt time.Time
}

// BackupCode is a one-time 2FA recovery code (stored hashed).
type BackupCode struct {
	ID       uint   `gorm:"primarykey"`
	UserID   uint   `gorm:"index;not null"`
	CodeHash string `gorm:"not null"`
	UsedAt   *time.Time
}

// PasswordReset is a single-use, expiring reset token (stored hashed).
type PasswordReset struct {
	ID        uint      `gorm:"primarykey"`
	TokenHash string    `gorm:"uniqueIndex;not null"`
	UserID    uint      `gorm:"index;not null"`
	ExpiresAt time.Time `gorm:"not null"`
	UsedAt    *time.Time
	CreatedAt time.Time
}
