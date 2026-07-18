// Package lsm implements an LSM-Tree (Log-Structured Merge-Tree) storage engine.
//
// The WAL (Write-Ahead Log) is the first layer: every mutation is appended to
// an append-only file before it is applied to the in-memory MemTable.  On a
// clean shutdown the WAL is removed; on a crash the WAL is replayed at startup
// to restore the MemTable to its last consistent state.
//
// File format — binary, CRC-protected:
//
//	[8B magic "LIMEWAL2"]                              file header
//	[4B crc32][1B op][8B ts_micros][4B key_len][4B val_len][key][val]   per record
//
// op is 'S' (set) or 'D' (delete tombstone; val_len is 0). The CRC covers
// everything after itself in the record. Replay stops at the first record
// that fails its CRC or is truncated — that is the crash point, and every
// record before it was fully fsynced.
package lsm

import (
	"encoding/binary"
	"fmt"
	"hash/crc32"
	"io"
	"os"
	"path/filepath"
	"sync"
)

// walMagic identifies the binary WAL format. A file that does not start with
// this magic is from an incompatible (pre-timestamp) version of LimeDB.
var walMagic = []byte("LIMEWAL2")

// OpType identifies the kind of WAL record.
type OpType byte

const (
	OpSet OpType = 'S' // set key = value
	OpDel OpType = 'D' // delete (tombstone)
)

// Record is a single decoded WAL entry.
type Record struct {
	Op              OpType
	Key             string
	Value           string // empty for OpDel records
	TimestampMicros int64
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
// The parent directory is created if it does not exist. An empty file gets
// the magic header written; a non-empty file that lacks the magic is
// rejected — the data dir predates the binary WAL format and must be wiped.
func OpenWAL(path string) (*WAL, error) {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return nil, fmt.Errorf("wal: mkdir: %w", err)
	}

	f, err := os.OpenFile(path, os.O_CREATE|os.O_APPEND|os.O_RDWR, 0o644)
	if err != nil {
		return nil, fmt.Errorf("wal: open %s: %w", path, err)
	}

	w := &WAL{f: f, path: path}
	if err := w.checkOrWriteMagic(); err != nil {
		f.Close()
		return nil, err
	}
	return w, nil
}

// checkOrWriteMagic writes the magic header to an empty file, or verifies it
// on an existing one.
func (w *WAL) checkOrWriteMagic() error {
	info, err := w.f.Stat()
	if err != nil {
		return fmt.Errorf("wal: stat: %w", err)
	}
	if info.Size() == 0 {
		if _, err := w.f.Write(walMagic); err != nil {
			return fmt.Errorf("wal: write magic: %w", err)
		}
		return w.f.Sync()
	}

	header := make([]byte, len(walMagic))
	if _, err := w.f.ReadAt(header, 0); err != nil {
		return fmt.Errorf("wal: read magic: %w", err)
	}
	if string(header) != string(walMagic) {
		return fmt.Errorf("wal: %s is not a %s file — data dir was written by an incompatible LimeDB version, wipe it and restart", w.path, walMagic)
	}
	return nil
}

// WritePut appends a record for the given versioned value (tombstone or not)
// and fsyncs.
func (w *WAL) WritePut(key, value string, tsMicros int64, tombstone bool) error {
	op := OpSet
	if tombstone {
		op = OpDel
		value = ""
	}

	// Build the CRC-covered payload: [op][ts][key_len][val_len][key][val].
	payload := make([]byte, 0, 1+8+4+4+len(key)+len(value))
	payload = append(payload, byte(op))
	payload = binary.LittleEndian.AppendUint64(payload, uint64(tsMicros))
	payload = binary.LittleEndian.AppendUint32(payload, uint32(len(key)))
	payload = binary.LittleEndian.AppendUint32(payload, uint32(len(value)))
	payload = append(payload, key...)
	payload = append(payload, value...)

	record := make([]byte, 0, 4+len(payload))
	record = binary.LittleEndian.AppendUint32(record, crc32.ChecksumIEEE(payload))
	record = append(record, payload...)

	w.mu.Lock()
	defer w.mu.Unlock()

	if _, err := w.f.Write(record); err != nil {
		return fmt.Errorf("wal: write: %w", err)
	}

	// fsync — block until the OS has flushed the page to durable storage.
	if err := w.f.Sync(); err != nil {
		return fmt.Errorf("wal: sync: %w", err)
	}

	return nil
}

// Replay reads the WAL file from the beginning and returns all valid records
// in the order they were written.  A truncated or CRC-corrupt record at the
// end (which can occur after a crash mid-write) ends the replay; everything
// before it is the last fully-flushed state.
func (w *WAL) Replay() ([]Record, error) {
	w.mu.Lock()
	defer w.mu.Unlock()

	if _, err := w.f.Seek(int64(len(walMagic)), io.SeekStart); err != nil {
		return nil, fmt.Errorf("wal: seek: %w", err)
	}

	var records []Record
	for {
		rec, err := readWALRecord(w.f)
		if err != nil {
			// Truncated or corrupt tail — recovery proceeds with what we have.
			break
		}
		records = append(records, rec)
	}

	// Return the write cursor to the end for subsequent appends.
	if _, err := w.f.Seek(0, io.SeekEnd); err != nil {
		return nil, fmt.Errorf("wal: seek end: %w", err)
	}
	return records, nil
}

// readWALRecord decodes one record, verifying its CRC.
func readWALRecord(r io.Reader) (Record, error) {
	var crc uint32
	if err := binary.Read(r, binary.LittleEndian, &crc); err != nil {
		return Record{}, err
	}

	fixed := make([]byte, 1+8+4+4)
	if _, err := io.ReadFull(r, fixed); err != nil {
		return Record{}, err
	}
	op := OpType(fixed[0])
	ts := int64(binary.LittleEndian.Uint64(fixed[1:9]))
	keyLen := binary.LittleEndian.Uint32(fixed[9:13])
	valLen := binary.LittleEndian.Uint32(fixed[13:17])

	kv := make([]byte, int(keyLen)+int(valLen))
	if _, err := io.ReadFull(r, kv); err != nil {
		return Record{}, err
	}

	if crc32.ChecksumIEEE(append(fixed, kv...)) != crc {
		return Record{}, fmt.Errorf("wal: crc mismatch")
	}
	if op != OpSet && op != OpDel {
		return Record{}, fmt.Errorf("wal: unknown op %q", op)
	}

	return Record{
		Op:              op,
		Key:             string(kv[:keyLen]),
		Value:           string(kv[keyLen:]),
		TimestampMicros: ts,
	}, nil
}

// Reset truncates the WAL back to just the magic header.
// Call this after a MemTable has been successfully flushed to an SSTable —
// at that point the WAL is no longer needed for recovery.
func (w *WAL) Reset() error {
	w.mu.Lock()
	defer w.mu.Unlock()

	if err := w.f.Truncate(0); err != nil {
		return fmt.Errorf("wal: truncate: %w", err)
	}
	if _, err := w.f.Seek(0, io.SeekStart); err != nil {
		return fmt.Errorf("wal: seek after reset: %w", err)
	}
	if _, err := w.f.Write(walMagic); err != nil {
		return fmt.Errorf("wal: rewrite magic: %w", err)
	}
	return w.f.Sync()
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
