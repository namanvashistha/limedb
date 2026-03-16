package lsm

import (
	"fmt"
	"limedb/internal/store"
	"sync"
	"testing"
	"time"
)

// Ensure Store implements store.Backend at compile time.
var _ store.Backend = (*Store)(nil)

func TestStore_BasicCRUD(t *testing.T) {
	dir := t.TempDir()
	s, err := NewStore(Config{Dir: dir})
	if err != nil {
		t.Fatalf("NewStore: %v", err)
	}

	// Set
	s.Set("a", "1")
	s.Set("b", "2")
	s.Set("a", "3") // overwrite

	// Get
	if v, ok := s.Get("a"); !ok || v != "3" {
		t.Errorf("Get(a): got (%q, %v), want (3, true)", v, ok)
	}
	if _, ok := s.Get("c"); ok {
		t.Error("Get(c): should be false")
	}

	// Delete
	if !s.Delete("b") {
		t.Error("Delete(b) should return true (existed)")
	}
	if s.Delete("c") {
		t.Error("Delete(c) should return false (didn't exist)")
	}
	if _, ok := s.Get("b"); ok {
		t.Error("Get(b) after delete should be false")
	}

	// ListKeys / Count
	keys := s.ListKeys()
	if len(keys) != 1 || keys[0] != "a" {
		t.Errorf("ListKeys: got %v, want [a]", keys)
	}
	if s.Count() != 1 {
		t.Errorf("Count: got %d, want 1", s.Count())
	}

	if err := s.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}
}

func TestStore_FlushAndReadFromSSTable(t *testing.T) {
	dir := t.TempDir()
	s, err := NewStore(Config{
		Dir:                dir,
		MemTableFlushBytes: 10, // tiny threshold to force flush immediately
	})
	if err != nil {
		t.Fatalf("NewStore: %v", err)
	}

	// Write enough to trigger a flush.
	s.Set("key1", "val1") // len=8, under 10
	s.Set("key2", "val2") // len=8, totals 16 -> triggers flush

	// Give the async flush / compactor a tiny moment (flush is currently
	// synchronous in Set/Delete, but replaceSSTables is fast).
	time.Sleep(50 * time.Millisecond)

	s.mu.Lock()
	tables := len(s.sstables)
	s.mu.Unlock()

	if tables < 1 {
		t.Fatalf("expected at least 1 SSTable after flush, got %d", tables)
	}

	// Value must still be readable (now from SSTable).
	if v, ok := s.Get("key1"); !ok || v != "val1" {
		t.Errorf("Get(key1): got (%q, %v)", v, ok)
	}

	_ = s.Close()
}

func TestStore_WALRecovery(t *testing.T) {
	dir := t.TempDir()

	// 1. Write some data and crash without flushing.
	s1, err := NewStore(Config{Dir: dir, MemTableFlushBytes: 1000000})
	if err != nil {
		t.Fatalf("NewStore 1: %v", err)
	}
	s1.Set("k1", "v1")
	s1.Set("k2", "v2")
	s1.Delete("k1")

	// Hard close (don't call graceful Close() which would flush the MemTable).
	s1.wal.f.Close()
	s1.closeAllSSTables()
	s1.compactor.Stop()

	// 2. Restart engine. It must read the WAL.
	s2, err := NewStore(Config{Dir: dir, MemTableFlushBytes: 1000000})
	if err != nil {
		t.Fatalf("NewStore 2: %v", err)
	}
	defer s2.Close()

	if _, ok := s2.Get("k1"); ok {
		t.Error("k1 should be deleted after recovery")
	}
	if v, ok := s2.Get("k2"); !ok || v != "v2" {
		t.Errorf("k2: got (%q, %v), want (v2, true) after recovery", v, ok)
	}
}

func TestStore_ConcurrentAccess(t *testing.T) {
	dir := t.TempDir()
	// Force frequent flushes.
	s, err := NewStore(Config{Dir: dir, MemTableFlushBytes: 50})
	if err != nil {
		t.Fatalf("NewStore: %v", err)
	}
	defer s.Close()

	var wg sync.WaitGroup
	const writers = 10
	const writes = 100

	// Concurrent writes.
	for i := 0; i < writers; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			for j := 0; j < writes; j++ {
				s.Set(fmt.Sprintf("k-%d-%d", id, j), "val")
				s.Get(fmt.Sprintf("k-%d-%d", id, j))
			}
		}(i)
	}

	// Concurrent compactor trigger (simulated).
	wg.Add(1)
	go func() {
		defer wg.Done()
		for i := 0; i < 50; i++ {
			s.ListKeys()
			time.Sleep(2 * time.Millisecond)
		}
	}()

	wg.Wait()
	if count := s.Count(); count != writers*writes {
		t.Errorf("Count: got %d, want %d", count, writers*writes)
	}
}
