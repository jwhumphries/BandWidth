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
	return r.createFolder(userSubj(userID), name)
}

// CreateBandFolder appends a folder to the band's list.
func (r *Repo) CreateBandFolder(bandID uint, name string) (*model.Folder, error) {
	return r.createFolder(bandSubj(bandID), name)
}

func (r *Repo) createFolder(s subj, name string) (*model.Folder, error) {
	cond, id := s.ownerScope()
	var maxPos int
	err := r.db.Model(&model.Folder{}).
		Select("COALESCE(MAX(position), 0)").
		Where(cond, id).Scan(&maxPos).Error
	if err != nil {
		return nil, err
	}
	folder := &model.Folder{Name: name, Position: maxPos + 1, OwnerUserID: s.userID, OwnerBandID: s.bandID}
	if err := r.db.Create(folder).Error; err != nil {
		return nil, err
	}
	return folder, nil
}

// folderForOwner loads a folder only when this subject owns it.
func (r *Repo) folderForOwner(folderID uint, s subj) (*model.Folder, error) {
	var folder model.Folder
	cond, id := s.ownerScope()
	err := r.db.Where("id = ? AND "+cond, folderID, id).First(&folder).Error
	if err != nil {
		return nil, err
	}
	return &folder, nil
}

// FoldersForUser returns the user's folders in position order with song IDs.
func (r *Repo) FoldersForUser(userID uint) ([]FolderWithSongs, error) {
	return r.foldersForOwner(userSubj(userID))
}

// FoldersForBand returns the band's folders in position order with song IDs.
func (r *Repo) FoldersForBand(bandID uint) ([]FolderWithSongs, error) {
	return r.foldersForOwner(bandSubj(bandID))
}

func (r *Repo) foldersForOwner(s subj) ([]FolderWithSongs, error) {
	cond, id := s.ownerScope()
	var folders []model.Folder
	err := r.db.Where(cond, id).Order("position, id").Find(&folders).Error
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
	return r.renameFolder(folderID, userSubj(userID), name)
}

// RenameBandFolder renames the band's folder.
func (r *Repo) RenameBandFolder(folderID, bandID uint, name string) error {
	return r.renameFolder(folderID, bandSubj(bandID), name)
}

func (r *Repo) renameFolder(folderID uint, s subj, name string) error {
	folder, err := r.folderForOwner(folderID, s)
	if err != nil {
		return err
	}
	folder.Name = name
	return r.db.Save(folder).Error
}

// DeleteFolder removes the user's folder and its entries; songs are untouched.
func (r *Repo) DeleteFolder(folderID, userID uint) error {
	return r.deleteFolder(folderID, userSubj(userID))
}

// DeleteBandFolder removes the band's folder and its entries; songs untouched.
func (r *Repo) DeleteBandFolder(folderID, bandID uint) error {
	return r.deleteFolder(folderID, bandSubj(bandID))
}

func (r *Repo) deleteFolder(folderID uint, s subj) error {
	folder, err := r.folderForOwner(folderID, s)
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

// ReorderFolders applies the given order to the user's folders.
func (r *Repo) ReorderFolders(userID uint, folderIDs []uint) error {
	return r.reorderFolders(userSubj(userID), folderIDs)
}

// ReorderBandFolders applies the given order to the band's folders.
func (r *Repo) ReorderBandFolders(bandID uint, folderIDs []uint) error {
	return r.reorderFolders(bandSubj(bandID), folderIDs)
}

// reorderFolders applies a new order. The request must name every one of the
// subject's folders exactly once — a partial or duplicated list would leave
// stale or colliding positions behind.
func (r *Repo) reorderFolders(s subj, folderIDs []uint) error {
	seen := make(map[uint]struct{}, len(folderIDs))
	for _, fid := range folderIDs {
		if _, dup := seen[fid]; dup {
			return fmt.Errorf("folder %d listed twice: %w", fid, gorm.ErrRecordNotFound)
		}
		seen[fid] = struct{}{}
	}
	cond, id := s.ownerScope()
	return r.db.Transaction(func(tx *gorm.DB) error {
		var total int64
		if err := tx.Model(&model.Folder{}).Where(cond, id).Count(&total).Error; err != nil {
			return err
		}
		if total != int64(len(folderIDs)) {
			return fmt.Errorf("order must name all %d folders: %w", total, gorm.ErrRecordNotFound)
		}
		for i, fid := range folderIDs {
			res := tx.Model(&model.Folder{}).
				Where("id = ? AND "+cond, fid, id).
				Update("position", i+1)
			if res.Error != nil {
				return res.Error
			}
			if res.RowsAffected != 1 {
				return fmt.Errorf("folder %d not found: %w", fid, gorm.ErrRecordNotFound)
			}
		}
		return nil
	})
}

// SetFolderEntries replaces a user folder's membership and order. Every song
// must be visible to the user (owned or shared by a band they belong to).
func (r *Repo) SetFolderEntries(folderID, userID uint, songIDs []uint) error {
	return r.setFolderEntries(folderID, userSubj(userID), songIDs)
}

// SetBandFolderEntries replaces a band folder's membership and order. Every
// song must be owned by this band.
func (r *Repo) SetBandFolderEntries(folderID, bandID uint, songIDs []uint) error {
	return r.setFolderEntries(folderID, bandSubj(bandID), songIDs)
}

func (r *Repo) setFolderEntries(folderID uint, s subj, songIDs []uint) error {
	if _, err := r.folderForOwner(folderID, s); err != nil {
		return err
	}
	if len(songIDs) > 0 {
		ok, err := r.songsSelectableFor(s, songIDs)
		if err != nil {
			return err
		}
		if !ok {
			return fmt.Errorf("one or more songs not found: %w", gorm.ErrRecordNotFound)
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

// songsSelectableFor reports whether every song may be placed in this
// subject's folder: any visible song for a user, only the band's own songs for
// a band.
func (r *Repo) songsSelectableFor(s subj, songIDs []uint) (bool, error) {
	var n int64
	q := r.db.Model(&model.Song{})
	if s.userID != nil {
		q = q.Where(`id IN ? AND (owner_user_id = ? OR owner_band_id IN
			(SELECT band_id FROM band_members WHERE user_id = ?))`,
			songIDs, *s.userID, *s.userID)
	} else {
		q = q.Where("id IN ? AND owner_band_id = ?", songIDs, *s.bandID)
	}
	if err := q.Count(&n).Error; err != nil {
		return false, err
	}
	return n == int64(len(songIDs)), nil
}
