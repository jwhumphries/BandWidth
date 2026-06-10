package repository

import (
	"time"

	"github.com/jwhumphries/bandwidth/internal/auth"
	"github.com/jwhumphries/bandwidth/internal/model"
)

// CreateSession stores a new session and returns the raw token for the cookie.
func (r *Repo) CreateSession(userID uint) (string, error) {
	token := auth.NewToken()
	session := &model.Session{
		TokenHash: auth.HashToken(token),
		UserID:    userID,
		ExpiresAt: time.Now().Add(auth.SessionDuration),
	}
	if err := r.db.Create(session).Error; err != nil {
		return "", err
	}
	return token, nil
}

// SessionUser returns the user owning an unexpired session token.
func (r *Repo) SessionUser(token string) (*model.User, error) {
	var session model.Session
	err := r.db.
		Where("token_hash = ? AND expires_at > ?", auth.HashToken(token), time.Now()).
		First(&session).Error
	if err != nil {
		return nil, err
	}
	return r.UserByID(session.UserID)
}

// DeleteSession removes a session by raw token.
func (r *Repo) DeleteSession(token string) error {
	return r.db.Where("token_hash = ?", auth.HashToken(token)).
		Delete(&model.Session{}).Error
}

// DeleteUserSessions removes every session belonging to a user.
func (r *Repo) DeleteUserSessions(userID uint) error {
	return r.db.Where("user_id = ?", userID).Delete(&model.Session{}).Error
}
