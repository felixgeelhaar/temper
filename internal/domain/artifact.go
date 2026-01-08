package domain

import (
	"time"

	"github.com/google/uuid"
)

// Artifact represents a user's code workspace
type Artifact struct {
	ID         uuid.UUID
	UserID     uuid.UUID
	ExerciseID *string // nil for freeform workspaces
	Name       string
	Content    map[string]string // filename -> content
	CreatedAt  time.Time
	UpdatedAt  time.Time
}

// ArtifactVersion represents a saved version of an artifact
type ArtifactVersion struct {
	ID         uuid.UUID
	ArtifactID uuid.UUID
	Version    int
	Content    map[string]string
	CreatedAt  time.Time
}

// GetFile returns the content of a file in the artifact
func (a *Artifact) GetFile(filename string) (string, bool) {
	content, ok := a.Content[filename]
	return content, ok
}

// SetFile sets the content of a file in the artifact
func (a *Artifact) SetFile(filename, content string) {
	if a.Content == nil {
		a.Content = make(map[string]string)
	}
	a.Content[filename] = content
}

// DeleteFile removes a file from the artifact
func (a *Artifact) DeleteFile(filename string) {
	delete(a.Content, filename)
}

// FileCount returns the number of files in the artifact
func (a *Artifact) FileCount() int {
	return len(a.Content)
}
