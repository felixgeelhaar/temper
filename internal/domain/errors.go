package domain

import "errors"

// -----------------------------------------------------------------------------
// Domain Errors
// These errors represent domain-level failures and are used by repositories
// and services to communicate domain-specific error conditions.
// -----------------------------------------------------------------------------

// User errors
var (
	ErrUserNotFound      = errors.New("user not found")
	ErrUserAlreadyExists = errors.New("user already exists")
	ErrInvalidEmail      = errors.New("invalid email address")
	ErrInvalidPassword   = errors.New("invalid password")
)

// Session errors (auth sessions)
var (
	ErrAuthSessionNotFound = errors.New("auth session not found")
	ErrAuthSessionExpired  = errors.New("auth session expired")
	ErrAuthSessionRevoked  = errors.New("auth session revoked")
)

// Artifact errors
var (
	ErrArtifactNotFound      = errors.New("artifact not found")
	ErrArtifactVersionNotFound = errors.New("artifact version not found")
)

// Learning profile errors
var (
	ErrLearningProfileNotFound = errors.New("learning profile not found")
)

// Intervention errors
var (
	ErrInterventionNotFound = errors.New("intervention not found")
)

// Run errors
var (
	ErrRunNotFound = errors.New("run not found")
)

// Exercise errors
var (
	ErrExerciseNotFound = errors.New("exercise not found")
	ErrExercisePackNotFound = errors.New("exercise pack not found")
)

// General errors
var (
	ErrNotFound          = errors.New("not found")
	ErrConflict          = errors.New("conflict")
	ErrInvalidInput      = errors.New("invalid input")
	ErrUnauthorized      = errors.New("unauthorized")
	ErrForbidden         = errors.New("forbidden")
	ErrInternalError     = errors.New("internal error")
)
