package node

import (
	"sort"
	"sync"
)

// Store is a thread-safe in-memory key-value store.
type Store struct {
	data sync.Map
}

// NewStore creates a new in-memory store.
func NewStore() *Store {
	return &Store{}
}

// Get retrieves a value for a key.
func (s *Store) Get(key string) (string, bool) {
	val, ok := s.data.Load(key)
	if !ok {
		return "", false
	}
	return val.(string), true
}

// Set sets a value for a key.
func (s *Store) Set(key, value string) {
	s.data.Store(key, value)
}

// Delete removes a key.
func (s *Store) Delete(key string) bool {
	_, ok := s.data.LoadAndDelete(key)
	return ok
}

// ListKeys returns all keys in sorted order.
func (s *Store) ListKeys() []string {
	keys := make([]string, 0)
	s.data.Range(func(key, value interface{}) bool {
		keys = append(keys, key.(string))
		return true
	})
	sort.Strings(keys)
	return keys
}

// Count returns the total number of keys.
func (s *Store) Count() int {
	count := 0
	s.data.Range(func(key, value interface{}) bool {
		count++
		return true
	})
	return count
}
