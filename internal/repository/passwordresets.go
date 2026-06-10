package repository

import (
	"time"

	"github.com/jwhumphries/bandwidth/internal/auth"
	"github.com/jwhumphries/bandwidth/internal/model"
	"gorm.io/gorm"
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

// ConsumePasswordReset atomically marks a valid token used and returns its
// user ID. The guarded UPDATE ensures a token can be consumed only once,
// even under concurrent requests.
func (r *Repo) ConsumePasswordReset(token string) (uint, error) {
	hash := auth.HashToken(token)
	res := r.db.Model(&model.PasswordReset{}).
		Where("token_hash = ? AND expires_at > ? AND used_at IS NULL",
			hash, time.Now()).
		Update("used_at", time.Now())
	if res.Error != nil {
		return 0, res.Error
	}
	if res.RowsAffected != 1 {
		return 0, gorm.ErrRecordNotFound
	}

	var reset model.PasswordReset
	if err := r.db.Where("token_hash = ?", hash).First(&reset).Error; err != nil {
		return 0, err
	}
	return reset.UserID, nil
}
