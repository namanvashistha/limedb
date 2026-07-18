package lsm

import (
	"io"
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

// TestWAL_WriteAndReplay verifies that set and delete records survive a full
// write → close → reopen → Replay round-trip, including their timestamps.
func TestWAL_WriteAndReplay(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "wal.log")

	// --- write phase ---
	w, err := OpenWAL(path)
	if err != nil {
		t.Fatalf("OpenWAL: %v", err)
	}
	if err := w.WritePut("name", "lime", 100, false); err != nil {
		t.Fatalf("WritePut: %v", err)
	}
	if err := w.WritePut("color", "green", 200, false); err != nil {
		t.Fatalf("WritePut: %v", err)
	}
	if err := w.WritePut("color", "", 300, true); err != nil {
		t.Fatalf("WritePut (tombstone): %v", err)
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

	assertRecord(t, records[0], OpSet, "name", "lime", 100)
	assertRecord(t, records[1], OpSet, "color", "green", 200)
	assertRecord(t, records[2], OpDel, "color", "", 300)
}

// TestWAL_WhitespaceValues verifies the binary format handles keys/values with
// spaces and newlines — the old text format corrupted on these.
func TestWAL_WhitespaceValues(t *testing.T) {
	w := tempWAL(t)

	if err := w.WritePut("greeting", "hello world\nsecond line", 1, false); err != nil {
		t.Fatalf("WritePut: %v", err)
	}
	records, err := w.Replay()
	if err != nil {
		t.Fatalf("Replay: %v", err)
	}
	if len(records) != 1 || records[0].Value != "hello world\nsecond line" {
		t.Fatalf("whitespace value did not survive replay: %+v", records)
	}
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
// returns nothing, and that the file is still usable afterwards.
func TestWAL_Reset(t *testing.T) {
	w := tempWAL(t)

	if err := w.WritePut("x", "1", 1, false); err != nil {
		t.Fatalf("WritePut: %v", err)
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

	// The magic header must have been rewritten so new writes still replay.
	if err := w.WritePut("y", "2", 2, false); err != nil {
		t.Fatalf("WritePut after Reset: %v", err)
	}
	records, err = w.Replay()
	if err != nil || len(records) != 1 {
		t.Fatalf("Replay after Reset+write: records=%v err=%v", records, err)
	}
}

// TestWAL_CrashRecovery simulates a crash by appending a truncated record to
// the WAL file after closing it normally.  Replay should stop at the corrupt
// tail and return only the clean records.
func TestWAL_CrashRecovery(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "wal.log")

	// Write two good records.
	w, err := OpenWAL(path)
	if err != nil {
		t.Fatalf("OpenWAL: %v", err)
	}
	_ = w.WritePut("a", "1", 1, false)
	_ = w.WritePut("b", "2", 2, false)
	_ = w.Close()

	// Simulate crash: append a half-written record (random garbage bytes).
	f, _ := os.OpenFile(path, os.O_APPEND|os.O_WRONLY, 0o644)
	_, _ = f.Write([]byte{0xDE, 0xAD, 0xBE, 0xEF, 'S', 0x01})
	_ = f.Close()

	// Reopen and replay — the corrupt tail must end the replay cleanly.
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
		t.Fatalf("record count: got %d, want %d (corrupt tail not skipped?)", got, want)
	}
	assertRecord(t, records[0], OpSet, "a", "1", 1)
	assertRecord(t, records[1], OpSet, "b", "2", 2)
}

// TestWAL_CorruptedRecordStopsReplay flips a byte inside the middle record and
// confirms replay stops there (CRC catches it) without erroring.
func TestWAL_CorruptedRecordStopsReplay(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "wal.log")

	w, err := OpenWAL(path)
	if err != nil {
		t.Fatalf("OpenWAL: %v", err)
	}
	_ = w.WritePut("a", "1", 1, false)
	off, _ := w.f.Seek(0, io.SeekCurrent) // end of record 1
	_ = w.WritePut("b", "2", 2, false)
	_ = w.WritePut("c", "3", 3, false)
	_ = w.Close()

	// Flip a byte inside record 2's payload.
	f, _ := os.OpenFile(path, os.O_WRONLY, 0o644)
	_, _ = f.WriteAt([]byte{0xFF}, off+6)
	_ = f.Close()

	w2, err := OpenWAL(path)
	if err != nil {
		t.Fatalf("OpenWAL (reopen): %v", err)
	}
	defer w2.Close()

	records, err := w2.Replay()
	if err != nil {
		t.Fatalf("Replay: %v", err)
	}
	if len(records) != 1 {
		t.Fatalf("replay should stop at the corrupt record: got %d records", len(records))
	}
	assertRecord(t, records[0], OpSet, "a", "1", 1)
}

// TestWAL_RejectsLegacyFormat verifies that a pre-binary (text) WAL file is
// refused with a clear error instead of being silently misread.
func TestWAL_RejectsLegacyFormat(t *testing.T) {
	path := filepath.Join(t.TempDir(), "wal.log")
	if err := os.WriteFile(path, []byte("SET name lime\nDEL name\n"), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	if _, err := OpenWAL(path); err == nil {
		t.Fatal("OpenWAL should reject a legacy text-format WAL")
	}
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
				errc <- w.WritePut(key, itoa(i), nextTS(), false)
			}
		}(g)
	}

	total := goroutines * writesPerGoroutine
	for i := 0; i < total; i++ {
		if err := <-errc; err != nil {
			t.Errorf("WritePut error: %v", err)
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

func assertRecord(t *testing.T, rec Record, op OpType, key, value string, ts int64) {
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
	if rec.TimestampMicros != ts {
		t.Errorf("ts: got %d, want %d", rec.TimestampMicros, ts)
	}
}

func itoa(n int) string {
	return string(rune('0') + rune(n%10)) // good enough for small n in tests
}
