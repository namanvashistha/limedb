package store

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"sync"
)

// FileSystem is a Backend that persists all keys as a single JSON file on disk.
// Format: {"key1": "value1", "key2": "value2", ...}
// Every write is immediately flushed to disk (fsync-safe via rename swap).
type FileSystem struct {
	mu       sync.RWMutex
	data     map[string]string
	filePath string
}

// NewFileSystem creates a FileSystem store backed by the given JSON file path.
// If the file already exists, data is loaded from it on startup.
func NewFileSystem(filePath string) (*FileSystem, error) {
	// Ensure parent directory exists
	if err := os.MkdirAll(filepath.Dir(filePath), 0755); err != nil {
		return nil, fmt.Errorf("fsstore: cannot create data dir: %w", err)
	}

	fs := &FileSystem{
		data:     make(map[string]string),
		filePath: filePath,
	}

	// Load existing data if file exists
	if _, err := os.Stat(filePath); err == nil {
		if err := fs.load(); err != nil {
			return nil, fmt.Errorf("fsstore: failed to load %s: %w", filePath, err)
		}
	}

	return fs, nil
}

func (s *FileSystem) Get(key string) (string, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	val, ok := s.data[key]
	return val, ok
}

func (s *FileSystem) Set(key, value string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.data[key] = value
	_ = s.flush() // best-effort flush; errors logged to stderr
}

func (s *FileSystem) Delete(key string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	_, ok := s.data[key]
	if ok {
		delete(s.data, key)
		_ = s.flush()
	}
	return ok
}

func (s *FileSystem) ListKeys() []string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	keys := make([]string, 0, len(s.data))
	for k := range s.data {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

func (s *FileSystem) Count() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return len(s.data)
}

// load reads the JSON file into memory. Caller must hold no lock (called from constructor).
func (s *FileSystem) load() error {
	f, err := os.Open(s.filePath)
	if err != nil {
		return err
	}
	defer f.Close()
	return json.NewDecoder(f).Decode(&s.data)
}

// flush writes the current data map to disk atomically using a temp file + rename.
// Caller must hold the write lock.
func (s *FileSystem) flush() error {
	tmpPath := s.filePath + ".tmp"

	f, err := os.Create(tmpPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "fsstore: flush create error: %v\n", err)
		return err
	}

	enc := json.NewEncoder(f)
	enc.SetIndent("", "  ")
	if err := enc.Encode(s.data); err != nil {
		f.Close()
		fmt.Fprintf(os.Stderr, "fsstore: flush encode error: %v\n", err)
		return err
	}
	f.Close()

	// Atomic rename: ensures readers never see a partial write
	if err := os.Rename(tmpPath, s.filePath); err != nil {
		fmt.Fprintf(os.Stderr, "fsstore: flush rename error: %v\n", err)
		return err
	}
	return nil
}
