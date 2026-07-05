package repository

import (
	"time"

	"github.com/jwhumphries/bandwidth/internal/model"
)

// PurgeExpired deletes rows that can never be used again: expired sessions,
// used or expired password resets, and dead band invites (expired, revoked,
// declined, or consumed direct invites). Live data is never touched.
func (r *Repo) PurgeExpired() error {
	now := time.Now()
	if err := r.db.Where("expires_at <= ?", now).
		Delete(&model.Session{}).Error; err != nil {
		return err
	}
	if err := r.db.Where("expires_at <= ? OR used_at IS NOT NULL", now).
		Delete(&model.PasswordReset{}).Error; err != nil {
		return err
	}
	// Dead invites are everything pendingInviteScope wouldn't return: expired,
	// revoked, or accepted (accepted_at is only ever set on single-use direct
	// invites; multi-use link invites stay live until expired or revoked).
	// Negating the shared condition keeps the two in sync if invite-liveness
	// rules change.
	return r.db.Not(pendingInviteCond, now).Delete(&model.BandInvite{}).Error
}
