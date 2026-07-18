package lsm

import (
	"sync/atomic"

	"limedb/internal/store"
)

// testClock hands out monotonically increasing LWW timestamps so test writes
// are ordered the same way coordinator-assigned wall-clock timestamps would be.
var testClock int64

func nextTS() int64 { return atomic.AddInt64(&testClock, 1) }

func val(v string) store.VersionedValue {
	return store.VersionedValue{Value: v, TimestampMicros: nextTS()}
}

func tomb() store.VersionedValue {
	return store.VersionedValue{TimestampMicros: nextTS(), Tombstone: true}
}

// liveGet adapts the versioned Get to the "live value" view most tests want:
// tombstones read as absent.
func liveGet(g interface {
	Get(key string) (store.VersionedValue, bool)
}, key string) (string, bool) {
	v, ok := g.Get(key)
	if !ok || v.Tombstone {
		return "", false
	}
	return v.Value, true
}
