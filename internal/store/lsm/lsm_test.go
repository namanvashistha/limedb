package lsm

import (
	"fmt"
	"sync"
	"testing"
	"time"

	"limedb/internal/store"
)

// Ensure Store implements store.Backend at compile time.
var _ store.Backend = (*Store)(nil)

func TestStore_BasicCRUD(t *testing.T) {
	dir := t.TempDir()
	s, err := NewStore(Config{Dir: dir})
	if err != nil {
		t.Fatalf("NewStore: %v", err)
	}

	// Put
	s.Put("a", val("1"))
	s.Put("b", val("2"))
	s.Put("a", val("3")) // overwrite with newer timestamp

	// Get
	if v, ok := liveGet(s, "a"); !ok || v != "3" {
		t.Errorf("Get(a): got (%q, %v), want (3, true)", v, ok)
	}
	if _, ok := s.Get("c"); ok {
		t.Error("Get(c): should be false")
	}

	// Delete = tombstone Put
	if !s.Put("b", tomb()) {
		t.Error("tombstone Put on existing key should apply")
	}
	if v, ok := s.Get("b"); !ok || !v.Tombstone {
		t.Errorf("Get(b) after delete should surface the tombstone: (%+v, %v)", v, ok)
	}

	// ListKeys / Count exclude tombstones
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

func TestStore_LWW_StaleWriteRejected(t *testing.T) {
	dir := t.TempDir()
	s, err := NewStore(Config{Dir: dir})
	if err != nil {
		t.Fatalf("NewStore: %v", err)
	}
	defer s.Close()

	if !s.Put("k", store.VersionedValue{Value: "new", TimestampMicros: 1000}) {
		t.Fatal("first write should apply")
	}
	// A replica replaying an old write must not clobber the newer value.
	if s.Put("k", store.VersionedValue{Value: "stale", TimestampMicros: 500}) {
		t.Error("stale write should be rejected")
	}
	if v, _ := s.Get("k"); v.Value != "new" {
		t.Errorf("got %q, want new", v.Value)
	}

	// A tombstone older than the value must not delete it.
	if s.Put("k", store.VersionedValue{TimestampMicros: 700, Tombstone: true}) {
		t.Error("stale tombstone should be rejected")
	}
	if v, ok := liveGet(s, "k"); !ok || v != "new" {
		t.Error("value should survive a stale tombstone")
	}

	// A newer tombstone wins; an older value cannot resurrect the key.
	if !s.Put("k", store.VersionedValue{TimestampMicros: 2000, Tombstone: true}) {
		t.Error("newer tombstone should apply")
	}
	if s.Put("k", store.VersionedValue{Value: "zombie", TimestampMicros: 1500}) {
		t.Error("older value should not resurrect a deleted key")
	}
	if _, ok := liveGet(s, "k"); ok {
		t.Error("key should stay deleted")
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
	s.Put("key1", val("val1")) // len=8, under 10
	s.Put("key2", val("val2")) // len=8, totals 16 -> triggers flush

	// Give the async flush / compactor a tiny moment (flush is currently
	// synchronous in Put, but replaceSSTables is fast).
	time.Sleep(50 * time.Millisecond)

	s.mu.Lock()
	tables := len(s.sstables)
	s.mu.Unlock()

	if tables < 1 {
		t.Fatalf("expected at least 1 SSTable after flush, got %d", tables)
	}

	// Value must still be readable (now from SSTable), with LWW still applied.
	if v, ok := liveGet(s, "key1"); !ok || v != "val1" {
		t.Errorf("Get(key1): got (%q, %v)", v, ok)
	}
	if s.Put("key1", store.VersionedValue{Value: "stale", TimestampMicros: 1}) {
		t.Error("stale write should be rejected even when current version is on disk")
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
	s1.Put("k1", val("v1"))
	s1.Put("k2", val("v2"))
	s1.Put("k1", tomb())

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

	if _, ok := liveGet(s2, "k1"); ok {
		t.Error("k1 should be deleted after recovery")
	}
	if v, ok := s2.Get("k1"); !ok || !v.Tombstone {
		t.Errorf("k1 tombstone should survive recovery: (%+v, %v)", v, ok)
	}
	if v, ok := liveGet(s2, "k2"); !ok || v != "v2" {
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
				s.Put(fmt.Sprintf("k-%d-%d", id, j), val("val"))
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
