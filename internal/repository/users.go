package repository

import (
	"strings"

	"github.com/jwhumphries/bandwidth/internal/model"
)

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
