package local

import (
	"os"
	"path/filepath"
	"sync"
	"testing"
)

func TestNewStore(t *testing.T) {
	// Create temp directory
	tmpDir := t.TempDir()

	store, err := NewStore(tmpDir)
	if err != nil {
		t.Fatalf("NewStore() error = %v", err)
	}

	if store == nil {
		t.Fatal("NewStore() returned nil")
	}

	if store.basePath != tmpDir {
		t.Errorf("basePath = %v, want %v", store.basePath, tmpDir)
	}
}

func TestNewStore_CreatesDirectory(t *testing.T) {
	tmpDir := t.TempDir()
	newDir := filepath.Join(tmpDir, "subdir", "nested")

	store, err := NewStore(newDir)
	if err != nil {
		t.Fatalf("NewStore() error = %v", err)
	}

	if store == nil {
		t.Fatal("NewStore() returned nil")
	}

	// Verify directory was created
	info, err := os.Stat(newDir)
	if err != nil {
		t.Fatalf("directory not created: %v", err)
	}

	if !info.IsDir() {
		t.Error("expected directory, got file")
	}
}

func TestStore_Save_Load(t *testing.T) {
	tmpDir := t.TempDir()
	store, _ := NewStore(tmpDir)

	type testData struct {
		Name  string `json:"name"`
		Value int    `json:"value"`
	}

	original := testData{Name: "test", Value: 42}

	// Save
	err := store.Save("collection", "item1", original)
	if err != nil {
		t.Fatalf("Save() error = %v", err)
	}

	// Load
	var loaded testData
	err = store.Load("collection", "item1", &loaded)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	if loaded.Name != original.Name {
		t.Errorf("Name = %v, want %v", loaded.Name, original.Name)
	}
	if loaded.Value != original.Value {
		t.Errorf("Value = %v, want %v", loaded.Value, original.Value)
	}
}

func TestStore_Load_NotFound(t *testing.T) {
	tmpDir := t.TempDir()
	store, _ := NewStore(tmpDir)

	var data struct{}
	err := store.Load("collection", "nonexistent", &data)

	if err != ErrNotFound {
		t.Errorf("Load() error = %v, want ErrNotFound", err)
	}
}

func TestStore_Delete(t *testing.T) {
	tmpDir := t.TempDir()
	store, _ := NewStore(tmpDir)

	data := map[string]string{"key": "value"}

	// Save first
	store.Save("collection", "to-delete", data)

	// Delete
	err := store.Delete("collection", "to-delete")
	if err != nil {
		t.Fatalf("Delete() error = %v", err)
	}

	// Verify deletion
	err = store.Load("collection", "to-delete", &data)
	if err != ErrNotFound {
		t.Error("Load() should return ErrNotFound after deletion")
	}
}

func TestStore_Delete_NotFound(t *testing.T) {
	tmpDir := t.TempDir()
	store, _ := NewStore(tmpDir)

	err := store.Delete("collection", "nonexistent")
	if err != ErrNotFound {
		t.Errorf("Delete() error = %v, want ErrNotFound", err)
	}
}

func TestStore_List(t *testing.T) {
	tmpDir := t.TempDir()
	store, _ := NewStore(tmpDir)

	data := map[string]string{"key": "value"}

	// Save multiple items
	store.Save("items", "a", data)
	store.Save("items", "b", data)
	store.Save("items", "c", data)

	ids, err := store.List("items")
	if err != nil {
		t.Fatalf("List() error = %v", err)
	}

	if len(ids) != 3 {
		t.Errorf("List() returned %d items, want 3", len(ids))
	}

	// Check all IDs are present
	found := make(map[string]bool)
	for _, id := range ids {
		found[id] = true
	}

	for _, expected := range []string{"a", "b", "c"} {
		if !found[expected] {
			t.Errorf("List() missing ID %q", expected)
		}
	}
}

func TestStore_List_EmptyCollection(t *testing.T) {
	tmpDir := t.TempDir()
	store, _ := NewStore(tmpDir)

	ids, err := store.List("empty-collection")
	if err != nil {
		t.Fatalf("List() error = %v", err)
	}

	if len(ids) != 0 {
		t.Errorf("List() returned %d items, want 0", len(ids))
	}
}

func TestStore_Exists(t *testing.T) {
	tmpDir := t.TempDir()
	store, _ := NewStore(tmpDir)

	data := map[string]string{"key": "value"}

	// Before save
	if store.Exists("collection", "item") {
		t.Error("Exists() should return false before save")
	}

	// After save
	store.Save("collection", "item", data)
	if !store.Exists("collection", "item") {
		t.Error("Exists() should return true after save")
	}

	// After delete
	store.Delete("collection", "item")
	if store.Exists("collection", "item") {
		t.Error("Exists() should return false after delete")
	}
}

func TestStore_SaveDir_LoadDir(t *testing.T) {
	tmpDir := t.TempDir()
	store, _ := NewStore(tmpDir)

	type nested struct {
		Data string `json:"data"`
	}

	original := nested{Data: "nested data"}

	// Save to subdirectory
	err := store.SaveDir("collection", "parent", "subdir", "file", original)
	if err != nil {
		t.Fatalf("SaveDir() error = %v", err)
	}

	// Load from subdirectory
	var loaded nested
	err = store.LoadDir("collection", "parent", "subdir", "file", &loaded)
	if err != nil {
		t.Fatalf("LoadDir() error = %v", err)
	}

	if loaded.Data != original.Data {
		t.Errorf("Data = %v, want %v", loaded.Data, original.Data)
	}
}

func TestStore_LoadDir_NotFound(t *testing.T) {
	tmpDir := t.TempDir()
	store, _ := NewStore(tmpDir)

	var data struct{}
	err := store.LoadDir("collection", "parent", "subdir", "nonexistent", &data)

	if err != ErrNotFound {
		t.Errorf("LoadDir() error = %v, want ErrNotFound", err)
	}
}

func TestStore_ListDir(t *testing.T) {
	tmpDir := t.TempDir()
	store, _ := NewStore(tmpDir)

	data := map[string]string{"key": "value"}

	// Save multiple files in subdirectory
	store.SaveDir("collection", "parent", "versions", "v1", data)
	store.SaveDir("collection", "parent", "versions", "v2", data)
	store.SaveDir("collection", "parent", "versions", "v3", data)

	names, err := store.ListDir("collection", "parent", "versions")
	if err != nil {
		t.Fatalf("ListDir() error = %v", err)
	}

	if len(names) != 3 {
		t.Errorf("ListDir() returned %d items, want 3", len(names))
	}
}

func TestStore_ListDir_Empty(t *testing.T) {
	tmpDir := t.TempDir()
	store, _ := NewStore(tmpDir)

	names, err := store.ListDir("collection", "parent", "empty")
	if err != nil {
		t.Fatalf("ListDir() error = %v", err)
	}

	if len(names) != 0 {
		t.Errorf("ListDir() returned %d items, want 0", len(names))
	}
}

func TestStore_Concurrency(t *testing.T) {
	tmpDir := t.TempDir()
	store, _ := NewStore(tmpDir)

	var wg sync.WaitGroup
	iterations := 10

	// Concurrent writes
	for i := 0; i < iterations; i++ {
		wg.Add(1)
		go func(n int) {
			defer wg.Done()
			data := map[string]int{"value": n}
			store.Save("concurrent", string(rune('a'+n)), data)
		}(i)
	}

	// Concurrent reads
	for i := 0; i < iterations; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			store.List("concurrent")
		}()
	}

	// Concurrent existence checks
	for i := 0; i < iterations; i++ {
		wg.Add(1)
		go func(n int) {
			defer wg.Done()
			store.Exists("concurrent", string(rune('a'+n)))
		}(i)
	}

	wg.Wait()

	// If we get here without deadlock or panic, concurrency is handled
}

func TestStore_ComplexData(t *testing.T) {
	tmpDir := t.TempDir()
	store, _ := NewStore(tmpDir)

	type complexData struct {
		Name     string            `json:"name"`
		Age      int               `json:"age"`
		Tags     []string          `json:"tags"`
		Metadata map[string]string `json:"metadata"`
		Nested   struct {
			Value float64 `json:"value"`
		} `json:"nested"`
	}

	original := complexData{
		Name: "Test",
		Age:  30,
		Tags: []string{"a", "b", "c"},
		Metadata: map[string]string{
			"key1": "value1",
			"key2": "value2",
		},
	}
	original.Nested.Value = 3.14

	store.Save("complex", "item", original)

	var loaded complexData
	store.Load("complex", "item", &loaded)

	if loaded.Name != original.Name {
		t.Errorf("Name = %v, want %v", loaded.Name, original.Name)
	}
	if loaded.Age != original.Age {
		t.Errorf("Age = %v, want %v", loaded.Age, original.Age)
	}
	if len(loaded.Tags) != len(original.Tags) {
		t.Errorf("Tags length = %v, want %v", len(loaded.Tags), len(original.Tags))
	}
	if loaded.Nested.Value != original.Nested.Value {
		t.Errorf("Nested.Value = %v, want %v", loaded.Nested.Value, original.Nested.Value)
	}
}

func TestStore_Overwrite(t *testing.T) {
	tmpDir := t.TempDir()
	store, _ := NewStore(tmpDir)

	type data struct {
		Value int `json:"value"`
	}

	// Save initial
	store.Save("collection", "item", data{Value: 1})

	// Overwrite
	store.Save("collection", "item", data{Value: 2})

	// Load and verify overwrite
	var loaded data
	store.Load("collection", "item", &loaded)

	if loaded.Value != 2 {
		t.Errorf("Value = %v, want 2 (overwritten)", loaded.Value)
	}
}
