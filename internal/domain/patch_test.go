package domain

import (
	"testing"
	"time"

	"github.com/google/uuid"
)

func TestPatch_IsPending(t *testing.T) {
	tests := []struct {
		name   string
		status PatchStatus
		want   bool
	}{
		{"pending", PatchStatusPending, true},
		{"approved", PatchStatusApproved, false},
		{"applied", PatchStatusApplied, false},
		{"rejected", PatchStatusRejected, false},
		{"expired", PatchStatusExpired, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			patch := &Patch{ID: uuid.New(), Status: tt.status}
			if got := patch.IsPending(); got != tt.want {
				t.Errorf("IsPending() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestPatch_CanApply(t *testing.T) {
	tests := []struct {
		name   string
		status PatchStatus
		want   bool
	}{
		{"pending", PatchStatusPending, true},
		{"approved", PatchStatusApproved, true},
		{"applied", PatchStatusApplied, false},
		{"rejected", PatchStatusRejected, false},
		{"expired", PatchStatusExpired, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			patch := &Patch{ID: uuid.New(), Status: tt.status}
			if got := patch.CanApply(); got != tt.want {
				t.Errorf("CanApply() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestPatch_Summary(t *testing.T) {
	tests := []struct {
		name        string
		description string
		file        string
		want        string
	}{
		{"with description", "Add error handling", "main.go", "Add error handling"},
		{"empty description", "", "handler.go", "Code change for handler.go"},
		{"whitespace description", "   ", "test.go", "   "}, // trims check handled elsewhere
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			patch := &Patch{
				ID:          uuid.New(),
				File:        tt.file,
				Description: tt.description,
			}
			if got := patch.Summary(); got != tt.want {
				t.Errorf("Summary() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestPatchStatus_Constants(t *testing.T) {
	tests := []struct {
		status PatchStatus
		want   string
	}{
		{PatchStatusPending, "pending"},
		{PatchStatusApproved, "approved"},
		{PatchStatusApplied, "applied"},
		{PatchStatusRejected, "rejected"},
		{PatchStatusExpired, "expired"},
	}

	for _, tt := range tests {
		t.Run(tt.want, func(t *testing.T) {
			if string(tt.status) != tt.want {
				t.Errorf("PatchStatus = %q, want %q", tt.status, tt.want)
			}
		})
	}
}

func TestPatch_Struct(t *testing.T) {
	now := time.Now()
	appliedAt := now.Add(time.Minute)

	patch := &Patch{
		ID:             uuid.New(),
		InterventionID: uuid.New(),
		SessionID:      uuid.New(),
		File:           "main.go",
		Original:       "old content",
		Proposed:       "new content",
		Diff:           "+new\n-old",
		Description:    "Update code",
		Status:         PatchStatusApplied,
		CreatedAt:      now,
		AppliedAt:      &appliedAt,
	}

	if patch.File != "main.go" {
		t.Errorf("File = %q, want main.go", patch.File)
	}
	if patch.Original != "old content" {
		t.Errorf("Original = %q, want old content", patch.Original)
	}
	if patch.AppliedAt == nil {
		t.Error("AppliedAt should not be nil")
	}
}

func TestPatchPreview_Struct(t *testing.T) {
	preview := &PatchPreview{
		Patch:         &Patch{ID: uuid.New(), File: "test.go"},
		ContextBefore: []string{"line1", "line2"},
		ContextAfter:  []string{"line3", "line4"},
		Additions:     5,
		Deletions:     2,
		Warnings:      []string{"Large change"},
	}

	if preview.Additions != 5 {
		t.Errorf("Additions = %d, want 5", preview.Additions)
	}
	if preview.Deletions != 2 {
		t.Errorf("Deletions = %d, want 2", preview.Deletions)
	}
	if len(preview.Warnings) != 1 {
		t.Errorf("Warnings len = %d, want 1", len(preview.Warnings))
	}
}

func TestPatchSet_Struct(t *testing.T) {
	now := time.Now()
	patchSet := &PatchSet{
		ID:             uuid.New(),
		InterventionID: uuid.New(),
		SessionID:      uuid.New(),
		Patches: []*Patch{
			{ID: uuid.New(), File: "a.go"},
			{ID: uuid.New(), File: "b.go"},
		},
		Description: "Multiple file changes",
		CreatedAt:   now,
	}

	if len(patchSet.Patches) != 2 {
		t.Errorf("Patches len = %d, want 2", len(patchSet.Patches))
	}
	if patchSet.Description != "Multiple file changes" {
		t.Errorf("Description = %q, want Multiple file changes", patchSet.Description)
	}
}
