package repository

import (
	"strings"

	"gorm.io/gorm"

	"github.com/jwhumphries/bandwidth/internal/model"
)

// normalizeEmail trims and lowercases an email so allow-list matching
// doesn't depend on every caller normalizing consistently.
func normalizeEmail(email string) string {
	return strings.ToLower(strings.TrimSpace(email))
}

// IsDuplicate reports whether err is a unique-constraint violation.
//
// It matches SQLite's "UNIQUE constraint failed" message text because GORM's
// SQLite drivers expose no structured constraint error codes; revisit if the
// driver or SQLite changes its error format.
func IsDuplicate(err error) bool {
	return err != nil && strings.Contains(err.Error(), "UNIQUE constraint failed")
}

// CreateUser inserts a new user.
func (r *Repo) CreateUser(username, email, passwordHash string) (*model.User, error) {
	user := &model.User{Username: username, Email: email, PasswordHash: passwordHash}
	if err := r.db.Create(user).Error; err != nil {
		return nil, err
	}
	return user, nil
}

// UserByLogin finds a user by username or email.
func (r *Repo) UserByLogin(login string) (*model.User, error) {
	var user model.User
	err := r.db.Where("username = ? OR email = ?", login, login).First(&user).Error
	if err != nil {
		return nil, err
	}
	return &user, nil
}

// UserByID finds a user by primary key.
func (r *Repo) UserByID(id uint) (*model.User, error) {
	var user model.User
	if err := r.db.First(&user, id).Error; err != nil {
		return nil, err
	}
	return &user, nil
}

// SaveUser persists changes to an existing user.
func (r *Repo) SaveUser(user *model.User) error {
	return r.db.Save(user).Error
}

// DeleteUser removes a user and everything they solely own: sessions, 2FA
// backup codes, pending password resets, personal (non-band) songs/folders,
// band memberships, pending invites addressed to them, and any band they
// created (cascaded, same as DeleteBand). Their personal layer (annotation/
// resource/practice rows) on any other band song is also cleared, since
// there is no longer a user to own a converted copy. Atomic.
func (r *Repo) DeleteUser(userID uint) error {
	return r.db.Transaction(func(tx *gorm.DB) error {
		var createdBands []model.Band
		if err := tx.Where("creator_id = ?", userID).Find(&createdBands).Error; err != nil {
			return err
		}
		for _, b := range createdBands {
			if err := deleteBandTx(tx, b.ID); err != nil {
				return err
			}
		}
		for _, m := range []any{&model.SongAnnotation{}, &model.Resource{}, &model.PracticeEvent{}} {
			if err := tx.Where("user_id = ?", userID).Delete(m).Error; err != nil {
				return err
			}
		}
		var personalSongs []model.Song
		if err := tx.Where("owner_user_id = ?", userID).Find(&personalSongs).Error; err != nil {
			return err
		}
		for _, s := range personalSongs {
			if err := deleteSongRowsTx(tx, s.ID); err != nil {
				return err
			}
		}
		if err := deleteOwnedFoldersTx(tx, userSubj(userID)); err != nil {
			return err
		}
		if err := tx.Where("user_id = ?", userID).Delete(&model.BandMember{}).Error; err != nil {
			return err
		}
		if err := tx.Where("invited_user_id = ?", userID).Delete(&model.BandInvite{}).Error; err != nil {
			return err
		}
		if err := tx.Where("created_by = ?", userID).Delete(&model.BandInvite{}).Error; err != nil {
			return err
		}
		if err := tx.Where("user_id = ?", userID).Delete(&model.Session{}).Error; err != nil {
			return err
		}
		if err := tx.Where("user_id = ?", userID).Delete(&model.BackupCode{}).Error; err != nil {
			return err
		}
		if err := tx.Where("user_id = ?", userID).Delete(&model.PasswordReset{}).Error; err != nil {
			return err
		}
		return tx.Delete(&model.User{}, userID).Error
	})
}
