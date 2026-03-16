package lsm

import (
	"fmt"
	"sync"
	"testing"
)

func TestMemTable_SetAndGet(t *testing.T) {
	m := NewMemTable()

	m.Set("color", "lime")
	m.Set("fruit", "mango")
	m.Set("color", "green") // overwrite

	if v, ok := m.Get("color"); !ok || v != "green" {
		t.Errorf("color: got (%q, %v), want (green, true)", v, ok)
	}
	if v, ok := m.Get("fruit"); !ok || v != "mango" {
		t.Errorf("fruit: got (%q, %v), want (mango, true)", v, ok)
	}
	if _, ok := m.Get("missing"); ok {
		t.Error("missing key should return false")
	}
}

func TestMemTable_Delete_Tombstone(t *testing.T) {
	m := NewMemTable()
	m.Set("x", "hello")
	m.Delete("x")

	if _, ok := m.Get("x"); ok {
		t.Error("Get after Delete should return false")
	}
	if !m.IsDeleted("x") {
		t.Error("IsDeleted should return true after Delete")
	}
	// Tombstone still makes Has() return true (needed for SSTable short-circuit).
	if !m.Has("x") {
		t.Error("Has should return true for a tombstoned key")
	}
}

func TestMemTable_DeleteNeverWritten(t *testing.T) {
	m := NewMemTable()
	// Deleting a key that was never written should still write a tombstone.
	m.Delete("ghost")
	if !m.IsDeleted("ghost") {
		t.Error("IsDeleted should be true even for never-written key")
	}
}

func TestMemTable_SetAfterDelete(t *testing.T) {
	m := NewMemTable()
	m.Set("k", "v1")
	m.Delete("k")
	m.Set("k", "v2") // resurrection

	v, ok := m.Get("k")
	if !ok || v != "v2" {
		t.Errorf("after resurrection: got (%q, %v), want (v2, true)", v, ok)
	}
	if m.IsDeleted("k") {
		t.Error("key should not be deleted after Set")
	}
}

func TestMemTable_Entries_SortedAndTombstones(t *testing.T) {
	m := NewMemTable()
	m.Set("banana", "yellow")
	m.Set("apple", "red")
	m.Set("cherry", "dark")
	m.Delete("apple")

	entries := m.Entries()
	if len(entries) != 3 {
		t.Fatalf("expected 3 entries, got %d", len(entries))
	}

	// Must be sorted by key.
	keys := []string{entries[0].Key, entries[1].Key, entries[2].Key}
	if keys[0] != "apple" || keys[1] != "banana" || keys[2] != "cherry" {
		t.Errorf("wrong order: %v", keys)
	}

	// apple should be flagged as deleted.
	if !entries[0].Deleted {
		t.Error("apple entry should be Deleted=true")
	}
	if entries[1].Deleted || entries[2].Deleted {
		t.Error("banana/cherry should not be Deleted")
	}
}

func TestMemTable_ApproximateSize(t *testing.T) {
	m := NewMemTable()
	if m.ApproximateSize() != 0 {
		t.Errorf("empty size should be 0, got %d", m.ApproximateSize())
	}

	m.Set("foo", "bar") // +3+3 = 6
	if m.ApproximateSize() < 6 {
		t.Errorf("size should be >= 6, got %d", m.ApproximateSize())
	}

	prev := m.ApproximateSize()
	m.Set("foo", "longervalue") // value grows
	if m.ApproximateSize() <= prev {
		t.Error("size should grow when value is replaced with a longer one")
	}
}

func TestMemTable_RestoreFromWAL(t *testing.T) {
	records := []Record{
		{Op: OpSet, Key: "a", Value: "1"},
		{Op: OpSet, Key: "b", Value: "2"},
		{Op: OpDel, Key: "a"},
		{Op: OpSet, Key: "c", Value: "3"},
	}

	m := NewMemTable()
	m.RestoreFromWAL(records)

	if _, ok := m.Get("a"); ok {
		t.Error("a should be deleted after WAL replay")
	}
	if v, ok := m.Get("b"); !ok || v != "2" {
		t.Errorf("b: got (%q, %v), want (2, true)", v, ok)
	}
	if v, ok := m.Get("c"); !ok || v != "3" {
		t.Errorf("c: got (%q, %v), want (3, true)", v, ok)
	}
	if m.Len() != 3 { // a (tombstone) + b + c
		t.Errorf("Len: got %d, want 3", m.Len())
	}
}

func TestMemTable_Concurrent(t *testing.T) {
	m := NewMemTable()
	var wg sync.WaitGroup

	// Writers.
	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			for j := 0; j < 100; j++ {
				m.Set(fmt.Sprintf("key-%d-%d", id, j), fmt.Sprintf("v%d", j))
			}
		}(i)
	}
	// Readers running concurrently.
	for i := 0; i < 5; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < 200; j++ {
				m.Get("key-0-0")
				m.ApproximateSize()
			}
		}()
	}

	wg.Wait()
	// 20 goroutines × 100 writes each.
	if m.Len() != 2000 {
		t.Errorf("Len: got %d, want 2000", m.Len())
	}
}
