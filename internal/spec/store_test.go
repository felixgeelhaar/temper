package spec

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/felixgeelhaar/temper/internal/domain"
)

func TestNewFileStore(t *testing.T) {
	store := NewFileStore("/test/path")
	if store == nil {
		t.Fatal("NewFileStore() returned nil")
	}
}

func TestFileStore_SpecPath(t *testing.T) {
	store := NewFileStore("/workspace")

	tests := []struct {
		input   string
		want    string
		wantErr error
	}{
		{"feature.yaml", "/workspace/.specs/feature.yaml", nil},
		{".specs/feature.yaml", "/workspace/.specs/feature.yaml", nil},
		{"subdir/feature.yaml", "/workspace/.specs/subdir/feature.yaml", nil},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got, err := store.SpecPath(tt.input)
			if err != tt.wantErr {
				t.Errorf("SpecPath(%q) error = %v; want %v", tt.input, err, tt.wantErr)
			}
			if got != tt.want {
				t.Errorf("SpecPath(%q) = %q; want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestFileStore_SpecPath_PathTraversal(t *testing.T) {
	store := NewFileStore("/workspace")

	tests := []struct {
		name  string
		input string
	}{
		{"parent directory", "../secret.yaml"},
		{"double parent", "../../etc/passwd"},
		{"hidden traversal", "subdir/../../../secret.yaml"},
		{"absolute path attempt", ".specs/../../../etc/passwd"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := store.SpecPath(tt.input)
			if err != ErrPathTraversal {
				t.Errorf("SpecPath(%q) error = %v; want ErrPathTraversal", tt.input, err)
			}
		})
	}
}

func TestFileStore_Load_NotFound(t *testing.T) {
	tmpDir := t.TempDir()
	store := NewFileStore(tmpDir)

	_, err := store.Load("nonexistent.yaml")
	if err != ErrSpecNotFound {
		t.Errorf("Load() error = %v; want ErrSpecNotFound", err)
	}
}

func TestFileStore_Save_Load(t *testing.T) {
	tmpDir := t.TempDir()
	store := NewFileStore(tmpDir)

	spec := &domain.ProductSpec{
		Name:     "Test Spec",
		Version:  "1.0.0",
		FilePath: "test.yaml",
		Goals:    []string{"Goal 1"},
		Features: []domain.Feature{
			{
				ID:          "feat-1",
				Title:       "Feature 1",
				Description: "Test feature",
			},
		},
	}

	// Save
	if err := store.Save(spec); err != nil {
		t.Fatalf("Save() error = %v", err)
	}

	// Verify file exists
	fullPath, err := store.SpecPath(spec.FilePath)
	if err != nil {
		t.Fatalf("SpecPath() error = %v", err)
	}
	if _, err := os.Stat(fullPath); os.IsNotExist(err) {
		t.Error("Save() should create spec file")
	}

	// Load
	loaded, err := store.Load(spec.FilePath)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	if loaded.Name != spec.Name {
		t.Errorf("Name = %q; want %q", loaded.Name, spec.Name)
	}
	if loaded.Version != spec.Version {
		t.Errorf("Version = %q; want %q", loaded.Version, spec.Version)
	}
}

func TestFileStore_Save_InvalidPath(t *testing.T) {
	tmpDir := t.TempDir()
	store := NewFileStore(tmpDir)

	spec := &domain.ProductSpec{
		Name: "Test",
		// No FilePath
	}

	err := store.Save(spec)
	if err != ErrInvalidPath {
		t.Errorf("Save() error = %v; want ErrInvalidPath", err)
	}
}

func TestFileStore_List(t *testing.T) {
	tmpDir := t.TempDir()
	store := NewFileStore(tmpDir)

	// Create spec directory and files
	specDir := filepath.Join(tmpDir, SpecDir)
	os.MkdirAll(specDir, 0755)
	os.WriteFile(filepath.Join(specDir, "spec1.yaml"), []byte("name: Spec1\nversion: 1.0.0\n"), 0644)
	os.WriteFile(filepath.Join(specDir, "spec2.yaml"), []byte("name: Spec2\nversion: 1.0.0\n"), 0644)

	specs, err := store.List()
	if err != nil {
		t.Fatalf("List() error = %v", err)
	}

	if len(specs) != 2 {
		t.Errorf("List() returned %d specs; want 2", len(specs))
	}
}

func TestFileStore_List_EmptyDir(t *testing.T) {
	tmpDir := t.TempDir()
	store := NewFileStore(tmpDir)

	specs, err := store.List()
	if err != nil {
		t.Fatalf("List() error = %v", err)
	}

	if len(specs) != 0 {
		t.Errorf("List() returned %d specs; want 0", len(specs))
	}
}

func TestFileStore_Delete(t *testing.T) {
	tmpDir := t.TempDir()
	store := NewFileStore(tmpDir)

	// Create spec
	spec := &domain.ProductSpec{
		Name:     "Test",
		Version:  "1.0.0",
		FilePath: "to-delete.yaml",
	}
	store.Save(spec)

	// Delete
	if err := store.Delete(spec.FilePath); err != nil {
		t.Fatalf("Delete() error = %v", err)
	}

	// Verify deleted
	_, err := store.Load(spec.FilePath)
	if err != ErrSpecNotFound {
		t.Error("Load() should return ErrSpecNotFound after delete")
	}
}

func TestFileStore_Delete_NotFound(t *testing.T) {
	tmpDir := t.TempDir()
	store := NewFileStore(tmpDir)

	err := store.Delete("nonexistent.yaml")
	if err != ErrSpecNotFound {
		t.Errorf("Delete() error = %v; want ErrSpecNotFound", err)
	}
}

func TestFileStore_SaveLock_LoadLock(t *testing.T) {
	tmpDir := t.TempDir()
	store := NewFileStore(tmpDir)

	lock := &domain.SpecLock{
		Version:  "1.0.0",
		SpecHash: "abc123",
		Features: map[string]domain.LockedFeature{
			"feat-1": {Hash: "hash1"},
		},
	}

	// Save
	if err := store.SaveLock(lock); err != nil {
		t.Fatalf("SaveLock() error = %v", err)
	}

	// Load
	loaded, err := store.LoadLock()
	if err != nil {
		t.Fatalf("LoadLock() error = %v", err)
	}

	if loaded.Version != lock.Version {
		t.Errorf("Version = %q; want %q", loaded.Version, lock.Version)
	}
	if loaded.SpecHash != lock.SpecHash {
		t.Errorf("SpecHash = %q; want %q", loaded.SpecHash, lock.SpecHash)
	}
}

func TestFileStore_LoadLock_NotFound(t *testing.T) {
	tmpDir := t.TempDir()
	store := NewFileStore(tmpDir)

	_, err := store.LoadLock()
	if err != ErrSpecNotFound {
		t.Errorf("LoadLock() error = %v; want ErrSpecNotFound", err)
	}
}

func TestFileStore_EnsureSpecDir(t *testing.T) {
	tmpDir := t.TempDir()
	store := NewFileStore(tmpDir)

	err := store.EnsureSpecDir()
	if err != nil {
		t.Fatalf("EnsureSpecDir() error = %v", err)
	}

	specDir := filepath.Join(tmpDir, SpecDir)
	if _, err := os.Stat(specDir); os.IsNotExist(err) {
		t.Error("EnsureSpecDir() should create directory")
	}

	// Call again - should not error
	err = store.EnsureSpecDir()
	if err != nil {
		t.Fatalf("Second EnsureSpecDir() error = %v", err)
	}
}

func TestSerializeSpec(t *testing.T) {
	spec := &domain.ProductSpec{
		Name:    "Test Spec",
		Version: "1.0.0",
		Goals:   []string{"Goal 1"},
		Features: []domain.Feature{
			{ID: "feat-1", Title: "Feature 1"},
		},
	}

	content, err := SerializeSpec(spec)
	if err != nil {
		t.Fatalf("SerializeSpec() error = %v", err)
	}

	if len(content) == 0 {
		t.Error("SerializeSpec() returned empty content")
	}
}
