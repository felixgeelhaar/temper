package spec

import (
	"errors"
	"testing"

	"github.com/felixgeelhaar/temper/internal/domain"
)

func TestFileStore_RejectsPathTraversal(t *testing.T) {
	store := NewFileStore(t.TempDir())

	spec := &domain.ProductSpec{
		Name:     "Bad Spec",
		Version:  "1.0.0",
		FilePath: "../evil.yaml",
	}

	if err := store.Save(spec); !errors.Is(err, ErrInvalidPath) {
		t.Fatalf("Save() error = %v, want %v", err, ErrInvalidPath)
	}

	if _, err := store.Load("../evil.yaml"); !errors.Is(err, ErrInvalidPath) {
		t.Fatalf("Load() error = %v, want %v", err, ErrInvalidPath)
	}

	if err := store.Delete("../evil.yaml"); !errors.Is(err, ErrInvalidPath) {
		t.Fatalf("Delete() error = %v, want %v", err, ErrInvalidPath)
	}
}

func TestFileStore_AllowsSpecsDirPrefix(t *testing.T) {
	store := NewFileStore(t.TempDir())

	spec := &domain.ProductSpec{
		Name:     "Spec",
		Version:  "1.0.0",
		FilePath: ".specs/ok.yaml",
	}

	if err := store.Save(spec); err != nil {
		t.Fatalf("Save() error = %v", err)
	}

	loaded, err := store.Load(".specs/ok.yaml")
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if loaded.FilePath == "" {
		t.Fatalf("Load() FilePath should be set")
	}
}

func TestFileStore_SaveNormalizesPath(t *testing.T) {
	store := NewFileStore(t.TempDir())

	spec := &domain.ProductSpec{
		Name:     "Spec",
		Version:  "1.0.0",
		FilePath: "./nested/../ok.yaml",
	}

	if err := store.Save(spec); err != nil {
		t.Fatalf("Save() error = %v", err)
	}

	loaded, err := store.Load("nested/../ok.yaml")
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if loaded.Name != spec.Name {
		t.Fatalf("Load() name = %q, want %q", loaded.Name, spec.Name)
	}
}
