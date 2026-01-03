package repository

import (
	"context"
	"database/sql"

	"github.com/felixgeelhaar/temper/internal/domain"
	"github.com/felixgeelhaar/temper/internal/storage"
)

// PostgresUnitOfWork implements domain.UnitOfWork for PostgreSQL
type PostgresUnitOfWork struct {
	db      *sql.DB
	tx      *sql.Tx
	queries *storage.Queries

	// Lazy-initialized repositories
	users           *UserRepository
	authSessions    *AuthSessionRepository
	pairingSessions *PairingSessionRepository
	artifacts       *ArtifactRepository
	profiles        *LearningProfileRepository
	interventions   *InterventionRepository
	runs            *RunRepository
	events          *EventRepository
}

// NewPostgresUnitOfWork creates a new PostgresUnitOfWork
func NewPostgresUnitOfWork(db *sql.DB) *PostgresUnitOfWork {
	return &PostgresUnitOfWork{
		db:      db,
		queries: storage.New(db),
	}
}

// Begin starts a new unit of work with a transaction
func (uow *PostgresUnitOfWork) Begin(ctx context.Context) (domain.UnitOfWork, error) {
	tx, err := uow.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}

	return &PostgresUnitOfWork{
		db:      uow.db,
		tx:      tx,
		queries: storage.New(tx),
	}, nil
}

// Commit commits the transaction
func (uow *PostgresUnitOfWork) Commit() error {
	if uow.tx == nil {
		return nil
	}
	return uow.tx.Commit()
}

// Rollback rolls back the transaction
func (uow *PostgresUnitOfWork) Rollback() error {
	if uow.tx == nil {
		return nil
	}
	return uow.tx.Rollback()
}

// Users returns the user repository
func (uow *PostgresUnitOfWork) Users() domain.UserRepository {
	if uow.users == nil {
		uow.users = NewUserRepository(uow.queries)
	}
	return uow.users
}

// AuthSessions returns the auth session repository
func (uow *PostgresUnitOfWork) AuthSessions() domain.AuthSessionRepository {
	if uow.authSessions == nil {
		uow.authSessions = NewAuthSessionRepository(uow.queries)
	}
	return uow.authSessions
}

// PairingSessions returns the pairing session repository
func (uow *PostgresUnitOfWork) PairingSessions() domain.PairingSessionRepository {
	if uow.pairingSessions == nil {
		uow.pairingSessions = NewPairingSessionRepository(uow.queries)
	}
	return uow.pairingSessions
}

// Artifacts returns the artifact repository
func (uow *PostgresUnitOfWork) Artifacts() domain.ArtifactRepository {
	if uow.artifacts == nil {
		uow.artifacts = NewArtifactRepository(uow.queries)
	}
	return uow.artifacts
}

// LearningProfiles returns the learning profile repository
func (uow *PostgresUnitOfWork) LearningProfiles() domain.LearningProfileRepository {
	if uow.profiles == nil {
		uow.profiles = NewLearningProfileRepository(uow.queries)
	}
	return uow.profiles
}

// Interventions returns the intervention repository
func (uow *PostgresUnitOfWork) Interventions() domain.InterventionRepository {
	if uow.interventions == nil {
		uow.interventions = NewInterventionRepository(uow.queries)
	}
	return uow.interventions
}

// Runs returns the run repository
func (uow *PostgresUnitOfWork) Runs() domain.RunRepository {
	if uow.runs == nil {
		uow.runs = NewRunRepository(uow.queries)
	}
	return uow.runs
}

// Events returns the event repository
func (uow *PostgresUnitOfWork) Events() domain.EventRepository {
	if uow.events == nil {
		uow.events = NewEventRepository(uow.queries)
	}
	return uow.events
}

// Ensure PostgresUnitOfWork implements domain.UnitOfWork
var _ domain.UnitOfWork = (*PostgresUnitOfWork)(nil)
