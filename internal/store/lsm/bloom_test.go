package lsm

import (
	"fmt"
	"path/filepath"
	"testing"
)

// TestBloom_AddAndMayContain verifies that all added keys return true.
func TestBloom_AddAndMayContain(t *testing.T) {
	f := NewBloomFilter(100, 0.01)
	keys := []string{"apple", "banana", "cherry", "durian", "elderberry"}
	for _, k := range keys {
		f.Add(k)
	}
	for _, k := range keys {
		if !f.MayContain(k) {
			t.Errorf("MayContain(%q) = false, want true (false negative!)", k)
		}
	}
}

// TestBloom_NoFalseNegatives is a larger-scale check: insert n keys, then
// verify every single one is found (false negatives are never acceptable).
func TestBloom_NoFalseNegatives(t *testing.T) {
	const n = 10_000
	f := NewBloomFilter(n, 0.01)
	keys := make([]string, n)
	for i := range keys {
		keys[i] = fmt.Sprintf("key-%08d", i)
		f.Add(keys[i])
	}
	for _, k := range keys {
		if !f.MayContain(k) {
			t.Errorf("false negative for %q", k)
		}
	}
}

// TestBloom_FalsePositiveRate inserts n keys then checks m distinct absent
// keys; the empirical FP rate must be within 2× the configured target.
func TestBloom_FalsePositiveRate(t *testing.T) {
	const n = 10_000
	const fpTarget = 0.01
	f := NewBloomFilter(n, fpTarget)

	for i := 0; i < n; i++ {
		f.Add(fmt.Sprintf("key-%d", i))
	}

	// Check keys that were never added (use a disjoint prefix).
	const checks = 100_000
	var falsePositives int
	for i := 0; i < checks; i++ {
		if f.MayContain(fmt.Sprintf("absent-%d", i)) {
			falsePositives++
		}
	}
	empirical := float64(falsePositives) / float64(checks)
	if empirical > fpTarget*2 {
		t.Errorf("false-positive rate too high: %.4f (target %.4f, budget 2×)", empirical, fpTarget)
	}
	t.Logf("FP rate: %.4f (target %.4f)", empirical, fpTarget)
}

// TestBloom_Serialization verifies a round-trip through Bytes → BloomFilterFromBytes.
func TestBloom_Serialization(t *testing.T) {
	f := NewBloomFilter(200, 0.01)
	for i := 0; i < 200; i++ {
		f.Add(fmt.Sprintf("k%d", i))
	}

	data := f.Bytes()
	f2, err := BloomFilterFromBytes(data)
	if err != nil {
		t.Fatalf("BloomFilterFromBytes: %v", err)
	}

	// All originally-inserted keys must still be found.
	for i := 0; i < 200; i++ {
		k := fmt.Sprintf("k%d", i)
		if !f2.MayContain(k) {
			t.Errorf("after deserialise: false negative for %q", k)
		}
	}
}

// TestBloom_SidecarFile exercises WriteBloomFile and ReadBloomFile.
func TestBloom_SidecarFile(t *testing.T) {
	sstPath := filepath.Join(t.TempDir(), "0001.sst")
	f := NewBloomFilter(50, 0.01)
	f.Add("hello")
	f.Add("world")

	if err := WriteBloomFile(sstPath, f); err != nil {
		t.Fatalf("WriteBloomFile: %v", err)
	}

	f2, err := ReadBloomFile(sstPath)
	if err != nil {
		t.Fatalf("ReadBloomFile: %v", err)
	}
	if f2 == nil {
		t.Fatal("ReadBloomFile returned nil (file should exist)")
	}
	if !f2.MayContain("hello") {
		t.Error("MayContain(hello) = false after sidecar round-trip")
	}
	if !f2.MayContain("world") {
		t.Error("MayContain(world) = false after sidecar round-trip")
	}
}

// TestBloom_SidecarMissing verifies ReadBloomFile returns (nil, nil) when the
// sidecar file doesn't exist (not an error — filter just isn't available).
func TestBloom_SidecarMissing(t *testing.T) {
	f, err := ReadBloomFile(filepath.Join(t.TempDir(), "nonexistent.sst"))
	if err != nil {
		t.Errorf("ReadBloomFile (missing): expected nil error, got %v", err)
	}
	if f != nil {
		t.Error("ReadBloomFile (missing): expected nil filter")
	}
}

// TestBloom_EmptyFilter verifies a filter with no insertions never returns true.
func TestBloom_EmptyFilter(t *testing.T) {
	f := NewBloomFilter(100, 0.01)
	// No keys added; nothing should match.
	for _, k := range []string{"a", "b", "hello", "limedb"} {
		if f.MayContain(k) {
			t.Errorf("empty filter: MayContain(%q) = true, want false", k)
		}
	}
}

// TestBloom_IntegrationWithSSTable checks the end-to-end path:
//  1. Write a MemTable to an SSTable
//  2. Build a Bloom filter from the same entries
//  3. Save the sidecar file
//  4. On read: filter correctly skips absent keys and passes present ones
func TestBloom_IntegrationWithSSTable(t *testing.T) {
	m := NewMemTable()
	const n = 50
	for i := 0; i < n; i++ {
		m.Set(fmt.Sprintf("item-%02d", i), fmt.Sprintf("v%d", i))
	}
	m.Delete("item-05") // tombstone

	entries := m.Entries()
	sstPath := filepath.Join(t.TempDir(), "0001.sst")

	// Flush to SSTable.
	w, _ := NewSSTableWriter(sstPath)
	f := NewBloomFilter(len(entries), 0.01)
	for _, e := range entries {
		_ = w.WriteEntry(e)
		f.Add(e.Key)
	}
	_ = w.Flush()
	_ = WriteBloomFile(sstPath, f)

	// Open for reading.
	r, err := OpenSSTable(sstPath)
	if err != nil {
		t.Fatalf("OpenSSTable: %v", err)
	}
	defer r.Close()

	bf, err := ReadBloomFile(sstPath)
	if err != nil {
		t.Fatalf("ReadBloomFile: %v", err)
	}

	// Every inserted key must pass the filter (no false negatives).
	for i := 0; i < n; i++ {
		k := fmt.Sprintf("item-%02d", i)
		if !bf.MayContain(k) {
			t.Errorf("filter: false negative for inserted key %q", k)
		}
	}

	// Absent keys should be skippable (show the pattern a caller would use).
	absent := []string{"zzzz", "does-not-exist", "item-99"}
	skipped := 0
	for _, k := range absent {
		if !bf.MayContain(k) {
			skipped++ // SSTable disk read avoided ✓
		}
	}
	t.Logf("%d/%d absent keys correctly skipped via Bloom filter", skipped, len(absent))
}
