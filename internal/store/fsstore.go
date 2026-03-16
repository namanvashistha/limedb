package store

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"sync"
)

// FileSystem is a Backend that reads and writes the JSON file on every operation.
// No in-memory caching — every Get/Set/Delete/ListKeys reads from disk first.
// Concurrent access is serialised with a mutex to avoid torn writes.
type FileSystem struct {
	mu       sync.RWMutex
	filePath string
}

// NewFileSystem creates a FileSystem store backed by the given JSON file path.
// Creates the parent directory if needed. No data is loaded into memory.
func NewFileSystem(filePath string) (*FileSystem, error) {
	if err := os.MkdirAll(filepath.Dir(filePath), 0755); err != nil {
		return nil, fmt.Errorf("fsstore: cannot create data dir: %w", err)
	}
	return &FileSystem{filePath: filePath}, nil
}

// Count reads the file and returns the number of keys.
func (s *FileSystem) Count() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	data, _ := s.readFile()
	return len(data)
}

func (s *FileSystem) Get(key string) (string, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	data, err := s.readFile()
	if err != nil {
		return "", false
	}
	val, ok := data[key]
	return val, ok
}

func (s *FileSystem) Set(key, value string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	data, _ := s.readFile() // start from current disk state
	data[key] = value
	_ = s.writeFile(data)
}

func (s *FileSystem) Delete(key string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	data, err := s.readFile()
	if err != nil {
		return false
	}
	_, ok := data[key]
	if ok {
		delete(data, key)
		_ = s.writeFile(data)
	}
	return ok
}

func (s *FileSystem) ListKeys() []string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	data, _ := s.readFile()
	keys := make([]string, 0, len(data))
	for k := range data {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

// readFile opens and decodes the JSON file from disk.
// Returns an empty map if the file does not exist yet.
func (s *FileSystem) readFile() (map[string]string, error) {
	f, err := os.Open(s.filePath)
	if os.IsNotExist(err) {
		return make(map[string]string), nil
	}
	if err != nil {
		fmt.Fprintf(os.Stderr, "fsstore: read error: %v\n", err)
		return make(map[string]string), err
	}
	defer f.Close()

	data := make(map[string]string)
	if err := json.NewDecoder(f).Decode(&data); err != nil {
		fmt.Fprintf(os.Stderr, "fsstore: decode error: %v\n", err)
		return make(map[string]string), err
	}
	return data, nil
}

// writeFile encodes data as indented JSON and atomically replaces the file.
func (s *FileSystem) writeFile(data map[string]string) error {
	tmpPath := s.filePath + ".tmp"

	f, err := os.Create(tmpPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "fsstore: write create error: %v\n", err)
		return err
	}

	enc := json.NewEncoder(f)
	enc.SetIndent("", "  ")
	if encErr := enc.Encode(data); encErr != nil {
		f.Close()
		fmt.Fprintf(os.Stderr, "fsstore: encode error: %v\n", encErr)
		return encErr
	}
	f.Close()

	if err := os.Rename(tmpPath, s.filePath); err != nil {
		fmt.Fprintf(os.Stderr, "fsstore: rename error: %v\n", err)
		return err
	}
	return nil
}

// Stats returns internal storage metrics.
func (s *FileSystem) Stats() map[string]interface{} {
	return map[string]interface{}{
		"type": "fsstore",
	}
}
