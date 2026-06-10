package repository

import (
	"time"

	"github.com/jwhumphries/bandwidth/internal/auth"
	"github.com/jwhumphries/bandwidth/internal/model"
)

const passwordResetDuration = time.Hour

// CreatePasswordReset stores a reset token and returns its raw value.
func (r *Repo) CreatePasswordReset(userID uint) (string, error) {
	token := auth.NewToken()
	reset := &model.PasswordReset{
		TokenHash: auth.HashToken(token),
		UserID:    userID,
		ExpiresAt: time.Now().Add(passwordResetDuration),
	}
	if err := r.db.Create(reset).Error; err != nil {
		return "", err
	}
	return token, nil
}

// ConsumePasswordReset marks a valid token used and returns its user ID.
func (r *Repo) ConsumePasswordReset(token string) (uint, error) {
	var reset model.PasswordReset
	err := r.db.
		Where("token_hash = ? AND expires_at > ? AND used_at IS NULL",
			auth.HashToken(token), time.Now()).
		First(&reset).Error
	if err != nil {
		return 0, err
	}
	now := time.Now()
	reset.UsedAt = &now
	if err := r.db.Save(&reset).Error; err != nil {
		return 0, err
	}
	return reset.UserID, nil
}
