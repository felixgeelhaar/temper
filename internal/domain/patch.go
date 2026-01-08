package domain

import (
	"time"

	"github.com/google/uuid"
)

// Patch represents a code change extracted from an intervention
type Patch struct {
	ID             uuid.UUID   `json:"id"`
	InterventionID uuid.UUID   `json:"intervention_id"`
	SessionID      uuid.UUID   `json:"session_id"`
	File           string      `json:"file"`
	Original       string      `json:"original"`    // original content (if known)
	Proposed       string      `json:"proposed"`    // proposed new content
	Diff           string      `json:"diff"`        // unified diff format
	Description    string      `json:"description"` // what this patch does
	Status         PatchStatus `json:"status"`
	CreatedAt      time.Time   `json:"created_at"`
	AppliedAt      *time.Time  `json:"applied_at,omitempty"`
}

// PatchStatus represents the state of a patch
type PatchStatus string

const (
	PatchStatusPending  PatchStatus = "pending"  // awaiting user review
	PatchStatusApproved PatchStatus = "approved" // user approved, ready to apply
	PatchStatusApplied  PatchStatus = "applied"  // successfully applied
	PatchStatusRejected PatchStatus = "rejected" // user rejected
	PatchStatusExpired  PatchStatus = "expired"  // session ended without action
)

// PatchPreview contains information for displaying a patch to the user
type PatchPreview struct {
	Patch         *Patch   `json:"patch"`
	ContextBefore []string `json:"context_before"` // lines before the change
	ContextAfter  []string `json:"context_after"`  // lines after the change
	Additions     int      `json:"additions"`      // lines added
	Deletions     int      `json:"deletions"`      // lines removed
	Warnings      []string `json:"warnings"`       // any concerns about the patch
}

// PatchSet represents multiple patches from a single intervention
type PatchSet struct {
	ID             uuid.UUID `json:"id"`
	InterventionID uuid.UUID `json:"intervention_id"`
	SessionID      uuid.UUID `json:"session_id"`
	Patches        []*Patch  `json:"patches"`
	Description    string    `json:"description"`
	CreatedAt      time.Time `json:"created_at"`
}

// IsPending returns true if the patch is awaiting action
func (p *Patch) IsPending() bool {
	return p.Status == PatchStatusPending
}

// CanApply returns true if the patch can be applied
func (p *Patch) CanApply() bool {
	return p.Status == PatchStatusPending || p.Status == PatchStatusApproved
}

// Summary returns a brief description of the patch
func (p *Patch) Summary() string {
	if p.Description != "" {
		return p.Description
	}
	return "Code change for " + p.File
}
