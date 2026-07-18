package lsm

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"
)

// ── helpers ───────────────────────────────────────────────────────────────────

// makeSSTable writes entries to a new .sst file in dir and returns the path.
func makeSSTable(t *testing.T, dir string, name string, entries []Entry) string {
	t.Helper()
	path := filepath.Join(dir, name+".sst")
	w, err := NewSSTableWriter(path)
	if err != nil {
		t.Fatalf("NewSSTableWriter(%s): %v", name, err)
	}
	bf := NewBloomFilter(len(entries)+1, 0.01)
	for _, e := range entries {
		if err := w.WriteEntry(e); err != nil {
			t.Fatalf("WriteEntry: %v", err)
		}
		bf.Add(e.Key)
	}
	if err := w.Flush(); err != nil {
		t.Fatalf("Flush: %v", err)
	}
	_ = WriteBloomFile(path, bf)
	return path
}

// readAll opens an SSTable and returns all its entries.
func readAll(t *testing.T, path string) []Entry {
	t.Helper()
	r, err := OpenSSTable(path)
	if err != nil {
		t.Fatalf("OpenSSTable(%s): %v", path, err)
	}
	defer r.Close()
	entries, err := r.Entries()
	if err != nil {
		t.Fatalf("Entries: %v", err)
	}
	return entries
}

// ── MergeSSTables tests ───────────────────────────────────────────────────────

// TestMerge_NewestWins verifies that when two SSTables contain the same key,
// the first (newest) input's value is kept.
func TestMerge_NewestWins(t *testing.T) {
	dir := t.TempDir()

	newer := makeSSTable(t, dir, "t1", []Entry{
		{Key: "a", Value: "new-a"},
		{Key: "c", Value: "new-c"},
	})
	older := makeSSTable(t, dir, "t2", []Entry{
		{Key: "a", Value: "old-a"}, // stale — must be dropped
		{Key: "b", Value: "old-b"},
	})

	out := filepath.Join(dir, "merged.sst")
	if err := MergeSSTables([]string{newer, older}, out, MergeOptions{}); err != nil {
		t.Fatalf("MergeSSTables: %v", err)
	}

	entries := readAll(t, out)
	if len(entries) != 3 {
		t.Fatalf("want 3 entries, got %d: %v", len(entries), entries)
	}
	// Sorted order: a, b, c
	check := func(i int, key, val string) {
		t.Helper()
		if entries[i].Key != key || entries[i].Value != val {
			t.Errorf("entry[%d]: got {%q,%q}, want {%q,%q}",
				i, entries[i].Key, entries[i].Value, key, val)
		}
	}
	check(0, "a", "new-a")
	check(1, "b", "old-b")
	check(2, "c", "new-c")
}

// TestMerge_HigherTimestampWins verifies that on a key collision the record
// with the higher LWW timestamp wins even when it lives in the older table
// (e.g. a late replica write applied after a flush).
func TestMerge_HigherTimestampWins(t *testing.T) {
	dir := t.TempDir()

	newerTable := makeSSTable(t, dir, "t1", []Entry{
		{Key: "a", Value: "flushed-later", TimestampMicros: 100},
	})
	olderTable := makeSSTable(t, dir, "t2", []Entry{
		{Key: "a", Value: "written-later", TimestampMicros: 200},
	})

	out := filepath.Join(dir, "merged.sst")
	if err := MergeSSTables([]string{newerTable, olderTable}, out, MergeOptions{}); err != nil {
		t.Fatalf("MergeSSTables: %v", err)
	}

	entries := readAll(t, out)
	if len(entries) != 1 || entries[0].Value != "written-later" || entries[0].TimestampMicros != 200 {
		t.Errorf("merge should keep the higher-timestamp record: got %+v", entries)
	}

	// A newer tombstone must also beat an older value across tables.
	tombTable := makeSSTable(t, dir, "t3", []Entry{
		{Key: "b", Deleted: true, TimestampMicros: 300},
	})
	valTable := makeSSTable(t, dir, "t4", []Entry{
		{Key: "b", Value: "zombie", TimestampMicros: 250},
	})
	out2 := filepath.Join(dir, "merged2.sst")
	if err := MergeSSTables([]string{valTable, tombTable}, out2, MergeOptions{}); err != nil {
		t.Fatalf("MergeSSTables: %v", err)
	}
	entries = readAll(t, out2)
	if len(entries) != 1 || !entries[0].Deleted {
		t.Errorf("newer tombstone should win: got %+v", entries)
	}
}

// TestMerge_TombstonePreservedByDefault verifies tombstones are kept when
// DropTombstones=false.
func TestMerge_TombstonePreservedByDefault(t *testing.T) {
	dir := t.TempDir()
	newer := makeSSTable(t, dir, "t1", []Entry{
		{Key: "gone", Deleted: true},
		{Key: "live", Value: "yes"},
	})
	out := filepath.Join(dir, "merged.sst")
	_ = MergeSSTables([]string{newer}, out, MergeOptions{DropTombstones: false})

	entries := readAll(t, out)
	if len(entries) != 2 {
		t.Fatalf("want 2 entries (tombstone kept), got %d", len(entries))
	}
	if !entries[0].Deleted {
		t.Error("tombstone should be preserved")
	}
}

// TestMerge_DropTombstones verifies tombstones are removed when opted in.
func TestMerge_DropTombstones(t *testing.T) {
	dir := t.TempDir()
	newer := makeSSTable(t, dir, "t1", []Entry{
		{Key: "gone", Deleted: true},
		{Key: "live", Value: "yes"},
	})
	out := filepath.Join(dir, "merged.sst")
	_ = MergeSSTables([]string{newer}, out, MergeOptions{DropTombstones: true})

	entries := readAll(t, out)
	if len(entries) != 1 || entries[0].Key != "live" {
		t.Errorf("expected only 'live' after dropping tombstones; got %v", entries)
	}
}

// TestMerge_ThreeWay merges three SSTables with overlapping keys.
func TestMerge_ThreeWay(t *testing.T) {
	dir := t.TempDir()
	t1 := makeSSTable(t, dir, "t1", []Entry{ // newest
		{Key: "b", Value: "b-v2"},
		{Key: "d", Value: "d-v1"},
	})
	t2 := makeSSTable(t, dir, "t2", []Entry{
		{Key: "a", Value: "a-v1"},
		{Key: "b", Value: "b-v1"}, // stale
		{Key: "c", Value: "c-v1"},
	})
	t3 := makeSSTable(t, dir, "t3", []Entry{ // oldest
		{Key: "a", Value: "a-v0"}, // stale
		{Key: "e", Value: "e-v1"},
	})

	out := filepath.Join(dir, "merged.sst")
	if err := MergeSSTables([]string{t1, t2, t3}, out, MergeOptions{DropTombstones: true}); err != nil {
		t.Fatalf("MergeSSTables: %v", err)
	}

	entries := readAll(t, out)
	want := []Entry{
		{Key: "a", Value: "a-v1"},
		{Key: "b", Value: "b-v2"},
		{Key: "c", Value: "c-v1"},
		{Key: "d", Value: "d-v1"},
		{Key: "e", Value: "e-v1"},
	}
	if len(entries) != len(want) {
		t.Fatalf("len: got %d, want %d; entries=%v", len(entries), len(want), entries)
	}
	for i, w := range want {
		if entries[i].Key != w.Key || entries[i].Value != w.Value {
			t.Errorf("[%d] got {%q,%q}, want {%q,%q}",
				i, entries[i].Key, entries[i].Value, w.Key, w.Value)
		}
	}
}

// TestMerge_BloomSidecarCreated verifies the merged SSTable has a usable sidecar.
func TestMerge_BloomSidecarCreated(t *testing.T) {
	dir := t.TempDir()
	src := makeSSTable(t, dir, "src", []Entry{
		{Key: "hello", Value: "world"},
	})
	out := filepath.Join(dir, "merged.sst")
	_ = MergeSSTables([]string{src}, out, MergeOptions{})

	bf, err := ReadBloomFile(out)
	if err != nil || bf == nil {
		t.Fatalf("ReadBloomFile after merge: err=%v, bf=%v", err, bf)
	}
	if !bf.MayContain("hello") {
		t.Error("merged Bloom filter should contain 'hello'")
	}
}

// ── Compactor tests ───────────────────────────────────────────────────────────

// TestCompactor_TriggersAboveThreshold verifies that adding >= threshold
// SSTables causes the compactor to merge them into one.
func TestCompactor_TriggersAboveThreshold(t *testing.T) {
	dir := t.TempDir()
	inputsCh := make(chan []string, 1)
	outputCh := make(chan string, 1)

	cfg := CompactorConfig{
		Dir:          dir,
		Threshold:    4,
		PollInterval: time.Hour, // disable polling; triggered by Add
		OnCompacted: func(inputs []string, output string, durationMs int64) {
			inputsCh <- inputs
			outputCh <- output
		},
	}

	c := NewCompactor(cfg)
	c.Start()
	defer c.Stop()

	// Create 4 SSTables and register them.
	for i := 0; i < 4; i++ {
		entries := []Entry{{Key: fmt.Sprintf("key-%d", i), Value: fmt.Sprintf("v%d", i)}}
		path := makeSSTable(t, dir, fmt.Sprintf("sst-%d", i), entries)
		c.Add(path)
	}

	// Wait for compactor — it should trigger automatically once threshold is hit.
	select {
	case inputs := <-inputsCh:
		output := <-outputCh
		if len(inputs) != 4 {
			t.Errorf("expected 4 inputs, got %d", len(inputs))
		}
		if _, err := os.Stat(output); err != nil {
			t.Errorf("merged SSTable missing: %v", err)
		}
		// After compaction the compactor's table list should have exactly 1 entry.
		if got := c.Tables(); len(got) != 1 || got[0] != output {
			t.Errorf("Tables() after compaction: %v", got)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("compaction did not run within 5s")
	}
}

// TestCompactor_OldFilesDeleted verifies that input SSTable files are removed
// after successful compaction.
func TestCompactor_OldFilesDeleted(t *testing.T) {
	dir := t.TempDir()
	doneCh := make(chan struct{}, 1)

	cfg := CompactorConfig{
		Dir:          dir,
		Threshold:    2,
		PollInterval: time.Hour,
		OnCompacted: func(_ []string, _ string, _ int64) {
			select {
			case doneCh <- struct{}{}:
			default:
			}
		},
	}
	c := NewCompactor(cfg)
	c.Start()
	defer c.Stop()

	paths := make([]string, 2)
	for i := range paths {
		entries := []Entry{{Key: fmt.Sprintf("k%d", i), Value: "v"}}
		paths[i] = makeSSTable(t, dir, fmt.Sprintf("s%d", i), entries)
		c.Add(paths[i])
	}

	select {
	case <-doneCh:
	case <-time.After(5 * time.Second):
		t.Fatal("compaction did not run")
	}

	// The compactor deletes old files after firing OnCompacted; poll briefly.
	deadline := time.Now().Add(2 * time.Second)
	for _, p := range paths {
		for {
			_, err := os.Stat(p)
			if os.IsNotExist(err) {
				break // deleted ✓
			}
			if time.Now().After(deadline) {
				t.Errorf("old SSTable %s should have been deleted", p)
				break
			}
			time.Sleep(10 * time.Millisecond)
		}
	}
}
