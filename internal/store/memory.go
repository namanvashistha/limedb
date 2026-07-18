package store

import (
	"sort"
	"sync"
)

// Memory is a thread-safe in-memory key-value store.
// Tombstones are kept in the map so LWW comparisons work after deletes.
type Memory struct {
	mu   sync.RWMutex
	data map[string]VersionedValue
}

// NewMemory creates a new in-memory store.
func NewMemory() *Memory {
	return &Memory{data: make(map[string]VersionedValue)}
}

func (s *Memory) Get(key string) (VersionedValue, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	v, ok := s.data[key]
	return v, ok
}

func (s *Memory) Put(key string, v VersionedValue) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	if cur, ok := s.data[key]; ok && !Newer(v, cur) {
		return false
	}
	s.data[key] = v
	return true
}

func (s *Memory) ListKeys() []string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	keys := make([]string, 0, len(s.data))
	for k, v := range s.data {
		if !v.Tombstone {
			keys = append(keys, k)
		}
	}
	sort.Strings(keys)
	return keys
}

// Stats returns internal storage metrics.
func (s *Memory) Stats() map[string]interface{} {
	return map[string]interface{}{
		"type": "memory",
		"keys": s.Count(),
	}
}

func (s *Memory) Count() int {
	return len(s.ListKeys())
}
