package repository

import (
	"errors"
	"time"

	"gorm.io/gorm"

	"github.com/jwhumphries/bandwidth/internal/model"
)

// AdminUserSummary is one row of the admin user list.
type AdminUserSummary struct {
	ID        uint      `json:"id"`
	Username  string    `json:"username"`
	Email     string    `json:"email"`
	CreatedAt time.Time `json:"createdAt"`
}

// AdminBandSummary is one row of the admin band list.
type AdminBandSummary struct {
	ID              uint   `json:"id"`
	Name            string `json:"name"`
	CreatorUsername string `json:"creatorUsername"`
	MemberCount     int    `json:"memberCount"`
}

// AllowedEmailInfo is one row of the signup allow-list.
type AllowedEmailInfo struct {
	ID        uint      `json:"id"`
	Email     string    `json:"email"`
	CreatedAt time.Time `json:"createdAt"`
}

// AllUsers lists every account for the admin panel.
func (r *Repo) AllUsers() ([]AdminUserSummary, error) {
	users := []AdminUserSummary{}
	err := r.db.Model(&model.User{}).
		Select("id, username, email, created_at").
		Order("username COLLATE NOCASE, id").
		Scan(&users).Error
	if err != nil {
		return nil, err
	}
	return users, nil
}

// AllBands lists every band for the admin panel.
func (r *Repo) AllBands() ([]AdminBandSummary, error) {
	bands := []AdminBandSummary{}
	err := r.db.Table("bands").
		Select(`bands.id, bands.name, users.username AS creator_username,
			(SELECT COUNT(*) FROM band_members bm WHERE bm.band_id = bands.id) AS member_count`).
		Joins("JOIN users ON users.id = bands.creator_id").
		Order("bands.name COLLATE NOCASE, bands.id").
		Scan(&bands).Error
	if err != nil {
		return nil, err
	}
	return bands, nil
}

// accessPolicy returns the singleton settings row, creating it (disabled)
// if this is the first access.
func (r *Repo) accessPolicy() (*model.AccessPolicy, error) {
	var policy model.AccessPolicy
	err := r.db.First(&policy).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		policy = model.AccessPolicy{Enabled: false}
		if err := r.db.Create(&policy).Error; err != nil {
			return nil, err
		}
		return &policy, nil
	}
	if err != nil {
		return nil, err
	}
	return &policy, nil
}

// AccessPolicyEnabled reports whether signup is currently gated.
func (r *Repo) AccessPolicyEnabled() (bool, error) {
	policy, err := r.accessPolicy()
	if err != nil {
		return false, err
	}
	return policy.Enabled, nil
}

// SetAccessPolicyEnabled toggles signup gating.
func (r *Repo) SetAccessPolicyEnabled(enabled bool) error {
	policy, err := r.accessPolicy()
	if err != nil {
		return err
	}
	policy.Enabled = enabled
	return r.db.Save(policy).Error
}

// EmailAllowed reports whether email is on the signup allow-list.
func (r *Repo) EmailAllowed(email string) (bool, error) {
	email = normalizeEmail(email)
	var n int64
	if err := r.db.Model(&model.AllowedEmail{}).Where("email = ?", email).Count(&n).Error; err != nil {
		return false, err
	}
	return n > 0, nil
}

// AllowedEmails lists the signup allow-list.
func (r *Repo) AllowedEmails() ([]AllowedEmailInfo, error) {
	emails := []AllowedEmailInfo{}
	err := r.db.Model(&model.AllowedEmail{}).
		Select("id, email, created_at").
		Order("email").
		Scan(&emails).Error
	if err != nil {
		return nil, err
	}
	return emails, nil
}

// AddAllowedEmail adds an email to the signup allow-list.
func (r *Repo) AddAllowedEmail(email string, addedBy uint) (*model.AllowedEmail, error) {
	row := &model.AllowedEmail{Email: normalizeEmail(email), CreatedBy: addedBy}
	if err := r.db.Create(row).Error; err != nil {
		return nil, err
	}
	return row, nil
}

// RemoveAllowedEmail removes an allow-list entry.
func (r *Repo) RemoveAllowedEmail(id uint) error {
	res := r.db.Delete(&model.AllowedEmail{}, id)
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected != 1 {
		return gorm.ErrRecordNotFound
	}
	return nil
}
