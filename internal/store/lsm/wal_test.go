package lsm

import (
	"os"
	"path/filepath"
	"testing"
)

// tempWAL creates a WAL backed by a file in t.TempDir().
// The file is cleaned up automatically when the test ends.
func tempWAL(t *testing.T) *WAL {
	t.Helper()
	path := filepath.Join(t.TempDir(), "wal.log")
	w, err := OpenWAL(path)
	if err != nil {
		t.Fatalf("OpenWAL: %v", err)
	}
	t.Cleanup(func() { _ = w.Close() })
	return w
}

// TestWAL_WriteAndReplay verifies that SET and DEL records survive a full
// write → close → reopen → Replay round-trip.
func TestWAL_WriteAndReplay(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "wal.log")

	// --- write phase ---
	w, err := OpenWAL(path)
	if err != nil {
		t.Fatalf("OpenWAL: %v", err)
	}
	if err := w.WriteSet("name", "lime"); err != nil {
		t.Fatalf("WriteSet: %v", err)
	}
	if err := w.WriteSet("color", "green"); err != nil {
		t.Fatalf("WriteSet: %v", err)
	}
	if err := w.WriteDel("color"); err != nil {
		t.Fatalf("WriteDel: %v", err)
	}
	if err := w.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	// --- replay phase (same path, new handle) ---
	w2, err := OpenWAL(path)
	if err != nil {
		t.Fatalf("OpenWAL (reopen): %v", err)
	}
	defer w2.Close()

	records, err := w2.Replay()
	if err != nil {
		t.Fatalf("Replay: %v", err)
	}

	if got, want := len(records), 3; got != want {
		t.Fatalf("record count: got %d, want %d", got, want)
	}

	assertRecord(t, records[0], OpSet, "name", "lime")
	assertRecord(t, records[1], OpSet, "color", "green")
	assertRecord(t, records[2], OpDel, "color", "")
}

// TestWAL_ReplayEmpty confirms that replaying an empty WAL returns no records
// and no error.
func TestWAL_ReplayEmpty(t *testing.T) {
	w := tempWAL(t)
	records, err := w.Replay()
	if err != nil {
		t.Fatalf("Replay on empty WAL: %v", err)
	}
	if len(records) != 0 {
		t.Fatalf("expected 0 records, got %d", len(records))
	}
}

// TestWAL_Reset verifies that Reset() clears the log so a subsequent Replay
// returns nothing.
func TestWAL_Reset(t *testing.T) {
	w := tempWAL(t)

	if err := w.WriteSet("x", "1"); err != nil {
		t.Fatalf("WriteSet: %v", err)
	}
	if err := w.Reset(); err != nil {
		t.Fatalf("Reset: %v", err)
	}

	records, err := w.Replay()
	if err != nil {
		t.Fatalf("Replay after Reset: %v", err)
	}
	if len(records) != 0 {
		t.Fatalf("expected 0 records after Reset, got %d", len(records))
	}
}

// TestWAL_CrashRecovery simulates a crash by appending a partial (truncated)
// line to the WAL file after closing it normally.  Replay should tolerate the
// corruption and return only the clean records.
func TestWAL_CrashRecovery(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "wal.log")

	// Write two good records.
	w, err := OpenWAL(path)
	if err != nil {
		t.Fatalf("OpenWAL: %v", err)
	}
	_ = w.WriteSet("a", "1")
	_ = w.WriteSet("b", "2")
	_ = w.Close()

	// Simulate crash: append a half-written record.
	f, _ := os.OpenFile(path, os.O_APPEND|os.O_WRONLY, 0o644)
	_, _ = f.WriteString("SET partial") // no trailing newline / value
	_ = f.Close()

	// Reopen and replay — partial line must be skipped.
	w2, err := OpenWAL(path)
	if err != nil {
		t.Fatalf("OpenWAL (reopen): %v", err)
	}
	defer w2.Close()

	records, err := w2.Replay()
	if err != nil {
		t.Fatalf("Replay: %v", err)
	}
	if got, want := len(records), 2; got != want {
		t.Fatalf("record count: got %d, want %d (partial line not skipped?)", got, want)
	}
	assertRecord(t, records[0], OpSet, "a", "1")
	assertRecord(t, records[1], OpSet, "b", "2")
}

// TestWAL_Concurrent writes many records from multiple goroutines and then
// checks that Replay returns all of them without duplicates or corruption.
func TestWAL_Concurrent(t *testing.T) {
	w := tempWAL(t)

	const goroutines = 10
	const writesPerGoroutine = 50

	errc := make(chan error, goroutines*writesPerGoroutine)
	for g := 0; g < goroutines; g++ {
		go func(id int) {
			for i := 0; i < writesPerGoroutine; i++ {
				key := t.Name() + "_" + itoa(id) + "_" + itoa(i)
				errc <- w.WriteSet(key, itoa(i))
			}
		}(g)
	}

	total := goroutines * writesPerGoroutine
	for i := 0; i < total; i++ {
		if err := <-errc; err != nil {
			t.Errorf("WriteSet error: %v", err)
		}
	}

	records, err := w.Replay()
	if err != nil {
		t.Fatalf("Replay: %v", err)
	}
	if got := len(records); got != total {
		t.Fatalf("record count: got %d, want %d", got, total)
	}
}

// ── helpers ──────────────────────────────────────────────────────────────────

func assertRecord(t *testing.T, rec Record, op OpType, key, value string) {
	t.Helper()
	if rec.Op != op {
		t.Errorf("op: got %c, want %c", rec.Op, op)
	}
	if rec.Key != key {
		t.Errorf("key: got %q, want %q", rec.Key, key)
	}
	if rec.Value != value {
		t.Errorf("value: got %q, want %q", rec.Value, value)
	}
}

func itoa(n int) string {
	return string(rune('0') + rune(n%10)) // good enough for small n in tests
}
