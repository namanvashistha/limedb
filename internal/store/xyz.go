package store

import (
	"fmt"
	"sort"
	"sync"
)

// XYZ is a sample Backend implementation that wraps a plain map with RWMutex.
// Useful as a reference for implementing new backends.
type XYZ struct {
	mu   sync.RWMutex
	data map[string]string
	name string
}

// NewXYZ creates a new XYZStore.
func NewXYZ() *XYZ {
	return &XYZ{
		data: make(map[string]string),
		name: "xyzstore",
	}
}

func (s *XYZ) Get(key string) (string, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	val, ok := s.data[key]
	fmt.Printf("[%s] GET %q → %q (found=%v)\n", s.name, key, val, ok)
	return val, ok
}

func (s *XYZ) Set(key, value string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	fmt.Printf("[%s] SET %q = %q\n", s.name, key, value)
	s.data[key] = value
}

func (s *XYZ) Delete(key string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	_, ok := s.data[key]
	if ok {
		delete(s.data, key)
		fmt.Printf("[%s] DELETE %q\n", s.name, key)
	}
	return ok
}

func (s *XYZ) ListKeys() []string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	keys := make([]string, 0, len(s.data))
	for k := range s.data {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

func (s *XYZ) Count() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return len(s.data)
}
