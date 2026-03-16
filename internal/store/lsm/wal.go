// Package lsm implements an LSM-Tree (Log-Structured Merge-Tree) storage engine.
//
// The WAL (Write-Ahead Log) is the first layer: every mutation is appended to
// an append-only file before it is applied to the in-memory MemTable.  On a
// clean shutdown the WAL is removed; on a crash the WAL is replayed at startup
// to restore the MemTable to its last consistent state.
//
// File format — one record per line, human-readable:
//
//	SET <key> <value>\n
//	DEL <key>\n
//
// Keys and values MUST NOT contain whitespace.  A future iteration can switch
// to a length-prefixed binary format, but the line-based format is deliberately
// simple and easy to inspect with `cat` during development.
package lsm

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
)

// OpType identifies the kind of WAL record.
type OpType byte

const (
	OpSet OpType = 'S' // SET key value
	OpDel OpType = 'D' // DEL key
)

// Record is a single decoded WAL entry.
type Record struct {
	Op    OpType
	Key   string
	Value string // empty for DEL records
}

// WAL is a thread-safe, append-only write-ahead log backed by a single file.
// Callers must call Close when done; during normal operation the file is kept
// open and fsynced after every write for durability.
type WAL struct {
	mu   sync.Mutex
	f    *os.File
	path string
}

// OpenWAL opens (or creates) the WAL file at the given path.
// The parent directory is created if it does not exist.
func OpenWAL(path string) (*WAL, error) {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return nil, fmt.Errorf("wal: mkdir: %w", err)
	}

	f, err := os.OpenFile(path, os.O_CREATE|os.O_APPEND|os.O_RDWR, 0o644)
	if err != nil {
		return nil, fmt.Errorf("wal: open %s: %w", path, err)
	}

	return &WAL{f: f, path: path}, nil
}

// WriteSet appends a SET record and fsyncs.
func (w *WAL) WriteSet(key, value string) error {
	return w.append(fmt.Sprintf("SET %s %s\n", key, value))
}

// WriteDel appends a DEL record and fsyncs.
func (w *WAL) WriteDel(key string) error {
	return w.append(fmt.Sprintf("DEL %s\n", key))
}

// append writes a raw line to the WAL file and calls fsync.
// It holds the mutex for the entire duration so concurrent callers
// never interleave partial writes.
func (w *WAL) append(line string) error {
	w.mu.Lock()
	defer w.mu.Unlock()

	if _, err := fmt.Fprint(w.f, line); err != nil {
		return fmt.Errorf("wal: write: %w", err)
	}

	// fsync — block until the OS has flushed the page to durable storage.
	if err := w.f.Sync(); err != nil {
		return fmt.Errorf("wal: sync: %w", err)
	}

	return nil
}

// Replay reads the WAL file from the beginning and returns all valid records
// in the order they were written.  Truncated or malformed lines at the end
// (which can occur after a crash mid-write) are silently skipped so that
// recovery can proceed with the last fully-flushed state.
func (w *WAL) Replay() ([]Record, error) {
	w.mu.Lock()
	defer w.mu.Unlock()

	// Seek to the start of the file before reading.
	if _, err := w.f.Seek(0, 0); err != nil {
		return nil, fmt.Errorf("wal: seek: %w", err)
	}

	var records []Record
	scanner := bufio.NewScanner(w.f)

	for scanner.Scan() {
		line := scanner.Text()
		rec, err := parseLine(line)
		if err != nil {
			// Partial / corrupt record — skip and continue.
			// In production you'd log this; keep it silent here.
			continue
		}
		records = append(records, rec)
	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("wal: scan: %w", err)
	}

	return records, nil
}

// parseLine decodes a single WAL line.
func parseLine(line string) (Record, error) {
	line = strings.TrimSpace(line)
	if line == "" {
		return Record{}, fmt.Errorf("empty line")
	}

	parts := strings.SplitN(line, " ", 3)

	switch parts[0] {
	case "SET":
		if len(parts) != 3 {
			return Record{}, fmt.Errorf("malformed SET: %q", line)
		}
		return Record{Op: OpSet, Key: parts[1], Value: parts[2]}, nil

	case "DEL":
		if len(parts) != 2 {
			return Record{}, fmt.Errorf("malformed DEL: %q", line)
		}
		return Record{Op: OpDel, Key: parts[1]}, nil

	default:
		return Record{}, fmt.Errorf("unknown op %q in line: %q", parts[0], line)
	}
}

// Reset truncates the WAL file to zero bytes.
// Call this after a MemTable has been successfully flushed to an SSTable —
// at that point the WAL is no longer needed for recovery.
func (w *WAL) Reset() error {
	w.mu.Lock()
	defer w.mu.Unlock()

	if err := w.f.Truncate(0); err != nil {
		return fmt.Errorf("wal: truncate: %w", err)
	}
	// Seek back to the beginning so subsequent appends start at offset 0.
	if _, err := w.f.Seek(0, 0); err != nil {
		return fmt.Errorf("wal: seek after reset: %w", err)
	}
	return nil
}

// Close flushes and closes the underlying file.
func (w *WAL) Close() error {
	w.mu.Lock()
	defer w.mu.Unlock()

	if err := w.f.Sync(); err != nil {
		return fmt.Errorf("wal: sync on close: %w", err)
	}
	return w.f.Close()
}

// Path returns the absolute path of the WAL file.
func (w *WAL) Path() string { return w.path }
