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

// TestSSTable_BasicGetHit writes a few entries then looks each one up,
// checking values and timestamps round-trip.
func TestSSTable_BasicGetHit(t *testing.T) {
	entries := []Entry{
		{Key: "apple", Value: "red", TimestampMicros: 11},
		{Key: "banana", Value: "yellow", TimestampMicros: 22},
		{Key: "cherry", Value: "dark-red", TimestampMicros: 33},
		{Key: "durian", Value: "spiky", TimestampMicros: 44},
	}
	r := writeTestSSTable(t, entries)

	for _, e := range entries {
		v, ok := r.Get(e.Key)
		if !ok || v.Value != e.Value || v.TimestampMicros != e.TimestampMicros || v.Tombstone {
			t.Errorf("Get(%q): got (%+v, %v), want (%q ts=%d, true)", e.Key, v, ok, e.Value, e.TimestampMicros)
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

// TestSSTable_Tombstone verifies that tombstoned keys are returned with the
// Tombstone flag set (and their timestamp intact).
func TestSSTable_Tombstone(t *testing.T) {
	r := writeTestSSTable(t, []Entry{
		{Key: "alive", Value: "yes", TimestampMicros: 1},
		{Key: "dead", Deleted: true, TimestampMicros: 2},
	})

	v, ok := r.Get("dead")
	if !ok || !v.Tombstone || v.TimestampMicros != 2 {
		t.Errorf("Get(dead): got (%+v, %v), want tombstone ts=2", v, ok)
	}
	v, ok = r.Get("alive")
	if !ok || v.Tombstone {
		t.Errorf("Get(alive): got (%+v, %v), want live value", v, ok)
	}
}

// TestSSTable_Entries verifies the full scan returns entries in order,
// including tombstones, with timestamps preserved.
func TestSSTable_Entries(t *testing.T) {
	src := []Entry{
		{Key: "a", Value: "1", TimestampMicros: 100},
		{Key: "b", Deleted: true, TimestampMicros: 200},
		{Key: "c", Value: "3", TimestampMicros: 300},
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
		if got[i].Key != e.Key || got[i].Deleted != e.Deleted || got[i].TimestampMicros != e.TimestampMicros {
			t.Errorf("entry[%d]: got %+v, want %+v", i, got[i], e)
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
			Key:             fmt.Sprintf("key-%04d", i),
			Value:           fmt.Sprintf("val-%d", i),
			TimestampMicros: int64(i + 1),
		}
	}
	r := writeTestSSTable(t, entries)

	// Spot-check a few entries spread across index blocks.
	for _, i := range []int{0, 15, 16, 31, 32, n - 1} {
		want := entries[i]
		v, ok := r.Get(want.Key)
		if !ok || v.Value != want.Value {
			t.Errorf("Get(%q): got (%+v, %v), want (%q, true)", want.Key, v, ok, want.Value)
		}
	}
}

// TestSSTable_FlushFromMemTable verifies the end-to-end path:
// MemTable → Entries() → SSTableWriter → SSTableReader.Get
func TestSSTable_FlushFromMemTable(t *testing.T) {
	m := NewMemTable()
	m.Put("x", val("10"))
	m.Put("y", val("20"))
	m.Put("z", val("30"))
	m.Put("y", tomb())

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

	if v, ok := liveGet(r, "x"); !ok || v != "10" {
		t.Errorf("x: got (%q, %v), want (10, true)", v, ok)
	}
	if v, ok := r.Get("y"); !ok || !v.Tombstone {
		t.Errorf("y should be a tombstone: got (%+v, %v)", v, ok)
	}
	if v, ok := liveGet(r, "z"); !ok || v != "30" {
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
