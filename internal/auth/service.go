package auth

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"time"

	"github.com/felixgeelhaar/temper/internal/domain"
	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"
)

var (
	ErrInvalidCredentials = errors.New("invalid email or password")
	ErrEmailExists        = errors.New("email already registered")
	ErrSessionExpired     = errors.New("session expired")
	ErrSessionNotFound    = errors.New("session not found")
)

// Repository defines the interface for auth data access
type Repository interface {
	CreateUser(ctx context.Context, user *domain.User) error
	GetUserByEmail(ctx context.Context, email string) (*domain.User, error)
	GetUserByID(ctx context.Context, id uuid.UUID) (*domain.User, error)

	CreateSession(ctx context.Context, session *domain.Session) error
	GetSessionByToken(ctx context.Context, token string) (*domain.Session, error)
	DeleteSession(ctx context.Context, id uuid.UUID) error
	DeleteUserSessions(ctx context.Context, userID uuid.UUID) error
	DeleteExpiredSessions(ctx context.Context) error
}

// Service handles authentication operations
type Service struct {
	repo          Repository
	sessionMaxAge time.Duration
	bcryptCost    int
}

// NewService creates a new auth service
func NewService(repo Repository, sessionMaxAge time.Duration) *Service {
	return &Service{
		repo:          repo,
		sessionMaxAge: sessionMaxAge,
		bcryptCost:    bcrypt.DefaultCost,
	}
}

// RegisterRequest contains registration data
type RegisterRequest struct {
	Email    string
	Name     string
	Password string
}

// Register creates a new user account
func (s *Service) Register(ctx context.Context, req RegisterRequest) (*domain.User, error) {
	// Check if email already exists
	existing, err := s.repo.GetUserByEmail(ctx, req.Email)
	if err == nil && existing != nil {
		return nil, ErrEmailExists
	}

	// Hash password
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(req.Password), s.bcryptCost)
	if err != nil {
		return nil, err
	}

	user := &domain.User{
		ID:           uuid.New(),
		Email:        req.Email,
		Name:         req.Name,
		PasswordHash: string(hashedPassword),
		CreatedAt:    time.Now(),
		UpdatedAt:    time.Now(),
	}

	if err := s.repo.CreateUser(ctx, user); err != nil {
		return nil, err
	}

	return user, nil
}

// LoginRequest contains login credentials
type LoginRequest struct {
	Email    string
	Password string
}

// LoginResponse contains login result
type LoginResponse struct {
	User    *domain.User
	Session *domain.Session
	Token   string
}

// Login authenticates a user and creates a session
func (s *Service) Login(ctx context.Context, req LoginRequest) (*LoginResponse, error) {
	user, err := s.repo.GetUserByEmail(ctx, req.Email)
	if err != nil {
		return nil, ErrInvalidCredentials
	}

	// Compare password
	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(req.Password)); err != nil {
		return nil, ErrInvalidCredentials
	}

	// Generate session token
	token, err := generateToken(32)
	if err != nil {
		return nil, err
	}

	session := &domain.Session{
		ID:        uuid.New(),
		UserID:    user.ID,
		Token:     token,
		ExpiresAt: time.Now().Add(s.sessionMaxAge),
		CreatedAt: time.Now(),
	}

	if err := s.repo.CreateSession(ctx, session); err != nil {
		return nil, err
	}

	return &LoginResponse{
		User:    user,
		Session: session,
		Token:   token,
	}, nil
}

// Logout invalidates a session
func (s *Service) Logout(ctx context.Context, token string) error {
	session, err := s.repo.GetSessionByToken(ctx, token)
	if err != nil {
		return ErrSessionNotFound
	}

	return s.repo.DeleteSession(ctx, session.ID)
}

// ValidateSession checks if a session token is valid
func (s *Service) ValidateSession(ctx context.Context, token string) (*domain.User, *domain.Session, error) {
	session, err := s.repo.GetSessionByToken(ctx, token)
	if err != nil {
		return nil, nil, ErrSessionNotFound
	}

	if session.IsExpired() {
		_ = s.repo.DeleteSession(ctx, session.ID)
		return nil, nil, ErrSessionExpired
	}

	user, err := s.repo.GetUserByID(ctx, session.UserID)
	if err != nil {
		return nil, nil, err
	}

	return user, session, nil
}

// LogoutAll invalidates all sessions for a user
func (s *Service) LogoutAll(ctx context.Context, userID uuid.UUID) error {
	return s.repo.DeleteUserSessions(ctx, userID)
}

// CleanupExpiredSessions removes all expired sessions
func (s *Service) CleanupExpiredSessions(ctx context.Context) error {
	return s.repo.DeleteExpiredSessions(ctx)
}

// generateToken creates a cryptographically secure random token
func generateToken(length int) (string, error) {
	bytes := make([]byte, length)
	if _, err := rand.Read(bytes); err != nil {
		return "", err
	}
	return base64.URLEncoding.EncodeToString(bytes), nil
}
