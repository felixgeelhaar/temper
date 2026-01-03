package domain

import (
	"errors"
	"fmt"
	"regexp"
	"strings"

	"github.com/google/uuid"
)

// ErrInvalidID indicates an invalid identifier format
var ErrInvalidID = errors.New("invalid identifier format")

// -----------------------------------------------------------------------------
// UserID - Typed identifier for users
// -----------------------------------------------------------------------------

// UserID is a typed identifier for users
type UserID struct {
	value uuid.UUID
}

// NewUserID creates a new UserID from a UUID
func NewUserID(id uuid.UUID) UserID {
	return UserID{value: id}
}

// NewUserIDFromString creates a UserID from a string
func NewUserIDFromString(s string) (UserID, error) {
	id, err := uuid.Parse(s)
	if err != nil {
		return UserID{}, fmt.Errorf("invalid user ID: %w", err)
	}
	return UserID{value: id}, nil
}

// GenerateUserID creates a new random UserID
func GenerateUserID() UserID {
	return UserID{value: uuid.New()}
}

// UUID returns the underlying uuid.UUID
func (id UserID) UUID() uuid.UUID {
	return id.value
}

// String returns the string representation
func (id UserID) String() string {
	return id.value.String()
}

// IsZero returns true if this is a zero value
func (id UserID) IsZero() bool {
	return id.value == uuid.Nil
}

// Equal compares two UserIDs
func (id UserID) Equal(other UserID) bool {
	return id.value == other.value
}

// -----------------------------------------------------------------------------
// SessionID - Typed identifier for pairing sessions
// -----------------------------------------------------------------------------

// SessionID is a typed identifier for pairing sessions
type SessionID struct {
	value uuid.UUID
}

// NewSessionID creates a new SessionID from a UUID
func NewSessionID(id uuid.UUID) SessionID {
	return SessionID{value: id}
}

// NewSessionIDFromString creates a SessionID from a string
func NewSessionIDFromString(s string) (SessionID, error) {
	id, err := uuid.Parse(s)
	if err != nil {
		return SessionID{}, fmt.Errorf("invalid session ID: %w", err)
	}
	return SessionID{value: id}, nil
}

// GenerateSessionID creates a new random SessionID
func GenerateSessionID() SessionID {
	return SessionID{value: uuid.New()}
}

// UUID returns the underlying uuid.UUID
func (id SessionID) UUID() uuid.UUID {
	return id.value
}

// String returns the string representation
func (id SessionID) String() string {
	return id.value.String()
}

// IsZero returns true if this is a zero value
func (id SessionID) IsZero() bool {
	return id.value == uuid.Nil
}

// Equal compares two SessionIDs
func (id SessionID) Equal(other SessionID) bool {
	return id.value == other.value
}

// -----------------------------------------------------------------------------
// ArtifactID - Typed identifier for artifacts
// -----------------------------------------------------------------------------

// ArtifactID is a typed identifier for artifacts
type ArtifactID struct {
	value uuid.UUID
}

// NewArtifactID creates a new ArtifactID from a UUID
func NewArtifactID(id uuid.UUID) ArtifactID {
	return ArtifactID{value: id}
}

// NewArtifactIDFromString creates an ArtifactID from a string
func NewArtifactIDFromString(s string) (ArtifactID, error) {
	id, err := uuid.Parse(s)
	if err != nil {
		return ArtifactID{}, fmt.Errorf("invalid artifact ID: %w", err)
	}
	return ArtifactID{value: id}, nil
}

// GenerateArtifactID creates a new random ArtifactID
func GenerateArtifactID() ArtifactID {
	return ArtifactID{value: uuid.New()}
}

// UUID returns the underlying uuid.UUID
func (id ArtifactID) UUID() uuid.UUID {
	return id.value
}

// String returns the string representation
func (id ArtifactID) String() string {
	return id.value.String()
}

// IsZero returns true if this is a zero value
func (id ArtifactID) IsZero() bool {
	return id.value == uuid.Nil
}

// Equal compares two ArtifactIDs
func (id ArtifactID) Equal(other ArtifactID) bool {
	return id.value == other.value
}

// -----------------------------------------------------------------------------
// ExerciseID - Value object for exercise identifiers
// -----------------------------------------------------------------------------

// exerciseIDPattern validates exercise ID format: pack/exercise-name
var exerciseIDPattern = regexp.MustCompile(`^[a-zA-Z0-9-]+/[a-zA-Z0-9_-]+$`)

// ExerciseID is a value object for exercise identifiers
// Format: "pack-id/exercise-slug" (e.g., "go-v1/hello-world")
type ExerciseID struct {
	value string
}

// NewExerciseID creates a new ExerciseID from a string
func NewExerciseID(s string) (ExerciseID, error) {
	if s == "" {
		return ExerciseID{}, fmt.Errorf("%w: exercise ID cannot be empty", ErrInvalidID)
	}
	if !exerciseIDPattern.MatchString(s) {
		return ExerciseID{}, fmt.Errorf("%w: exercise ID must be in format 'pack/exercise-name'", ErrInvalidID)
	}
	return ExerciseID{value: s}, nil
}

// MustExerciseID creates a new ExerciseID, panicking on error
func MustExerciseID(s string) ExerciseID {
	id, err := NewExerciseID(s)
	if err != nil {
		panic(err)
	}
	return id
}

// String returns the string representation
func (id ExerciseID) String() string {
	return id.value
}

// Pack returns the pack portion of the exercise ID
func (id ExerciseID) Pack() string {
	parts := strings.SplitN(id.value, "/", 2)
	if len(parts) > 0 {
		return parts[0]
	}
	return ""
}

// Slug returns the exercise slug portion of the ID
func (id ExerciseID) Slug() string {
	parts := strings.SplitN(id.value, "/", 2)
	if len(parts) > 1 {
		return parts[1]
	}
	return ""
}

// IsZero returns true if this is a zero value
func (id ExerciseID) IsZero() bool {
	return id.value == ""
}

// Equal compares two ExerciseIDs
func (id ExerciseID) Equal(other ExerciseID) bool {
	return id.value == other.value
}

// -----------------------------------------------------------------------------
// Email - Value object for email addresses
// -----------------------------------------------------------------------------

// emailPattern is a basic email validation pattern
var emailPattern = regexp.MustCompile(`^[a-zA-Z0-9._%+-]+@[a-zA-Z0-9.-]+\.[a-zA-Z]{2,}$`)

// Email is a value object for validated email addresses
type Email struct {
	value string
}

// NewEmail creates a new Email from a string
func NewEmail(s string) (Email, error) {
	s = strings.TrimSpace(strings.ToLower(s))
	if s == "" {
		return Email{}, errors.New("email cannot be empty")
	}
	if !emailPattern.MatchString(s) {
		return Email{}, errors.New("invalid email format")
	}
	return Email{value: s}, nil
}

// String returns the normalized email address
func (e Email) String() string {
	return e.value
}

// IsZero returns true if this is a zero value
func (e Email) IsZero() bool {
	return e.value == ""
}

// Equal compares two Email values
func (e Email) Equal(other Email) bool {
	return e.value == other.value
}

// Domain returns the domain part of the email
func (e Email) Domain() string {
	parts := strings.SplitN(e.value, "@", 2)
	if len(parts) > 1 {
		return parts[1]
	}
	return ""
}

// LocalPart returns the local part of the email (before @)
func (e Email) LocalPart() string {
	parts := strings.SplitN(e.value, "@", 2)
	if len(parts) > 0 {
		return parts[0]
	}
	return ""
}

// Note: SkillLevel is defined in profile.go with more comprehensive tracking
