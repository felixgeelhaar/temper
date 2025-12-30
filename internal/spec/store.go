package spec

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/felixgeelhaar/temper/internal/domain"
	"gopkg.in/yaml.v3"
)

const (
	// SpecDir is the default directory for specs within a workspace
	SpecDir = ".specs"
	// LockFile is the name of the lock file
	LockFile = "spec.lock"
)

var (
	ErrSpecNotFound = errors.New("spec not found")
	ErrInvalidPath  = errors.New("invalid spec path")
)

// FileStore manages specs in the .specs/ directory
type FileStore struct {
	basePath string // workspace root
}

// NewFileStore creates a new file store for a workspace
func NewFileStore(basePath string) *FileStore {
	return &FileStore{basePath: basePath}
}

// SpecPath returns the full path to a spec file
func (s *FileStore) SpecPath(relativePath string) string {
	// If path already starts with .specs/, use it directly
	if strings.HasPrefix(relativePath, SpecDir) {
		return filepath.Join(s.basePath, relativePath)
	}
	// Otherwise, prepend .specs/
	return filepath.Join(s.basePath, SpecDir, relativePath)
}

// Load reads and parses a spec from the given path
func (s *FileStore) Load(path string) (*domain.ProductSpec, error) {
	fullPath := s.SpecPath(path)

	content, err := os.ReadFile(fullPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, ErrSpecNotFound
		}
		return nil, fmt.Errorf("read spec file: %w", err)
	}

	spec, err := ParseSpec(content)
	if err != nil {
		return nil, fmt.Errorf("parse spec: %w", err)
	}

	spec.FilePath = path

	// Get file timestamps
	info, err := os.Stat(fullPath)
	if err == nil {
		spec.UpdatedAt = info.ModTime()
	}

	return spec, nil
}

// Save writes a spec to the given path
func (s *FileStore) Save(spec *domain.ProductSpec) error {
	if spec.FilePath == "" {
		return ErrInvalidPath
	}

	fullPath := s.SpecPath(spec.FilePath)

	// Ensure directory exists
	dir := filepath.Dir(fullPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("create spec directory: %w", err)
	}

	content, err := SerializeSpec(spec)
	if err != nil {
		return fmt.Errorf("serialize spec: %w", err)
	}

	if err := os.WriteFile(fullPath, content, 0644); err != nil {
		return fmt.Errorf("write spec file: %w", err)
	}

	spec.UpdatedAt = time.Now()
	return nil
}

// List returns all specs in the .specs/ directory
func (s *FileStore) List() ([]*domain.ProductSpec, error) {
	specsDir := filepath.Join(s.basePath, SpecDir)

	// Check if .specs/ exists
	if _, err := os.Stat(specsDir); os.IsNotExist(err) {
		return []*domain.ProductSpec{}, nil
	}

	var specs []*domain.ProductSpec

	err := filepath.Walk(specsDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		// Skip directories and non-yaml files
		if info.IsDir() {
			return nil
		}

		ext := strings.ToLower(filepath.Ext(path))
		if ext != ".yaml" && ext != ".yml" {
			return nil
		}

		// Skip lock files
		if strings.HasSuffix(path, LockFile) {
			return nil
		}

		// Get relative path from workspace root
		relPath, err := filepath.Rel(s.basePath, path)
		if err != nil {
			return nil // skip files we can't resolve
		}

		spec, err := s.Load(relPath)
		if err != nil {
			// Log but don't fail on individual spec errors
			return nil
		}

		specs = append(specs, spec)
		return nil
	})

	if err != nil {
		return nil, fmt.Errorf("walk specs directory: %w", err)
	}

	return specs, nil
}

// Delete removes a spec file
func (s *FileStore) Delete(path string) error {
	fullPath := s.SpecPath(path)
	if err := os.Remove(fullPath); err != nil {
		if os.IsNotExist(err) {
			return ErrSpecNotFound
		}
		return fmt.Errorf("delete spec file: %w", err)
	}
	return nil
}

// LoadLock reads the spec.lock file
func (s *FileStore) LoadLock() (*domain.SpecLock, error) {
	lockPath := filepath.Join(s.basePath, SpecDir, LockFile)

	content, err := os.ReadFile(lockPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, ErrSpecNotFound
		}
		return nil, fmt.Errorf("read lock file: %w", err)
	}

	var lock domain.SpecLock
	if err := json.Unmarshal(content, &lock); err != nil {
		return nil, fmt.Errorf("parse lock file: %w", err)
	}

	return &lock, nil
}

// SaveLock writes the spec.lock file
func (s *FileStore) SaveLock(lock *domain.SpecLock) error {
	specsDir := filepath.Join(s.basePath, SpecDir)
	if err := os.MkdirAll(specsDir, 0755); err != nil {
		return fmt.Errorf("create specs directory: %w", err)
	}

	lockPath := filepath.Join(specsDir, LockFile)

	content, err := json.MarshalIndent(lock, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal lock: %w", err)
	}

	if err := os.WriteFile(lockPath, content, 0644); err != nil {
		return fmt.Errorf("write lock file: %w", err)
	}

	return nil
}

// EnsureSpecDir creates the .specs/ directory if it doesn't exist
func (s *FileStore) EnsureSpecDir() error {
	specsDir := filepath.Join(s.basePath, SpecDir)
	return os.MkdirAll(specsDir, 0755)
}

// ParseSpec parses a spec from YAML content
func ParseSpec(content []byte) (*domain.ProductSpec, error) {
	var spec domain.ProductSpec
	if err := yaml.Unmarshal(content, &spec); err != nil {
		return nil, fmt.Errorf("unmarshal yaml: %w", err)
	}
	return &spec, nil
}

// SerializeSpec serializes a spec to YAML
func SerializeSpec(spec *domain.ProductSpec) ([]byte, error) {
	return yaml.Marshal(spec)
}
