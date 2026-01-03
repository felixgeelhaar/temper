package domain

import (
	"testing"
	"time"

	"github.com/google/uuid"
)

func TestArtifact_GetFile(t *testing.T) {
	artifact := &Artifact{
		ID:      uuid.New(),
		Content: map[string]string{"main.go": "package main"},
	}

	tests := []struct {
		name     string
		filename string
		want     string
		wantOK   bool
	}{
		{"existing file", "main.go", "package main", true},
		{"non-existent file", "other.go", "", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, ok := artifact.GetFile(tt.filename)
			if ok != tt.wantOK {
				t.Errorf("GetFile(%q) ok = %v, want %v", tt.filename, ok, tt.wantOK)
			}
			if got != tt.want {
				t.Errorf("GetFile(%q) = %q, want %q", tt.filename, got, tt.want)
			}
		})
	}
}

func TestArtifact_SetFile(t *testing.T) {
	t.Run("set on nil content", func(t *testing.T) {
		artifact := &Artifact{ID: uuid.New()}
		artifact.SetFile("test.go", "test content")

		if artifact.Content == nil {
			t.Fatal("SetFile should initialize Content map")
		}
		if artifact.Content["test.go"] != "test content" {
			t.Errorf("Content[test.go] = %q, want %q", artifact.Content["test.go"], "test content")
		}
	})

	t.Run("set on existing content", func(t *testing.T) {
		artifact := &Artifact{
			ID:      uuid.New(),
			Content: map[string]string{"existing.go": "existing"},
		}
		artifact.SetFile("new.go", "new content")

		if artifact.Content["new.go"] != "new content" {
			t.Errorf("Content[new.go] = %q, want %q", artifact.Content["new.go"], "new content")
		}
		if artifact.Content["existing.go"] != "existing" {
			t.Error("SetFile should not affect existing files")
		}
	})

	t.Run("overwrite existing file", func(t *testing.T) {
		artifact := &Artifact{
			ID:      uuid.New(),
			Content: map[string]string{"test.go": "original"},
		}
		artifact.SetFile("test.go", "updated")

		if artifact.Content["test.go"] != "updated" {
			t.Errorf("Content[test.go] = %q, want %q", artifact.Content["test.go"], "updated")
		}
	})
}

func TestArtifact_DeleteFile(t *testing.T) {
	artifact := &Artifact{
		ID: uuid.New(),
		Content: map[string]string{
			"main.go": "package main",
			"test.go": "package main",
		},
	}

	artifact.DeleteFile("main.go")

	if _, exists := artifact.Content["main.go"]; exists {
		t.Error("DeleteFile should remove the file")
	}
	if _, exists := artifact.Content["test.go"]; !exists {
		t.Error("DeleteFile should not affect other files")
	}

	// Deleting non-existent file should not panic
	artifact.DeleteFile("nonexistent.go")
}

func TestArtifact_FileCount(t *testing.T) {
	tests := []struct {
		name    string
		content map[string]string
		want    int
	}{
		{"nil content", nil, 0},
		{"empty content", map[string]string{}, 0},
		{"one file", map[string]string{"a.go": "a"}, 1},
		{"multiple files", map[string]string{"a.go": "a", "b.go": "b", "c.go": "c"}, 3},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			artifact := &Artifact{ID: uuid.New(), Content: tt.content}
			if got := artifact.FileCount(); got != tt.want {
				t.Errorf("FileCount() = %d, want %d", got, tt.want)
			}
		})
	}
}

func TestArtifact_Struct(t *testing.T) {
	now := time.Now()
	exerciseID := "go-v1/hello"
	artifact := &Artifact{
		ID:         uuid.New(),
		UserID:     uuid.New(),
		ExerciseID: &exerciseID,
		Name:       "My Artifact",
		Content:    map[string]string{"main.go": "package main"},
		CreatedAt:  now,
		UpdatedAt:  now,
	}

	if artifact.Name != "My Artifact" {
		t.Errorf("Name = %q, want %q", artifact.Name, "My Artifact")
	}
	if *artifact.ExerciseID != exerciseID {
		t.Errorf("ExerciseID = %q, want %q", *artifact.ExerciseID, exerciseID)
	}
}

func TestArtifactVersion_Struct(t *testing.T) {
	now := time.Now()
	version := &ArtifactVersion{
		ID:         uuid.New(),
		ArtifactID: uuid.New(),
		Version:    1,
		Content:    map[string]string{"main.go": "package main"},
		CreatedAt:  now,
	}

	if version.Version != 1 {
		t.Errorf("Version = %d, want 1", version.Version)
	}
}
