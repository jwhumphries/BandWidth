package repository

import (
	"github.com/jwhumphries/bandwidth/internal/model"
)

// ResourcesForSongUser returns the user's resources for a song, in position order.
func (r *Repo) ResourcesForSongUser(songID, userID uint) ([]model.Resource, error) {
	resources := []model.Resource{}
	err := r.db.Where("song_id = ? AND user_id = ?", songID, userID).
		Order("position, id").Find(&resources).Error
	if err != nil {
		return nil, err
	}
	return resources, nil
}

// CreateResource appends a resource to the user's list for a song.
func (r *Repo) CreateResource(songID, userID uint, url, label string) (*model.Resource, error) {
	var maxPos int
	err := r.db.Model(&model.Resource{}).
		Select("COALESCE(MAX(position), 0)").
		Where("song_id = ? AND user_id = ?", songID, userID).
		Scan(&maxPos).Error
	if err != nil {
		return nil, err
	}
	res := &model.Resource{
		SongID: songID, UserID: &userID,
		URL: url, Label: label, Position: maxPos + 1,
	}
	if err := r.db.Create(res).Error; err != nil {
		return nil, err
	}
	return res, nil
}

// resourceForUser loads a resource only when it belongs to the user.
func (r *Repo) resourceForUser(resourceID, userID uint) (*model.Resource, error) {
	var res model.Resource
	err := r.db.Where("id = ? AND user_id = ?", resourceID, userID).
		First(&res).Error
	if err != nil {
		return nil, err
	}
	return &res, nil
}

// UpdateResource applies any provided fields to the user's resource.
func (r *Repo) UpdateResource(resourceID, userID uint, url, label *string) (*model.Resource, error) {
	res, err := r.resourceForUser(resourceID, userID)
	if err != nil {
		return nil, err
	}
	if url != nil {
		res.URL = *url
	}
	if label != nil {
		res.Label = *label
	}
	if err := r.db.Save(res).Error; err != nil {
		return nil, err
	}
	return res, nil
}

// DeleteResource removes the user's resource.
func (r *Repo) DeleteResource(resourceID, userID uint) error {
	res, err := r.resourceForUser(resourceID, userID)
	if err != nil {
		return err
	}
	return r.db.Delete(res).Error
}
