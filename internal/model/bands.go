package model

import "time"

// BandRole is a member's permission level within a band.
type BandRole string

// Band roles, in ascending privilege order.
const (
	RoleViewer BandRole = "viewer"
	RoleEditor BandRole = "editor"
	RoleAdmin  BandRole = "admin"
)

// Valid reports whether r is a known role.
func (r BandRole) Valid() bool {
	switch r {
	case RoleViewer, RoleEditor, RoleAdmin:
		return true
	}
	return false
}

func (r BandRole) rank() int {
	switch r {
	case RoleAdmin:
		return 3
	case RoleEditor:
		return 2
	case RoleViewer:
		return 1
	}
	return 0
}

// AtLeast reports whether r grants at least threshold's privileges.
func (r BandRole) AtLeast(threshold BandRole) bool {
	return r.rank() >= threshold.rank()
}

// Band is a group of users sharing songs. The creator is a permanent Admin.
type Band struct {
	ID        uint   `gorm:"primarykey"`
	Name      string `gorm:"not null"`
	CreatorID uint   `gorm:"not null"`
	CreatedAt time.Time
	UpdatedAt time.Time
}

// BandMember links a user to a band with a role.
type BandMember struct {
	ID        uint     `gorm:"primarykey"`
	BandID    uint     `gorm:"not null;uniqueIndex:idx_band_member"`
	UserID    uint     `gorm:"not null;uniqueIndex:idx_band_member"`
	Role      BandRole `gorm:"not null"`
	CreatedAt time.Time
}

// BandInvite is either a direct invite (InvitedUserID set, single-use) or a
// share link (TokenHash set, multi-use until revoked/expired). Declining a
// direct invite sets RevokedAt.
type BandInvite struct {
	ID            uint      `gorm:"primarykey"`
	BandID        uint      `gorm:"index;not null"`
	Role          BandRole  `gorm:"not null"`
	InvitedUserID *uint     `gorm:"index"`
	TokenHash     *string   `gorm:"uniqueIndex"`
	ExpiresAt     time.Time `gorm:"not null"`
	RevokedAt     *time.Time
	AcceptedAt    *time.Time
	CreatedBy     uint `gorm:"not null"`
	CreatedAt     time.Time
}
