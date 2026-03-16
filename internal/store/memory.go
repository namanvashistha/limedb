package store

import "sync"

// Memory is a thread-safe in-memory key-value store backed by sync.Map.
type Memory struct {
	data sync.Map
}

// NewMemory creates a new in-memory store.
func NewMemory() *Memory {
	return &Memory{}
}

func (s *Memory) Get(key string) (string, bool) {
	val, ok := s.data.Load(key)
	if !ok {
		return "", false
	}
	return val.(string), true
}

func (s *Memory) Set(key, value string) {
	s.data.Store(key, value)
}

func (s *Memory) Delete(key string) bool {
	_, ok := s.data.LoadAndDelete(key)
	return ok
}

func (s *Memory) ListKeys() []string {
	keys := make([]string, 0)
	s.data.Range(func(key, value interface{}) bool {
		keys = append(keys, key.(string))
		return true
	})
	return keys
}

// Stats returns internal storage metrics.
func (m *Memory) Stats() map[string]interface{} {
	return map[string]interface{}{
		"type": "memory",
		"keys": m.Count(),
	}
}

func (s *Memory) Count() int {
	count := 0
	s.data.Range(func(key, value interface{}) bool {
		count++
		return true
	})
	return count
}
