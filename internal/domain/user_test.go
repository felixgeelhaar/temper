package domain

import (
	"testing"
	"time"

	"github.com/google/uuid"
)

func TestSession_IsExpired(t *testing.T) {
	tests := []struct {
		name      string
		expiresAt time.Time
		want      bool
	}{
		{"expired", time.Now().Add(-time.Hour), true},
		{"not expired", time.Now().Add(time.Hour), false},
		{"just expired", time.Now().Add(-time.Millisecond), true},
		{"about to expire", time.Now().Add(time.Millisecond), false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			session := &Session{
				ID:        uuid.New(),
				ExpiresAt: tt.expiresAt,
			}
			if got := session.IsExpired(); got != tt.want {
				t.Errorf("IsExpired() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestSession_IsExpired_Revoked(t *testing.T) {
	session := &AuthSession{
		ID:        uuid.New(),
		ExpiresAt: time.Now().Add(time.Hour), // Not expired by time
	}

	// Session should not be expired initially
	if session.IsExpired() {
		t.Error("Session should not be expired before revocation")
	}

	// Revoke the session
	session.Revoke()

	// Now it should be expired
	if !session.IsExpired() {
		t.Error("Session should be expired after revocation")
	}

	// RevokedAt should be set
	if session.RevokedAt == nil {
		t.Error("RevokedAt should be set after revocation")
	}
}

func TestUser_SafeUser(t *testing.T) {
	now := time.Now()
	user := &User{
		ID:           uuid.New(),
		Email:        "test@example.com",
		Name:         "Test User",
		PasswordHash: "$2a$10$secrethash",
		CreatedAt:    now,
		UpdatedAt:    now,
	}

	safe := user.SafeUser()

	// Password hash should NOT be in safe representation
	if _, ok := safe["password_hash"]; ok {
		t.Error("SafeUser() should not include password_hash")
	}
	if _, ok := safe["passwordHash"]; ok {
		t.Error("SafeUser() should not include passwordHash")
	}

	// Other fields should be present
	if safe["email"] != "test@example.com" {
		t.Errorf("SafeUser() email = %v, want test@example.com", safe["email"])
	}
	if safe["name"] != "Test User" {
		t.Errorf("SafeUser() name = %v, want Test User", safe["name"])
	}
	if safe["id"] != user.ID {
		t.Errorf("SafeUser() id = %v, want %v", safe["id"], user.ID)
	}
}

func TestUser_Struct(t *testing.T) {
	now := time.Now()
	user := &User{
		ID:           uuid.New(),
		Email:        "test@example.com",
		Name:         "Test User",
		PasswordHash: "$2a$10$...",
		CreatedAt:    now,
		UpdatedAt:    now,
	}

	if user.Email != "test@example.com" {
		t.Errorf("Email = %q, want test@example.com", user.Email)
	}
	if user.Name != "Test User" {
		t.Errorf("Name = %q, want Test User", user.Name)
	}
	if user.PasswordHash != "$2a$10$..." {
		t.Errorf("PasswordHash = %q, want $2a$10$...", user.PasswordHash)
	}
}

func TestSession_Struct(t *testing.T) {
	now := time.Now()
	session := &Session{
		ID:        uuid.New(),
		UserID:    uuid.New(),
		Token:     "session-token-123",
		ExpiresAt: now.Add(24 * time.Hour),
		CreatedAt: now,
	}

	if session.Token != "session-token-123" {
		t.Errorf("Token = %q, want session-token-123", session.Token)
	}
	if session.ExpiresAt.Before(now) {
		t.Error("ExpiresAt should be in the future")
	}
}
