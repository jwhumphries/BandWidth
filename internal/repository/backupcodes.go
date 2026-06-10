package repository

import (
	"time"

	"gorm.io/gorm"

	"github.com/jwhumphries/bandwidth/internal/auth"
	"github.com/jwhumphries/bandwidth/internal/model"
)

// ReplaceBackupCodes deletes any existing codes and stores hashes of the new set.
func (r *Repo) ReplaceBackupCodes(userID uint, codes []string) error {
	return r.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("user_id = ?", userID).
			Delete(&model.BackupCode{}).Error; err != nil {
			return err
		}
		for _, code := range codes {
			bc := &model.BackupCode{UserID: userID, CodeHash: auth.HashToken(code)}
			if err := tx.Create(bc).Error; err != nil {
				return err
			}
		}
		return nil
	})
}

// ConsumeBackupCode marks an unused matching code as used, reporting success.
func (r *Repo) ConsumeBackupCode(userID uint, code string) bool {
	res := r.db.Model(&model.BackupCode{}).
		Where("user_id = ? AND code_hash = ? AND used_at IS NULL",
			userID, auth.HashToken(code)).
		Update("used_at", time.Now())
	return res.Error == nil && res.RowsAffected == 1
}

// DeleteBackupCodes removes all of a user's backup codes.
func (r *Repo) DeleteBackupCodes(userID uint) error {
	return r.db.Where("user_id = ?", userID).Delete(&model.BackupCode{}).Error
}
