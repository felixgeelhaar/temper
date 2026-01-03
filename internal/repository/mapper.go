// Package repository provides the anti-corruption layer between
// the domain and storage layers. It implements domain repository
// interfaces using the storage layer internally.
package repository

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/sqlc-dev/pqtype"

	"github.com/felixgeelhaar/temper/internal/domain"
	"github.com/felixgeelhaar/temper/internal/storage"
)

// -----------------------------------------------------------------------------
// User Mappers
// -----------------------------------------------------------------------------

// mapUserToDomain converts a storage User to a domain User
func mapUserToDomain(s storage.User) *domain.User {
	return &domain.User{
		ID:           s.ID,
		Email:        s.Email,
		Name:         nullStringValue(s.Name),
		PasswordHash: s.PasswordHash,
		CreatedAt:    s.CreatedAt,
		UpdatedAt:    s.UpdatedAt,
	}
}

// mapUserToStorage converts a domain User to storage parameters
func mapUserToStorage(u *domain.User) storage.CreateUserParams {
	return storage.CreateUserParams{
		Email:        u.Email,
		Name:         stringToNullString(u.Name),
		PasswordHash: u.PasswordHash,
	}
}

// -----------------------------------------------------------------------------
// AuthSession Mappers
// -----------------------------------------------------------------------------

// mapAuthSessionToDomain converts a storage Session to a domain AuthSession
func mapAuthSessionToDomain(s storage.Session) *domain.AuthSession {
	return &domain.AuthSession{
		ID:        s.ID,
		UserID:    s.UserID,
		Token:     s.Token,
		ExpiresAt: s.ExpiresAt,
		CreatedAt: s.CreatedAt,
	}
}

// mapAuthSessionToStorage converts a domain AuthSession to storage parameters
func mapAuthSessionToStorage(s *domain.AuthSession) storage.CreateSessionParams {
	return storage.CreateSessionParams{
		UserID:    s.UserID,
		Token:     s.Token,
		ExpiresAt: s.ExpiresAt,
	}
}

// -----------------------------------------------------------------------------
// Artifact Mappers
// -----------------------------------------------------------------------------

// mapArtifactToDomain converts a storage Artifact to a domain Artifact
func mapArtifactToDomain(s storage.Artifact) (*domain.Artifact, error) {
	var content map[string]string
	if len(s.Content) > 0 {
		if err := json.Unmarshal(s.Content, &content); err != nil {
			return nil, err
		}
	}

	return &domain.Artifact{
		ID:         s.ID,
		UserID:     s.UserID,
		ExerciseID: nullStringToPtr(s.ExerciseID),
		Name:       s.Name,
		Content:    content,
		CreatedAt:  s.CreatedAt,
		UpdatedAt:  s.UpdatedAt,
	}, nil
}

// mapArtifactToStorage converts a domain Artifact to storage parameters
func mapArtifactToStorage(a *domain.Artifact) (storage.CreateArtifactParams, error) {
	content, err := json.Marshal(a.Content)
	if err != nil {
		return storage.CreateArtifactParams{}, err
	}

	return storage.CreateArtifactParams{
		UserID:     a.UserID,
		ExerciseID: ptrToNullString(a.ExerciseID),
		Name:       a.Name,
		Content:    content,
	}, nil
}

// mapArtifactVersionToDomain converts a storage ArtifactVersion to a domain ArtifactVersion
func mapArtifactVersionToDomain(s storage.ArtifactVersion) (*domain.ArtifactVersion, error) {
	var content map[string]string
	if len(s.Content) > 0 {
		if err := json.Unmarshal(s.Content, &content); err != nil {
			return nil, err
		}
	}

	return &domain.ArtifactVersion{
		ID:         s.ID,
		ArtifactID: s.ArtifactID,
		Version:    int(s.Version),
		Content:    content,
		CreatedAt:  s.CreatedAt,
	}, nil
}

// -----------------------------------------------------------------------------
// LearningProfile Mappers
// -----------------------------------------------------------------------------

// mapLearningProfileToDomain converts a storage LearningProfile to a domain LearningProfile
func mapLearningProfileToDomain(s storage.LearningProfile) (*domain.LearningProfile, error) {
	var topicSkills map[string]domain.SkillLevel
	if len(s.TopicSkills) > 0 {
		if err := json.Unmarshal(s.TopicSkills, &topicSkills); err != nil {
			return nil, err
		}
	}

	var avgTimeToGreen time.Duration
	if s.AvgTimeToGreenMs.Valid {
		avgTimeToGreen = time.Duration(s.AvgTimeToGreenMs.Int64) * time.Millisecond
	}

	return &domain.LearningProfile{
		ID:             s.ID,
		UserID:         s.UserID,
		TopicSkills:    topicSkills,
		TotalExercises: int(s.TotalExercises),
		TotalRuns:      int(s.TotalRuns),
		HintRequests:   int(s.HintRequests),
		AvgTimeToGreen: avgTimeToGreen,
		CommonErrors:   s.CommonErrors,
		UpdatedAt:      s.UpdatedAt,
	}, nil
}

// -----------------------------------------------------------------------------
// Intervention Mappers
// -----------------------------------------------------------------------------

// mapInterventionToDomain converts a storage Intervention to a domain Intervention
func mapInterventionToDomain(s storage.Intervention) (*domain.Intervention, error) {
	var targets []domain.Target
	if s.Targets.Valid && len(s.Targets.RawMessage) > 0 {
		if err := json.Unmarshal(s.Targets.RawMessage, &targets); err != nil {
			return nil, err
		}
	}

	intent, err := domain.NewIntent(s.Intent)
	if err != nil {
		return nil, fmt.Errorf("invalid intent: %w", err)
	}

	interventionType, err := domain.NewInterventionType(s.Type)
	if err != nil {
		return nil, fmt.Errorf("invalid intervention type: %w", err)
	}

	return &domain.Intervention{
		ID:          s.ID,
		SessionID:   s.SessionID,
		UserID:      s.UserID,
		RunID:       nullUUIDToPtr(s.RunID),
		Intent:      intent,
		Level:       domain.InterventionLevel(s.Level),
		Type:        interventionType,
		Content:     s.Content,
		Targets:     targets,
		Rationale:   nullStringValue(s.Rationale),
		RequestedAt: s.RequestedAt,
		DeliveredAt: s.DeliveredAt,
	}, nil
}

// mapInterventionToStorage converts a domain Intervention to storage parameters
func mapInterventionToStorage(i *domain.Intervention) (storage.CreateInterventionParams, error) {
	var targets pqtype.NullRawMessage
	if len(i.Targets) > 0 {
		data, err := json.Marshal(i.Targets)
		if err != nil {
			return storage.CreateInterventionParams{}, err
		}
		targets = pqtype.NullRawMessage{RawMessage: data, Valid: true}
	}

	return storage.CreateInterventionParams{
		SessionID:   i.SessionID,
		UserID:      i.UserID,
		RunID:       ptrToNullUUID(i.RunID),
		Intent:      i.Intent.String(),
		Level:       int32(i.Level),
		Type:        i.Type.String(),
		Content:     i.Content,
		Targets:     targets,
		Rationale:   stringToNullString(i.Rationale),
		RequestedAt: i.RequestedAt,
	}, nil
}

// -----------------------------------------------------------------------------
// Run Mappers
// -----------------------------------------------------------------------------

// mapRunToDomain converts a storage Run to a domain Run
func mapRunToDomain(s storage.Run) (*domain.Run, error) {
	var recipe domain.CheckRecipe
	if len(s.Recipe) > 0 {
		if err := json.Unmarshal(s.Recipe, &recipe); err != nil {
			return nil, err
		}
	}

	var output *domain.RunOutput
	if s.Output.Valid && len(s.Output.RawMessage) > 0 {
		output = &domain.RunOutput{}
		if err := json.Unmarshal(s.Output.RawMessage, output); err != nil {
			return nil, err
		}
	}

	status, err := domain.NewRunStatus(s.Status)
	if err != nil {
		return nil, fmt.Errorf("invalid run status: %w", err)
	}

	return &domain.Run{
		ID:         s.ID,
		ArtifactID: s.ArtifactID,
		UserID:     s.UserID,
		ExerciseID: nullStringToPtr(s.ExerciseID),
		Status:     status,
		Recipe:     recipe,
		Output:     output,
		StartedAt:  nullTimeToPtr(s.StartedAt),
		FinishedAt: nullTimeToPtr(s.FinishedAt),
		CreatedAt:  s.CreatedAt,
	}, nil
}

// mapRunToStorage converts a domain Run to storage parameters
func mapRunToStorage(r *domain.Run) (storage.CreateRunParams, error) {
	recipe, err := json.Marshal(r.Recipe)
	if err != nil {
		return storage.CreateRunParams{}, err
	}

	return storage.CreateRunParams{
		ArtifactID: r.ArtifactID,
		UserID:     r.UserID,
		ExerciseID: ptrToNullString(r.ExerciseID),
		Status:     r.Status.String(),
		Recipe:     recipe,
	}, nil
}

// -----------------------------------------------------------------------------
// Helper Functions
// -----------------------------------------------------------------------------

func nullStringToPtr(ns sql.NullString) *string {
	if ns.Valid {
		return &ns.String
	}
	return nil
}

func ptrToNullString(s *string) sql.NullString {
	if s != nil {
		return sql.NullString{String: *s, Valid: true}
	}
	return sql.NullString{}
}

func stringToNullString(s string) sql.NullString {
	if s != "" {
		return sql.NullString{String: s, Valid: true}
	}
	return sql.NullString{}
}

func nullStringValue(ns sql.NullString) string {
	if ns.Valid {
		return ns.String
	}
	return ""
}

func nullTimeToPtr(nt sql.NullTime) *time.Time {
	if nt.Valid {
		return &nt.Time
	}
	return nil
}

func ptrToNullTime(t *time.Time) sql.NullTime {
	if t != nil {
		return sql.NullTime{Time: *t, Valid: true}
	}
	return sql.NullTime{}
}

func nullUUIDToPtr(nu uuid.NullUUID) *uuid.UUID {
	if nu.Valid {
		return &nu.UUID
	}
	return nil
}

func ptrToNullUUID(u *uuid.UUID) uuid.NullUUID {
	if u != nil {
		return uuid.NullUUID{UUID: *u, Valid: true}
	}
	return uuid.NullUUID{}
}
