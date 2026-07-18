package store

// VersionedValue is a value plus the metadata needed for last-write-wins
// (LWW) conflict resolution between replicas. The timestamp is assigned once
// by the coordinator that accepted the client write and travels with the
// value through the WAL, SSTables, and replication messages.
//
// Deletes are represented as tombstones (Tombstone=true, Value="") so that
// a delete can win against an older value that resurfaces from another
// replica or an older SSTable.
type VersionedValue struct {
	Value           string `json:"value"`
	TimestampMicros int64  `json:"timestamp_micros"`
	Tombstone       bool   `json:"tombstone"`
}

// Newer reports whether a should win over b under LWW rules:
// higher timestamp wins; on an exact tie a tombstone beats a value, and
// otherwise the lexically larger value wins. The tie-breaks are arbitrary
// but deterministic, so every replica converges to the same answer.
func Newer(a, b VersionedValue) bool {
	if a.TimestampMicros != b.TimestampMicros {
		return a.TimestampMicros > b.TimestampMicros
	}
	if a.Tombstone != b.Tombstone {
		return a.Tombstone
	}
	return a.Value > b.Value
}

// Backend defines the interface for a key-value store backend.
// Implement this interface to plug in any storage engine (in-memory, bbolt, badger, etc.)
//
// Get returns ok=true even when the stored record is a tombstone — callers
// that only care about live data must check .Tombstone. Put applies LWW
// against the existing record and reports whether the write was applied.
type Backend interface {
	Get(key string) (VersionedValue, bool)
	Put(key string, v VersionedValue) bool
	ListKeys() []string // live keys only (tombstones excluded)
	Count() int
	Stats() map[string]interface{}
}
