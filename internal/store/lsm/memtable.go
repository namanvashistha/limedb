package lsm

import (
	"sort"
	"sync"
)

// tombstone is the special sentinel value stored for deleted keys.
// It is unexported so callers never have to know about it.
const tombstone = "\x00__tombstone__\x00"

// entry holds a single key-value pair in the MemTable.
// deletions are represented by value == tombstone.
type entry struct {
	key   string
	value string
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

// Set inserts or updates key with value.
// If the key previously held a tombstone (deleted) it is overwritten.
func (m *MemTable) Set(key, value string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	idx, found := m.search(key)
	if found {
		old := m.entries[idx].value
		m.sizeB -= int64(len(old))
		m.entries[idx].value = value
		m.sizeB += int64(len(value))
		return
	}
	// Insert at sorted position.
	m.entries = append(m.entries, entry{})
	copy(m.entries[idx+1:], m.entries[idx:])
	m.entries[idx] = entry{key: key, value: value}
	m.sizeB += int64(len(key) + len(value))
}

// Delete marks key as deleted by writing a tombstone.
// Tombstones are necessary so that a key deleted from the MemTable is not
// "resurrected" by an older version living in an SSTable on disk.
func (m *MemTable) Delete(key string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	idx, found := m.search(key)
	if found {
		old := m.entries[idx].value
		m.sizeB -= int64(len(old))
		m.entries[idx].value = tombstone
		m.sizeB += int64(len(tombstone))
		return
	}
	// Key not present — write a tombstone anyway so SSTables can be skipped.
	m.entries = append(m.entries, entry{})
	copy(m.entries[idx+1:], m.entries[idx:])
	m.entries[idx] = entry{key: key, value: tombstone}
	m.sizeB += int64(len(key) + len(tombstone))
}

// Get returns (value, true) if the key exists and has not been deleted.
// Returns ("", false) if the key is absent or has a tombstone.
func (m *MemTable) Get(key string) (string, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	idx, found := m.search(key)
	if !found {
		return "", false
	}
	v := m.entries[idx].value
	if v == tombstone {
		return "", false // deleted
	}
	return v, true
}

// Has returns true if the key is present in the MemTable, even if it is a
// tombstone.  Used during SSTable lookups to short-circuit disk reads.
func (m *MemTable) Has(key string) bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	_, found := m.search(key)
	return found
}

// IsDeleted reports whether key carries a tombstone in this MemTable.
func (m *MemTable) IsDeleted(key string) bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	idx, found := m.search(key)
	if !found {
		return false
	}
	return m.entries[idx].value == tombstone
}

// Entries returns a snapshot of all entries (including tombstones) in sorted
// key order.  This is the data that gets flushed to an SSTable.
func (m *MemTable) Entries() []Entry {
	m.mu.RLock()
	defer m.mu.RUnlock()

	out := make([]Entry, len(m.entries))
	for i, e := range m.entries {
		out[i] = Entry{
			Key:     e.key,
			Value:   e.value,
			Deleted: e.value == tombstone,
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
		switch r.Op {
		case OpSet:
			m.Set(r.Key, r.Value)
		case OpDel:
			m.Delete(r.Key)
		}
	}
}

// Entry is a public view of a single MemTable entry used during SSTable flush.
type Entry struct {
	Key     string
	Value   string
	Deleted bool // true if this entry is a tombstone
}
