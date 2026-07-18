package lsm

import (
	"fmt"
	"sync"
	"testing"

	"limedb/internal/store"
)

func TestMemTable_PutAndGet(t *testing.T) {
	m := NewMemTable()

	m.Put("color", val("lime"))
	m.Put("fruit", val("mango"))
	m.Put("color", val("green")) // overwrite with newer timestamp

	if v, ok := liveGet(m, "color"); !ok || v != "green" {
		t.Errorf("color: got (%q, %v), want (green, true)", v, ok)
	}
	if v, ok := liveGet(m, "fruit"); !ok || v != "mango" {
		t.Errorf("fruit: got (%q, %v), want (mango, true)", v, ok)
	}
	if _, ok := m.Get("missing"); ok {
		t.Error("missing key should return false")
	}
}

func TestMemTable_LWW_OlderWriteIgnored(t *testing.T) {
	m := NewMemTable()

	if !m.Put("k", store.VersionedValue{Value: "new", TimestampMicros: 100}) {
		t.Fatal("first write should apply")
	}
	if m.Put("k", store.VersionedValue{Value: "stale", TimestampMicros: 50}) {
		t.Error("older write should be rejected")
	}
	if v, _ := m.Get("k"); v.Value != "new" || v.TimestampMicros != 100 {
		t.Errorf("got %+v, want the newer record", v)
	}
}

func TestMemTable_LWW_TieBreaks(t *testing.T) {
	m := NewMemTable()

	// Exact-tie: tombstone beats value.
	m.Put("k", store.VersionedValue{Value: "v", TimestampMicros: 10})
	if !m.Put("k", store.VersionedValue{TimestampMicros: 10, Tombstone: true}) {
		t.Error("tombstone should win a timestamp tie against a value")
	}
	// Value must not win back against the tombstone at the same timestamp.
	if m.Put("k", store.VersionedValue{Value: "v", TimestampMicros: 10}) {
		t.Error("value should not beat a tombstone at the same timestamp")
	}

	// Value-vs-value tie: lexically larger wins, deterministically.
	m.Put("x", store.VersionedValue{Value: "aaa", TimestampMicros: 20})
	if !m.Put("x", store.VersionedValue{Value: "bbb", TimestampMicros: 20}) {
		t.Error("lexically larger value should win the tie")
	}
	if m.Put("x", store.VersionedValue{Value: "aaa", TimestampMicros: 20}) {
		t.Error("lexically smaller value should lose the tie")
	}
}

func TestMemTable_Delete_Tombstone(t *testing.T) {
	m := NewMemTable()
	m.Put("x", val("hello"))
	m.Put("x", tomb())

	v, ok := m.Get("x")
	if !ok || !v.Tombstone {
		t.Errorf("tombstone should be present and flagged: got (%+v, %v)", v, ok)
	}
	// Tombstone still makes Has() return true (needed for SSTable short-circuit).
	if !m.Has("x") {
		t.Error("Has should return true for a tombstoned key")
	}
}

func TestMemTable_DeleteNeverWritten(t *testing.T) {
	m := NewMemTable()
	// Deleting a key that was never written should still write a tombstone.
	m.Put("ghost", tomb())
	if v, ok := m.Get("ghost"); !ok || !v.Tombstone {
		t.Error("tombstone should exist even for never-written key")
	}
}

func TestMemTable_SetAfterDelete(t *testing.T) {
	m := NewMemTable()
	m.Put("k", val("v1"))
	m.Put("k", tomb())
	m.Put("k", val("v2")) // resurrection with a newer timestamp

	v, ok := m.Get("k")
	if !ok || v.Tombstone || v.Value != "v2" {
		t.Errorf("after resurrection: got (%+v, %v), want v2", v, ok)
	}
}

func TestMemTable_Entries_SortedAndTombstones(t *testing.T) {
	m := NewMemTable()
	m.Put("banana", val("yellow"))
	m.Put("apple", val("red"))
	m.Put("cherry", val("dark"))
	m.Put("apple", tomb())

	entries := m.Entries()
	if len(entries) != 3 {
		t.Fatalf("expected 3 entries, got %d", len(entries))
	}

	// Must be sorted by key.
	keys := []string{entries[0].Key, entries[1].Key, entries[2].Key}
	if keys[0] != "apple" || keys[1] != "banana" || keys[2] != "cherry" {
		t.Errorf("wrong order: %v", keys)
	}

	// apple should be flagged as deleted, with its timestamp preserved.
	if !entries[0].Deleted {
		t.Error("apple entry should be Deleted=true")
	}
	if entries[0].TimestampMicros == 0 {
		t.Error("tombstone entry should carry its timestamp")
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

	m.Put("foo", val("bar")) // +3+3 = 6
	if m.ApproximateSize() < 6 {
		t.Errorf("size should be >= 6, got %d", m.ApproximateSize())
	}

	prev := m.ApproximateSize()
	m.Put("foo", val("longervalue")) // value grows
	if m.ApproximateSize() <= prev {
		t.Error("size should grow when value is replaced with a longer one")
	}
}

func TestMemTable_RestoreFromWAL(t *testing.T) {
	records := []Record{
		{Op: OpSet, Key: "a", Value: "1", TimestampMicros: 1},
		{Op: OpSet, Key: "b", Value: "2", TimestampMicros: 2},
		{Op: OpDel, Key: "a", TimestampMicros: 3},
		{Op: OpSet, Key: "c", Value: "3", TimestampMicros: 4},
	}

	m := NewMemTable()
	m.RestoreFromWAL(records)

	if v, ok := liveGet(m, "a"); ok {
		t.Errorf("a should be deleted after WAL replay, got %q", v)
	}
	if v, ok := liveGet(m, "b"); !ok || v != "2" {
		t.Errorf("b: got (%q, %v), want (2, true)", v, ok)
	}
	if v, ok := liveGet(m, "c"); !ok || v != "3" {
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
				m.Put(fmt.Sprintf("key-%d-%d", id, j), val(fmt.Sprintf("v%d", j)))
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
