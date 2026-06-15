package repository

import (
	"github.com/jwhumphries/bandwidth/internal/model"
)

func (r *Repo) resourcesForSong(songID uint, s subj) ([]model.Resource, error) {
	resources := []model.Resource{}
	cond, id := s.scope()
	err := r.db.Where("song_id = ? AND "+cond, songID, id).
		Order("position, id").Find(&resources).Error
	if err != nil {
		return nil, err
	}
	return resources, nil
}

func (r *Repo) createResource(songID uint, s subj, url, label string) (*model.Resource, error) {
	cond, id := s.scope()
	var maxPos int
	err := r.db.Model(&model.Resource{}).
		Select("COALESCE(MAX(position), 0)").
		Where("song_id = ? AND "+cond, songID, id).
		Scan(&maxPos).Error
	if err != nil {
		return nil, err
	}
	res := &model.Resource{
		SongID: songID, UserID: s.userID, BandID: s.bandID,
		URL: url, Label: label, Position: maxPos + 1,
	}
	if err := r.db.Create(res).Error; err != nil {
		return nil, err
	}
	return res, nil
}

func (r *Repo) resourceForSubject(resourceID, songID uint, s subj) (*model.Resource, error) {
	var res model.Resource
	cond, id := s.scope()
	err := r.db.Where("id = ? AND song_id = ? AND "+cond, resourceID, songID, id).
		First(&res).Error
	if err != nil {
		return nil, err
	}
	return &res, nil
}

func (r *Repo) updateResource(resourceID, songID uint, s subj, url, label *string) (*model.Resource, error) {
	res, err := r.resourceForSubject(resourceID, songID, s)
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

func (r *Repo) deleteResource(resourceID, songID uint, s subj) error {
	res, err := r.resourceForSubject(resourceID, songID, s)
	if err != nil {
		return err
	}
	return r.db.Delete(res).Error
}

// ResourcesForSongUser returns the user's resources for a song, in position order.
func (r *Repo) ResourcesForSongUser(songID, userID uint) ([]model.Resource, error) {
	return r.resourcesForSong(songID, userSubj(userID))
}

// CreateResource appends a resource to the user's list for a song.
func (r *Repo) CreateResource(songID, userID uint, url, label string) (*model.Resource, error) {
	return r.createResource(songID, userSubj(userID), url, label)
}

// UpdateResource applies any provided fields to the user's resource.
func (r *Repo) UpdateResource(resourceID, songID, userID uint, url, label *string) (*model.Resource, error) {
	return r.updateResource(resourceID, songID, userSubj(userID), url, label)
}

// DeleteResource removes the user's resource.
func (r *Repo) DeleteResource(resourceID, songID, userID uint) error {
	return r.deleteResource(resourceID, songID, userSubj(userID))
}

// ResourcesForSongBand returns the band's resources for a song, in position order.
func (r *Repo) ResourcesForSongBand(songID, bandID uint) ([]model.Resource, error) {
	return r.resourcesForSong(songID, bandSubj(bandID))
}

// CreateBandResource appends a resource to the band's list for a song.
func (r *Repo) CreateBandResource(songID, bandID uint, url, label string) (*model.Resource, error) {
	return r.createResource(songID, bandSubj(bandID), url, label)
}

// UpdateBandResource applies any provided fields to the band's resource.
func (r *Repo) UpdateBandResource(resourceID, songID, bandID uint, url, label *string) (*model.Resource, error) {
	return r.updateResource(resourceID, songID, bandSubj(bandID), url, label)
}

// DeleteBandResource removes the band's resource.
func (r *Repo) DeleteBandResource(resourceID, songID, bandID uint) error {
	return r.deleteResource(resourceID, songID, bandSubj(bandID))
}
