package lsm

import (
	"fmt"
	"path/filepath"
	"testing"
)

// writeTestSSTable writes a sorted slice of entries to a temp .sst file
// and returns a reader opened on it.
func writeTestSSTable(t *testing.T, entries []Entry) *SSTableReader {
	t.Helper()
	path := filepath.Join(t.TempDir(), "test.sst")

	w, err := NewSSTableWriter(path)
	if err != nil {
		t.Fatalf("NewSSTableWriter: %v", err)
	}
	for _, e := range entries {
		if err := w.WriteEntry(e); err != nil {
			t.Fatalf("WriteEntry %q: %v", e.Key, err)
		}
	}
	if err := w.Flush(); err != nil {
		t.Fatalf("Flush: %v", err)
	}

	r, err := OpenSSTable(path)
	if err != nil {
		t.Fatalf("OpenSSTable: %v", err)
	}
	t.Cleanup(func() { _ = r.Close() })
	return r
}

// TestSSTable_BasicGetHit writes a few entries then looks each one up.
func TestSSTable_BasicGetHit(t *testing.T) {
	entries := []Entry{
		{Key: "apple", Value: "red"},
		{Key: "banana", Value: "yellow"},
		{Key: "cherry", Value: "dark-red"},
		{Key: "durian", Value: "spiky"},
	}
	r := writeTestSSTable(t, entries)

	for _, e := range entries {
		v, ok := r.Get(e.Key)
		if !ok || v != e.Value {
			t.Errorf("Get(%q): got (%q, %v), want (%q, true)", e.Key, v, ok, e.Value)
		}
	}
}

// TestSSTable_GetMiss verifies Get returns false for absent keys.
func TestSSTable_GetMiss(t *testing.T) {
	r := writeTestSSTable(t, []Entry{
		{Key: "a", Value: "1"},
		{Key: "c", Value: "3"},
	})

	for _, key := range []string{"b", "d", "z", ""} {
		if _, ok := r.Get(key); ok {
			t.Errorf("Get(%q) should be false (key absent)", key)
		}
	}
}

// TestSSTable_Tombstone verifies that tombstoned keys return (false) from Get
// but true from IsTombstone.
func TestSSTable_Tombstone(t *testing.T) {
	r := writeTestSSTable(t, []Entry{
		{Key: "alive", Value: "yes"},
		{Key: "dead", Deleted: true},
	})

	if _, ok := r.Get("dead"); ok {
		t.Error("Get(tombstone key) should return false")
	}
	if !r.IsTombstone("dead") {
		t.Error("IsTombstone should return true for deleted entry")
	}
	if r.IsTombstone("alive") {
		t.Error("IsTombstone should return false for live entry")
	}
}

// TestSSTable_Entries verifies the full scan returns entries in order,
// including tombstones.
func TestSSTable_Entries(t *testing.T) {
	src := []Entry{
		{Key: "a", Value: "1"},
		{Key: "b", Deleted: true},
		{Key: "c", Value: "3"},
	}
	r := writeTestSSTable(t, src)

	got, err := r.Entries()
	if err != nil {
		t.Fatalf("Entries: %v", err)
	}
	if len(got) != len(src) {
		t.Fatalf("len(Entries): got %d, want %d", len(got), len(src))
	}
	for i, e := range src {
		if got[i].Key != e.Key || got[i].Deleted != e.Deleted {
			t.Errorf("entry[%d]: got {%q,%v}, want {%q,%v}",
				i, got[i].Key, got[i].Deleted, e.Key, e.Deleted)
		}
	}
}

// TestSSTable_SparseIndex exercises lookups across multiple index blocks.
// We write more than indexInterval entries so the sparse index has >1 entry.
func TestSSTable_SparseIndex(t *testing.T) {
	const n = indexInterval*3 + 7 // 55 entries ⇒ 4 index blocks

	entries := make([]Entry, n)
	for i := range entries {
		entries[i] = Entry{
			Key:   fmt.Sprintf("key-%04d", i),
			Value: fmt.Sprintf("val-%d", i),
		}
	}
	r := writeTestSSTable(t, entries)

	// Spot-check a few entries spread across index blocks.
	for _, i := range []int{0, 15, 16, 31, 32, n - 1} {
		want := entries[i]
		v, ok := r.Get(want.Key)
		if !ok || v != want.Value {
			t.Errorf("Get(%q): got (%q, %v), want (%q, true)", want.Key, v, ok, want.Value)
		}
	}
}

// TestSSTable_FlushFromMemTable verifies the end-to-end path:
// MemTable → Entries() → SSTableWriter → SSTableReader.Get
func TestSSTable_FlushFromMemTable(t *testing.T) {
	m := NewMemTable()
	m.Set("x", "10")
	m.Set("y", "20")
	m.Set("z", "30")
	m.Delete("y")

	path := filepath.Join(t.TempDir(), "flush.sst")
	w, err := NewSSTableWriter(path)
	if err != nil {
		t.Fatalf("NewSSTableWriter: %v", err)
	}
	for _, e := range m.Entries() {
		if err := w.WriteEntry(e); err != nil {
			t.Fatalf("WriteEntry: %v", err)
		}
	}
	if err := w.Flush(); err != nil {
		t.Fatalf("Flush: %v", err)
	}

	r, err := OpenSSTable(path)
	if err != nil {
		t.Fatalf("OpenSSTable: %v", err)
	}
	defer r.Close()

	if v, ok := r.Get("x"); !ok || v != "10" {
		t.Errorf("x: got (%q, %v), want (10, true)", v, ok)
	}
	if _, ok := r.Get("y"); ok {
		t.Error("y should be a tombstone, Get should return false")
	}
	if !r.IsTombstone("y") {
		t.Error("y should be IsTombstone=true")
	}
	if v, ok := r.Get("z"); !ok || v != "30" {
		t.Errorf("z: got (%q, %v), want (30, true)", v, ok)
	}
}

// TestSSTable_EmptyTable verifies that an SSTable with zero entries
// opens cleanly and all lookups return false.
func TestSSTable_EmptyTable(t *testing.T) {
	r := writeTestSSTable(t, nil)
	if _, ok := r.Get("anything"); ok {
		t.Error("Get on empty SSTable should return false")
	}
	entries, err := r.Entries()
	if err != nil {
		t.Fatalf("Entries on empty SSTable: %v", err)
	}
	if len(entries) != 0 {
		t.Errorf("expected 0 entries, got %d", len(entries))
	}
}
