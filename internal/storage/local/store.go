package local

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
)

// Store provides thread-safe JSON file storage
type Store struct {
	basePath string
	mu       sync.RWMutex
}

// NewStore creates a new local JSON store
func NewStore(basePath string) (*Store, error) {
	if err := os.MkdirAll(basePath, 0755); err != nil {
		return nil, fmt.Errorf("create store directory: %w", err)
	}
	return &Store{basePath: basePath}, nil
}

// Save persists data to a JSON file
func (s *Store) Save(collection, id string, data interface{}) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	dir := filepath.Join(s.basePath, collection)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("create collection directory: %w", err)
	}

	path := filepath.Join(dir, id+".json")
	file, err := os.Create(path)
	if err != nil {
		return fmt.Errorf("create file: %w", err)
	}
	defer file.Close()

	encoder := json.NewEncoder(file)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(data); err != nil {
		return fmt.Errorf("encode json: %w", err)
	}

	return nil
}

// Load reads data from a JSON file
func (s *Store) Load(collection, id string, data interface{}) error {
	s.mu.RLock()
	defer s.mu.RUnlock()

	path := filepath.Join(s.basePath, collection, id+".json")
	file, err := os.Open(path)
	if err != nil {
		if os.IsNotExist(err) {
			return ErrNotFound
		}
		return fmt.Errorf("open file: %w", err)
	}
	defer file.Close()

	if err := json.NewDecoder(file).Decode(data); err != nil {
		return fmt.Errorf("decode json: %w", err)
	}

	return nil
}

// Delete removes a JSON file
func (s *Store) Delete(collection, id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	path := filepath.Join(s.basePath, collection, id+".json")
	if err := os.Remove(path); err != nil {
		if os.IsNotExist(err) {
			return ErrNotFound
		}
		return fmt.Errorf("remove file: %w", err)
	}

	return nil
}

// List returns all IDs in a collection
func (s *Store) List(collection string) ([]string, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	dir := filepath.Join(s.basePath, collection)
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return []string{}, nil
		}
		return nil, fmt.Errorf("read directory: %w", err)
	}

	var ids []string
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := entry.Name()
		if filepath.Ext(name) == ".json" {
			ids = append(ids, name[:len(name)-5]) // remove .json
		}
	}

	return ids, nil
}

// Exists checks if a record exists
func (s *Store) Exists(collection, id string) bool {
	s.mu.RLock()
	defer s.mu.RUnlock()

	path := filepath.Join(s.basePath, collection, id+".json")
	_, err := os.Stat(path)
	return err == nil
}

// SaveDir saves data to a subdirectory within a collection
func (s *Store) SaveDir(collection, id, subdir, filename string, data interface{}) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	dir := filepath.Join(s.basePath, collection, id, subdir)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("create subdirectory: %w", err)
	}

	path := filepath.Join(dir, filename+".json")
	file, err := os.Create(path)
	if err != nil {
		return fmt.Errorf("create file: %w", err)
	}
	defer file.Close()

	encoder := json.NewEncoder(file)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(data); err != nil {
		return fmt.Errorf("encode json: %w", err)
	}

	return nil
}

// LoadDir loads data from a subdirectory within a collection
func (s *Store) LoadDir(collection, id, subdir, filename string, data interface{}) error {
	s.mu.RLock()
	defer s.mu.RUnlock()

	path := filepath.Join(s.basePath, collection, id, subdir, filename+".json")
	file, err := os.Open(path)
	if err != nil {
		if os.IsNotExist(err) {
			return ErrNotFound
		}
		return fmt.Errorf("open file: %w", err)
	}
	defer file.Close()

	if err := json.NewDecoder(file).Decode(data); err != nil {
		return fmt.Errorf("decode json: %w", err)
	}

	return nil
}

// ListDir lists all files in a subdirectory
func (s *Store) ListDir(collection, id, subdir string) ([]string, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	dir := filepath.Join(s.basePath, collection, id, subdir)
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return []string{}, nil
		}
		return nil, fmt.Errorf("read directory: %w", err)
	}

	var names []string
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := entry.Name()
		if filepath.Ext(name) == ".json" {
			names = append(names, name[:len(name)-5])
		}
	}

	return names, nil
}
