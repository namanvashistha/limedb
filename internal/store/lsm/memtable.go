package lsm

import (
	"sort"
	"sync"

	"limedb/internal/store"
)

// entry holds a single key-value pair in the MemTable.
// Deletions are represented as tombstones (deleted=true, value="").
type entry struct {
	key     string
	value   string
	ts      int64 // LWW timestamp in microseconds
	deleted bool
}

// MemTable is a thread-safe, size-bounded in-memory store.
//
// Writes are O(log n) (binary-search insertion into a sorted slice). Reads
// are O(log n). We use a sorted slice rather than a skip list or red-black
// tree to keep the implementation self-contained; for a production engine
// you'd swap in github.com/google/btree.
//
// When ApproximateSize() exceeds the configured threshold callers should
// freeze this MemTable, flush it to an SSTable, and start a fresh one.
// The WAL that backs this MemTable can then be Reset().
type MemTable struct {
	mu      sync.RWMutex
	entries []entry // kept sorted by key at all times
	sizeB   int64   // approximate byte footprint of stored data
}

// NewMemTable creates an empty MemTable.
func NewMemTable() *MemTable {
	return &MemTable{}
}

// Put inserts or updates key with the given versioned value, applying LWW:
// if an existing entry is newer, the write is dropped. Returns whether the
// write was applied. Tombstones are stored like any other entry so that a
// deleted key is not "resurrected" by an older version living in an SSTable.
func (m *MemTable) Put(key string, v store.VersionedValue) bool {
	m.mu.Lock()
	defer m.mu.Unlock()

	idx, found := m.search(key)
	if found {
		cur := m.entries[idx]
		if !store.Newer(v, store.VersionedValue{Value: cur.value, TimestampMicros: cur.ts, Tombstone: cur.deleted}) {
			return false
		}
		m.sizeB -= int64(len(cur.value))
		m.entries[idx].value = v.Value
		m.entries[idx].ts = v.TimestampMicros
		m.entries[idx].deleted = v.Tombstone
		m.sizeB += int64(len(v.Value))
		return true
	}
	// Insert at sorted position.
	m.entries = append(m.entries, entry{})
	copy(m.entries[idx+1:], m.entries[idx:])
	m.entries[idx] = entry{key: key, value: v.Value, ts: v.TimestampMicros, deleted: v.Tombstone}
	m.sizeB += int64(len(key) + len(v.Value))
	return true
}

// Get returns (value, true) if the key is present, including tombstones —
// callers check .Tombstone to distinguish a delete from a live value.
func (m *MemTable) Get(key string) (store.VersionedValue, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	idx, found := m.search(key)
	if !found {
		return store.VersionedValue{}, false
	}
	e := m.entries[idx]
	return store.VersionedValue{Value: e.value, TimestampMicros: e.ts, Tombstone: e.deleted}, true
}

// Has returns true if the key is present in the MemTable, even if it is a
// tombstone.  Used during SSTable lookups to short-circuit disk reads.
func (m *MemTable) Has(key string) bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	_, found := m.search(key)
	return found
}

// Entries returns a snapshot of all entries (including tombstones) in sorted
// key order.  This is the data that gets flushed to an SSTable.
func (m *MemTable) Entries() []Entry {
	m.mu.RLock()
	defer m.mu.RUnlock()

	out := make([]Entry, len(m.entries))
	for i, e := range m.entries {
		out[i] = Entry{
			Key:             e.key,
			Value:           e.value,
			TimestampMicros: e.ts,
			Deleted:         e.deleted,
		}
	}
	return out
}

// Len returns the number of entries (including tombstones).
func (m *MemTable) Len() int {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return len(m.entries)
}

// ApproximateSize returns the approximate in-memory byte footprint of the
// stored data (keys + values).  Does not include Go struct overhead.
func (m *MemTable) ApproximateSize() int64 {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.sizeB
}

// search returns (index, found) using binary search on m.entries.
// If found is false, index is the insertion point.
// Callers must hold at least a read lock.
func (m *MemTable) search(key string) (int, bool) {
	idx := sort.Search(len(m.entries), func(i int) bool {
		return m.entries[i].key >= key
	})
	if idx < len(m.entries) && m.entries[idx].key == key {
		return idx, true
	}
	return idx, false
}

// RestoreFromWAL applies a slice of WAL records (as returned by WAL.Replay)
// to this MemTable. Call this once at startup before accepting new writes.
func (m *MemTable) RestoreFromWAL(records []Record) {
	for _, r := range records {
		m.Put(r.Key, store.VersionedValue{
			Value:           r.Value,
			TimestampMicros: r.TimestampMicros,
			Tombstone:       r.Op == OpDel,
		})
	}
}

// Entry is a public view of a single MemTable entry used during SSTable flush.
type Entry struct {
	Key             string
	Value           string
	TimestampMicros int64
	Deleted         bool // true if this entry is a tombstone
}
