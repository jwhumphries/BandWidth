package repository

import (
	"errors"
	"time"

	"gorm.io/gorm"

	"github.com/jwhumphries/bandwidth/internal/auth"
	"github.com/jwhumphries/bandwidth/internal/model"
)

// Invite lifetimes.
const (
	directInviteDuration = 14 * 24 * time.Hour
	linkInviteDuration   = 7 * 24 * time.Hour
)

// Invite errors the handler layer maps to HTTP statuses.
var (
	ErrAlreadyMember = errors.New("user is already a member")
	ErrInvitePending = errors.New("an invite for this user is already pending")
)

// PendingInvite is one row of a user's incoming-invite list.
type PendingInvite struct {
	ID       uint           `json:"id"`
	BandID   uint           `json:"bandId"`
	BandName string         `json:"bandName"`
	Role     model.BandRole `json:"role"`
}

// BandInviteInfo is one row of a band's outgoing-invite list (admin view).
type BandInviteInfo struct {
	ID              uint           `json:"id"`
	Role            model.BandRole `json:"role"`
	InvitedUsername *string        `json:"invitedUsername"`
	IsLink          bool           `json:"isLink"`
	ExpiresAt       time.Time      `json:"expiresAt"`
}

// pendingInviteScope filters to usable invites.
func pendingInviteScope(db *gorm.DB) *gorm.DB {
	return db.Where("accepted_at IS NULL AND revoked_at IS NULL AND expires_at > ?", time.Now())
}

// CreateDirectInvite invites an existing user to a band.
func (r *Repo) CreateDirectInvite(bandID, invitedUserID uint, role model.BandRole, createdBy uint) (*model.BandInvite, error) {
	if _, err := r.MemberRole(bandID, invitedUserID); err == nil {
		return nil, ErrAlreadyMember
	} else if !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, err
	}
	var n int64
	err := pendingInviteScope(r.db.Model(&model.BandInvite{})).
		Where("band_id = ? AND invited_user_id = ?", bandID, invitedUserID).
		Count(&n).Error
	if err != nil {
		return nil, err
	}
	if n > 0 {
		return nil, ErrInvitePending
	}
	invite := &model.BandInvite{
		BandID:        bandID,
		Role:          role,
		InvitedUserID: &invitedUserID,
		ExpiresAt:     time.Now().Add(directInviteDuration),
		CreatedBy:     createdBy,
	}
	if err := r.db.Create(invite).Error; err != nil {
		return nil, err
	}
	return invite, nil
}

// CreateLinkInvite creates a multi-use share link and returns its raw token.
func (r *Repo) CreateLinkInvite(bandID uint, role model.BandRole, createdBy uint) (*model.BandInvite, string, error) {
	token := auth.NewToken()
	hash := auth.HashToken(token)
	invite := &model.BandInvite{
		BandID:    bandID,
		Role:      role,
		TokenHash: &hash,
		ExpiresAt: time.Now().Add(linkInviteDuration),
		CreatedBy: createdBy,
	}
	if err := r.db.Create(invite).Error; err != nil {
		return nil, "", err
	}
	return invite, token, nil
}

// PendingInvitesForUser lists a user's incoming pending invites.
func (r *Repo) PendingInvitesForUser(userID uint) ([]PendingInvite, error) {
	pending := []PendingInvite{}
	err := pendingInviteScope(r.db.Table("band_invites")).
		Select("band_invites.id, band_invites.band_id, bands.name AS band_name, band_invites.role").
		Joins("JOIN bands ON bands.id = band_invites.band_id").
		Where("band_invites.invited_user_id = ?", userID).
		Order("band_invites.id").
		Scan(&pending).Error
	if err != nil {
		return nil, err
	}
	return pending, nil
}

// InvitesForBand lists a band's pending invites (admin view).
func (r *Repo) InvitesForBand(bandID uint) ([]BandInviteInfo, error) {
	invites := []BandInviteInfo{}
	err := pendingInviteScope(r.db.Table("band_invites")).
		Select(`band_invites.id, band_invites.role, users.username AS invited_username,
			CASE WHEN band_invites.token_hash IS NOT NULL THEN 1 ELSE 0 END AS is_link,
			band_invites.expires_at`).
		Joins("LEFT JOIN users ON users.id = band_invites.invited_user_id").
		Where("band_invites.band_id = ?", bandID).
		Order("band_invites.id").
		Scan(&invites).Error
	if err != nil {
		return nil, err
	}
	return invites, nil
}

// RevokeInvite revokes a band's invite (band-scoped).
func (r *Repo) RevokeInvite(inviteID, bandID uint) error {
	res := r.db.Model(&model.BandInvite{}).
		Where("id = ? AND band_id = ? AND revoked_at IS NULL", inviteID, bandID).
		Update("revoked_at", time.Now())
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected != 1 {
		return gorm.ErrRecordNotFound
	}
	return nil
}

// AcceptInvite joins the invited user to the band with the invite's role.
// Single-use; idempotent if the user is somehow already a member.
func (r *Repo) AcceptInvite(inviteID, userID uint) (uint, error) {
	var invite model.BandInvite
	err := pendingInviteScope(r.db).
		Where("id = ? AND invited_user_id = ?", inviteID, userID).
		First(&invite).Error
	if err != nil {
		return 0, err
	}
	err = r.db.Transaction(func(tx *gorm.DB) error {
		now := time.Now()
		if err := tx.Model(&model.BandInvite{}).Where("id = ?", invite.ID).
			Update("accepted_at", now).Error; err != nil {
			return err
		}
		// Check membership within the same transaction to avoid a second
		// connection acquiring the write lock on SQLite.
		var count int64
		if err := tx.Model(&model.BandMember{}).
			Where("band_id = ? AND user_id = ?", invite.BandID, userID).
			Count(&count).Error; err != nil {
			return err
		}
		if count > 0 {
			return nil // already a member; just consume the invite
		}
		return tx.Create(&model.BandMember{
			BandID: invite.BandID, UserID: userID, Role: invite.Role,
		}).Error
	})
	if err != nil {
		return 0, err
	}
	return invite.BandID, nil
}

// DeclineInvite marks a user's incoming invite revoked.
func (r *Repo) DeclineInvite(inviteID, userID uint) error {
	res := pendingInviteScope(r.db.Model(&model.BandInvite{})).
		Where("id = ? AND invited_user_id = ?", inviteID, userID).
		Update("revoked_at", time.Now())
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected != 1 {
		return gorm.ErrRecordNotFound
	}
	return nil
}

// JoinByLink adds the user to the link's band (multi-use, idempotent).
func (r *Repo) JoinByLink(token string, userID uint) (uint, error) {
	hash := auth.HashToken(token)
	var invite model.BandInvite
	err := pendingInviteScope(r.db).
		Where("token_hash = ?", hash).
		First(&invite).Error
	if err != nil {
		return 0, err
	}
	if _, err := r.MemberRole(invite.BandID, userID); err == nil {
		return invite.BandID, nil
	} else if !errors.Is(err, gorm.ErrRecordNotFound) {
		return 0, err
	}
	if err := r.AddMember(invite.BandID, userID, invite.Role); err != nil {
		if IsDuplicate(err) {
			return invite.BandID, nil
		}
		return 0, err
	}
	return invite.BandID, nil
}
