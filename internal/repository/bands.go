package repository

import (
	"gorm.io/gorm"

	"github.com/jwhumphries/bandwidth/internal/model"
)

// BandSummary is one row of a user's band list.
type BandSummary struct {
	ID          uint           `json:"id"`
	Name        string         `json:"name"`
	CreatorID   uint           `json:"creatorId"`
	Role        model.BandRole `json:"role"`
	MemberCount int            `json:"memberCount"`
}

// BandMemberInfo is one roster row.
type BandMemberInfo struct {
	UserID   uint           `json:"userId"`
	Username string         `json:"username"`
	Role     model.BandRole `json:"role"`
}

// CreateBand creates a band with the creator as permanent Admin.
func (r *Repo) CreateBand(creatorID uint, name string) (*model.Band, error) {
	band := &model.Band{Name: name, CreatorID: creatorID}
	err := r.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(band).Error; err != nil {
			return err
		}
		member := &model.BandMember{
			BandID: band.ID, UserID: creatorID, Role: model.RoleAdmin,
		}
		return tx.Create(member).Error
	})
	if err != nil {
		return nil, err
	}
	return band, nil
}

// AddMember adds a user to a band with a role.
func (r *Repo) AddMember(bandID, userID uint, role model.BandRole) error {
	return r.db.Create(&model.BandMember{
		BandID: bandID, UserID: userID, Role: role,
	}).Error
}

// BandsForUser lists the user's bands with their role and the member count.
func (r *Repo) BandsForUser(userID uint) ([]BandSummary, error) {
	summaries := []BandSummary{}
	err := r.db.Table("band_members").
		Select(`bands.id, bands.name, bands.creator_id, band_members.role,
			(SELECT COUNT(*) FROM band_members bm WHERE bm.band_id = bands.id) AS member_count`).
		Joins("JOIN bands ON bands.id = band_members.band_id").
		Where("band_members.user_id = ?", userID).
		Order("bands.name COLLATE NOCASE, bands.id").
		Scan(&summaries).Error
	if err != nil {
		return nil, err
	}
	return summaries, nil
}

// BandByID loads a band.
func (r *Repo) BandByID(bandID uint) (*model.Band, error) {
	var band model.Band
	if err := r.db.First(&band, bandID).Error; err != nil {
		return nil, err
	}
	return &band, nil
}

// MemberRole returns the user's role in the band, or
// gorm.ErrRecordNotFound for non-members.
func (r *Repo) MemberRole(bandID, userID uint) (model.BandRole, error) {
	var member model.BandMember
	err := r.db.Where("band_id = ? AND user_id = ?", bandID, userID).
		First(&member).Error
	if err != nil {
		return "", err
	}
	return member.Role, nil
}

// MembersForBand returns the roster with usernames, oldest member first.
func (r *Repo) MembersForBand(bandID uint) ([]BandMemberInfo, error) {
	members := []BandMemberInfo{}
	err := r.db.Table("band_members").
		Select("band_members.user_id, users.username, band_members.role").
		Joins("JOIN users ON users.id = band_members.user_id").
		Where("band_members.band_id = ?", bandID).
		Order("band_members.created_at, band_members.id").
		Scan(&members).Error
	if err != nil {
		return nil, err
	}
	return members, nil
}

// RenameBand updates the band's name.
func (r *Repo) RenameBand(bandID uint, name string) error {
	return r.db.Model(&model.Band{}).Where("id = ?", bandID).
		Update("name", name).Error
}

// SetMemberRole changes a member's role. Unknown members error.
func (r *Repo) SetMemberRole(bandID, userID uint, role model.BandRole) error {
	res := r.db.Model(&model.BandMember{}).
		Where("band_id = ? AND user_id = ?", bandID, userID).
		Update("role", role)
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected != 1 {
		return gorm.ErrRecordNotFound
	}
	return nil
}

// RemoveMember removes a user from a band. Before the membership is dropped,
// each band song the member personally touched is converted to a personal
// copy (preserving their notes/status/resources/practice); the band's own
// songs are untouched.
func (r *Repo) RemoveMember(bandID, userID uint) error {
	return r.db.Transaction(func(tx *gorm.DB) error {
		var member model.BandMember
		if err := tx.Where("band_id = ? AND user_id = ?", bandID, userID).
			First(&member).Error; err != nil {
			return err // gorm.ErrRecordNotFound for non-members
		}
		var songs []model.Song
		if err := tx.Where("owner_band_id = ?", bandID).Find(&songs).Error; err != nil {
			return err
		}
		for i := range songs {
			if err := convertBandSongForUser(tx, &songs[i], userID); err != nil {
				return err
			}
		}
		return tx.Delete(&member).Error
	})
}

// DeleteBand removes the band, its songs, memberships, and invites. Each
// member's personal work on the band's songs is converted to personal copies
// first. Atomic.
func (r *Repo) DeleteBand(bandID uint) error {
	return r.db.Transaction(func(tx *gorm.DB) error {
		return deleteBandTx(tx, bandID)
	})
}

// deleteBandTx runs DeleteBand's work inside an existing transaction, so
// DeleteUser can cascade a created band without nesting transactions.
func deleteBandTx(tx *gorm.DB, bandID uint) error {
	var songs []model.Song
	if err := tx.Where("owner_band_id = ?", bandID).Find(&songs).Error; err != nil {
		return err
	}
	var members []model.BandMember
	if err := tx.Where("band_id = ?", bandID).Find(&members).Error; err != nil {
		return err
	}
	for i := range songs {
		for _, m := range members {
			if err := convertBandSongForUser(tx, &songs[i], m.UserID); err != nil {
				return err
			}
		}
		if err := deleteBandSongRows(tx, songs[i].ID); err != nil {
			return err
		}
	}
	if err := tx.Where("band_id = ?", bandID).Delete(&model.BandMember{}).Error; err != nil {
		return err
	}
	if err := tx.Where("band_id = ?", bandID).Delete(&model.BandInvite{}).Error; err != nil {
		return err
	}
	var folders []model.Folder
	if err := tx.Where("owner_band_id = ?", bandID).Find(&folders).Error; err != nil {
		return err
	}
	for _, f := range folders {
		if err := tx.Where("folder_id = ?", f.ID).Delete(&model.FolderEntry{}).Error; err != nil {
			return err
		}
	}
	if err := tx.Where("owner_band_id = ?", bandID).Delete(&model.Folder{}).Error; err != nil {
		return err
	}
	return tx.Delete(&model.Band{}, bandID).Error
}
