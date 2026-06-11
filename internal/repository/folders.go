package repository

import (
	"fmt"

	"gorm.io/gorm"

	"github.com/jwhumphries/bandwidth/internal/model"
)

// FolderWithSongs is a folder plus its ordered song IDs.
type FolderWithSongs struct {
	ID       uint   `json:"id"`
	Name     string `json:"name"`
	Position int    `json:"position"`
	SongIDs  []uint `json:"songIds"`
}

// CreateFolder appends a folder to the user's list.
func (r *Repo) CreateFolder(userID uint, name string) (*model.Folder, error) {
	var maxPos int
	err := r.db.Model(&model.Folder{}).
		Select("COALESCE(MAX(position), 0)").
		Where("owner_user_id = ?", userID).
		Scan(&maxPos).Error
	if err != nil {
		return nil, err
	}
	folder := &model.Folder{Name: name, Position: maxPos + 1, OwnerUserID: &userID}
	if err := r.db.Create(folder).Error; err != nil {
		return nil, err
	}
	return folder, nil
}

// folderForUser loads a folder only when the user owns it.
func (r *Repo) folderForUser(folderID, userID uint) (*model.Folder, error) {
	var folder model.Folder
	err := r.db.Where("id = ? AND owner_user_id = ?", folderID, userID).
		First(&folder).Error
	if err != nil {
		return nil, err
	}
	return &folder, nil
}

// FoldersForUser returns the user's folders in position order, each with
// its ordered song IDs.
func (r *Repo) FoldersForUser(userID uint) ([]FolderWithSongs, error) {
	var folders []model.Folder
	err := r.db.Where("owner_user_id = ?", userID).
		Order("position, id").Find(&folders).Error
	if err != nil {
		return nil, err
	}
	result := make([]FolderWithSongs, 0, len(folders))
	for _, f := range folders {
		var entries []model.FolderEntry
		if err := r.db.Where("folder_id = ?", f.ID).
			Order("position, id").Find(&entries).Error; err != nil {
			return nil, err
		}
		songIDs := make([]uint, 0, len(entries))
		for _, e := range entries {
			songIDs = append(songIDs, e.SongID)
		}
		result = append(result, FolderWithSongs{
			ID: f.ID, Name: f.Name, Position: f.Position, SongIDs: songIDs,
		})
	}
	return result, nil
}

// RenameFolder renames the user's folder.
func (r *Repo) RenameFolder(folderID, userID uint, name string) error {
	folder, err := r.folderForUser(folderID, userID)
	if err != nil {
		return err
	}
	folder.Name = name
	return r.db.Save(folder).Error
}

// DeleteFolder removes the user's folder and its entries; songs are untouched.
func (r *Repo) DeleteFolder(folderID, userID uint) error {
	folder, err := r.folderForUser(folderID, userID)
	if err != nil {
		return err
	}
	return r.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("folder_id = ?", folder.ID).
			Delete(&model.FolderEntry{}).Error; err != nil {
			return err
		}
		return tx.Delete(folder).Error
	})
}

// ReorderFolders applies the given order to the user's folders. IDs not
// owned by the user are rejected.
func (r *Repo) ReorderFolders(userID uint, folderIDs []uint) error {
	return r.db.Transaction(func(tx *gorm.DB) error {
		for i, id := range folderIDs {
			res := tx.Model(&model.Folder{}).
				Where("id = ? AND owner_user_id = ?", id, userID).
				Update("position", i+1)
			if res.Error != nil {
				return res.Error
			}
			if res.RowsAffected != 1 {
				return fmt.Errorf("folder %d not found", id)
			}
		}
		return nil
	})
}

// SetFolderEntries replaces the folder's membership and order with songIDs.
// Every song must be visible to the user.
func (r *Repo) SetFolderEntries(folderID, userID uint, songIDs []uint) error {
	if _, err := r.folderForUser(folderID, userID); err != nil {
		return err
	}
	if len(songIDs) > 0 {
		var visible int64
		err := r.db.Model(&model.Song{}).
			Where("id IN ? AND owner_user_id = ?", songIDs, userID).
			Count(&visible).Error
		if err != nil {
			return err
		}
		if visible != int64(len(songIDs)) {
			return fmt.Errorf("one or more songs not found")
		}
	}
	return r.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("folder_id = ?", folderID).
			Delete(&model.FolderEntry{}).Error; err != nil {
			return err
		}
		for i, songID := range songIDs {
			entry := &model.FolderEntry{FolderID: folderID, SongID: songID, Position: i + 1}
			if err := tx.Create(entry).Error; err != nil {
				return err
			}
		}
		return nil
	})
}
