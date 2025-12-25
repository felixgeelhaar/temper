package domain

import (
	"time"

	"github.com/google/uuid"
)

// User represents a registered user
type User struct {
	ID           uuid.UUID
	Email        string
	Name         string
	PasswordHash string
	CreatedAt    time.Time
	UpdatedAt    time.Time
}

// Session represents an authenticated session
type Session struct {
	ID        uuid.UUID
	UserID    uuid.UUID
	Token     string
	ExpiresAt time.Time
	CreatedAt time.Time
}

// IsExpired checks if the session has expired
func (s *Session) IsExpired() bool {
	return time.Now().After(s.ExpiresAt)
}
