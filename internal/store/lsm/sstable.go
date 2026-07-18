package lsm

// SSTable — Sorted String Table
//
// An SSTable is an immutable, sorted on-disk file written when a MemTable is
// flushed.  Multiple SSTables accumulate over time; compaction merges them.
//
// ── File format ──────────────────────────────────────────────────────────────
//
// A single ".sst" file contains three sections in order:
//
//  1. DATA SECTION   — packed binary records, one per entry:
//
//       [4B key_len][4B val_len][1B flags][8B ts_micros][key_bytes][val_bytes]
//
//       flags bit 0: 1 = tombstone (deleted). When set, val_len is 0.
//       ts_micros is the LWW write timestamp assigned by the coordinator.
//
//  2. INDEX SECTION  — sparse index (every indexInterval-th entry):
//
//       [4B key_len][key_bytes][8B data_offset]
//
//       data_offset is the byte position of the corresponding DATA record.
//       Binary-searching the index gives us the block to start a linear scan.
//
//  3. FOOTER (fixed 24 bytes at EOF):
//
//       [8B index_offset][8B index_count][8B magic]
//
//       magic = ssMagic (defined below). Used to detect truncated files.
//
// ─────────────────────────────────────────────────────────────────────────────

import (
	"encoding/binary"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"

	"limedb/internal/store"
)

const (
	// ssMagic is written in the footer so we can detect corrupt / truncated files.
	// Version 2 added the per-record timestamp; v1 files are rejected.
	ssMagic = uint64(0x4C696D65_53535402) // "LimeSST\x02"

	// footerSize is the fixed size of the SSTable footer in bytes.
	footerSize = 24 // 8 (index offset) + 8 (index count) + 8 (magic)

	// flagTombstone is set in the flags byte when the entry is a deletion marker.
	flagTombstone = byte(1)

	// indexInterval controls how dense the sparse index is: one index entry per
	// N data records. Lower = faster seeks, larger index footprint.
	indexInterval = 16
)

// ── Writer ───────────────────────────────────────────────────────────────────

// SSTableWriter writes a sorted sequence of MemTable entries to a .sst file.
// Call Flush() once at the end — it writes the index and footer, then closes
// the file.  The caller must pass entries in sorted key order (MemTable.Entries
// already guarantees this).
type SSTableWriter struct {
	f          *os.File
	path       string
	dataOffset int64        // current write position in the data section
	index      []indexEntry // sparse index accumulation
	count      int          // total entries written
}

type indexEntry struct {
	key    string
	offset int64
}

// NewSSTableWriter creates a new SSTable file at path.
// The file must not exist; call os.Remove first if you need to overwrite.
func NewSSTableWriter(path string) (*SSTableWriter, error) {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return nil, fmt.Errorf("sstable: mkdir: %w", err)
	}
	f, err := os.OpenFile(path, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o644)
	if err != nil {
		return nil, fmt.Errorf("sstable: create %s: %w", path, err)
	}
	return &SSTableWriter{f: f, path: path}, nil
}

// WriteEntry appends one entry to the data section.
// Entries MUST be passed in sorted key order.
func (w *SSTableWriter) WriteEntry(e Entry) error {
	// Record index entry for every Nth record.
	if w.count%indexInterval == 0 {
		w.index = append(w.index, indexEntry{key: e.Key, offset: w.dataOffset})
	}

	var flags byte
	valBytes := []byte(e.Value)
	if e.Deleted {
		flags = flagTombstone
		valBytes = nil
	}

	// [4B key_len][4B val_len][1B flags][8B ts_micros][key_bytes][val_bytes]
	keyBytes := []byte(e.Key)
	n, err := writeRecord(w.f, keyBytes, valBytes, flags, e.TimestampMicros)
	if err != nil {
		return fmt.Errorf("sstable: write entry %q: %w", e.Key, err)
	}
	w.dataOffset += int64(n)
	w.count++
	return nil
}

// Flush writes the index section and footer, then syncs and closes the file.
// After Flush the writer is no longer usable.
func (w *SSTableWriter) Flush() error {
	indexStart := w.dataOffset

	// Write index section.
	idxCount := int64(len(w.index))
	for _, ie := range w.index {
		keyBytes := []byte(ie.key)
		// [4B key_len][key_bytes][8B offset]
		if err := binary.Write(w.f, binary.LittleEndian, uint32(len(keyBytes))); err != nil {
			return fmt.Errorf("sstable: index key_len: %w", err)
		}
		if _, err := w.f.Write(keyBytes); err != nil {
			return fmt.Errorf("sstable: index key: %w", err)
		}
		if err := binary.Write(w.f, binary.LittleEndian, ie.offset); err != nil {
			return fmt.Errorf("sstable: index offset: %w", err)
		}
	}

	// Write footer: [8B index_offset][8B index_count][8B magic].
	footer := [3]uint64{uint64(indexStart), uint64(idxCount), ssMagic}
	if err := binary.Write(w.f, binary.LittleEndian, footer); err != nil {
		return fmt.Errorf("sstable: footer: %w", err)
	}

	if err := w.f.Sync(); err != nil {
		return fmt.Errorf("sstable: sync: %w", err)
	}
	return w.f.Close()
}

// Path returns the file path of the SSTable being written.
func (w *SSTableWriter) Path() string { return w.path }

// ── Reader ───────────────────────────────────────────────────────────────────

// SSTableReader reads data from an immutable .sst file.
// The sparse index is loaded into memory on Open; each point lookup does a
// binary search of the index followed by a short linear scan in the file.
type SSTableReader struct {
	f     *os.File
	index []indexEntry // in-memory sparse index
	path  string
}

// OpenSSTable opens an existing SSTable file and loads its index.
func OpenSSTable(path string) (*SSTableReader, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("sstable: open %s: %w", path, err)
	}

	r := &SSTableReader{f: f, path: path}
	if err := r.loadIndex(); err != nil {
		f.Close()
		return nil, err
	}
	return r, nil
}

// loadIndex reads the footer then the index section into memory.
func (r *SSTableReader) loadIndex() error {
	// Seek to footer.
	if _, err := r.f.Seek(-footerSize, io.SeekEnd); err != nil {
		return fmt.Errorf("sstable: seek footer: %w", err)
	}

	var footer [3]uint64
	if err := binary.Read(r.f, binary.LittleEndian, &footer); err != nil {
		return fmt.Errorf("sstable: read footer: %w", err)
	}
	indexOffset := int64(footer[0])
	indexCount := int(footer[1])
	magic := footer[2]

	if magic != ssMagic {
		return fmt.Errorf("sstable: bad magic 0x%X — corrupt, truncated, or written by an incompatible LimeDB version (wipe the data dir and restart)", magic)
	}

	// Seek to index section.
	if _, err := r.f.Seek(indexOffset, io.SeekStart); err != nil {
		return fmt.Errorf("sstable: seek index: %w", err)
	}

	r.index = make([]indexEntry, 0, indexCount)
	for i := 0; i < indexCount; i++ {
		var keyLen uint32
		if err := binary.Read(r.f, binary.LittleEndian, &keyLen); err != nil {
			return fmt.Errorf("sstable: index key_len[%d]: %w", i, err)
		}
		keyBytes := make([]byte, keyLen)
		if _, err := io.ReadFull(r.f, keyBytes); err != nil {
			return fmt.Errorf("sstable: index key[%d]: %w", i, err)
		}
		var offset int64
		if err := binary.Read(r.f, binary.LittleEndian, &offset); err != nil {
			return fmt.Errorf("sstable: index offset[%d]: %w", i, err)
		}
		r.index = append(r.index, indexEntry{key: string(keyBytes), offset: offset})
	}
	return nil
}

// Get looks up key in the SSTable.
// Returns (record, true) if the key is present, including tombstones —
// callers check .Tombstone to distinguish a delete from a live value.
func (r *SSTableReader) Get(key string) (store.VersionedValue, bool) {
	if len(r.index) == 0 {
		return store.VersionedValue{}, false
	}

	// Binary search: find the last index entry whose key <= target key.
	pos := sort.Search(len(r.index), func(i int) bool {
		return r.index[i].key > key
	}) - 1

	if pos < 0 {
		pos = 0
	}

	startOffset := r.index[pos].offset

	// Determine the scan limit: next index entry's offset (or index section start).
	var endOffset int64
	if pos+1 < len(r.index) {
		endOffset = r.index[pos+1].offset
	} else {
		// No next index entry — scan until the index section (footer[0]).
		// We stored it in r.index[0].offset only if the table has entries;
		// instead peek at the file footer again.
		if _, err := r.f.Seek(-footerSize, io.SeekEnd); err == nil {
			var ftr [3]uint64
			if binary.Read(r.f, binary.LittleEndian, &ftr) == nil {
				endOffset = int64(ftr[0])
			}
		}
	}

	// Linear scan from startOffset, stopping at endOffset.
	if _, err := r.f.Seek(startOffset, io.SeekStart); err != nil {
		return store.VersionedValue{}, false
	}

	for {
		if endOffset > 0 {
			cur, _ := r.f.Seek(0, io.SeekCurrent)
			if cur >= endOffset {
				break
			}
		}

		k, v, flags, ts, err := readRecord(r.f)
		if err != nil {
			break
		}

		if k == key {
			return store.VersionedValue{
				Value:           v,
				TimestampMicros: ts,
				Tombstone:       flags&flagTombstone != 0,
			}, true
		}
		// Since data is sorted, if we've passed the target key, stop early.
		if k > key {
			break
		}
	}
	return store.VersionedValue{}, false
}

// Entries returns all entries in sorted order, including tombstones.
// Used by compaction to iterate an entire SSTable.
func (r *SSTableReader) Entries() ([]Entry, error) {
	// Read from the start up to the index section.
	if _, err := r.f.Seek(-footerSize, io.SeekEnd); err != nil {
		return nil, fmt.Errorf("sstable: seek footer: %w", err)
	}
	var ftr [3]uint64
	if err := binary.Read(r.f, binary.LittleEndian, &ftr); err != nil {
		return nil, fmt.Errorf("sstable: read footer: %w", err)
	}
	indexOffset := int64(ftr[0])

	if _, err := r.f.Seek(0, io.SeekStart); err != nil {
		return nil, err
	}

	var entries []Entry
	for {
		cur, _ := r.f.Seek(0, io.SeekCurrent)
		if cur >= indexOffset {
			break
		}
		k, v, flags, ts, err := readRecord(r.f)
		if err != nil {
			if err == io.EOF || err == io.ErrUnexpectedEOF {
				break
			}
			return nil, err
		}
		entries = append(entries, Entry{
			Key:             k,
			Value:           v,
			TimestampMicros: ts,
			Deleted:         flags&flagTombstone != 0,
		})
	}
	return entries, nil
}

// Close releases the file handle.
func (r *SSTableReader) Close() error { return r.f.Close() }

// Path returns the file path of this SSTable.
func (r *SSTableReader) Path() string { return r.path }

// ── low-level record I/O ─────────────────────────────────────────────────────

// writeRecord encodes one record and writes it to w.
// Returns the number of bytes written.
func writeRecord(w io.Writer, key, val []byte, flags byte, tsMicros int64) (int, error) {
	header := make([]byte, 0, 4+4+1+8)
	header = binary.LittleEndian.AppendUint32(header, uint32(len(key)))
	header = binary.LittleEndian.AppendUint32(header, uint32(len(val)))
	header = append(header, flags)
	header = binary.LittleEndian.AppendUint64(header, uint64(tsMicros))

	total := 0
	if n, err := w.Write(header); err != nil {
		return total + n, err
	}
	total += len(header)

	if _, err := w.Write(key); err != nil {
		return total, err
	}
	total += len(key)

	if len(val) > 0 {
		if _, err := w.Write(val); err != nil {
			return total, err
		}
		total += len(val)
	}

	return total, nil
}

// readRecord decodes one record from r.
func readRecord(r io.Reader) (key, value string, flags byte, tsMicros int64, err error) {
	header := make([]byte, 4+4+1+8)
	if _, err = io.ReadFull(r, header); err != nil {
		return
	}
	keyLen := binary.LittleEndian.Uint32(header[0:4])
	valLen := binary.LittleEndian.Uint32(header[4:8])
	flags = header[8]
	tsMicros = int64(binary.LittleEndian.Uint64(header[9:17]))

	kv := make([]byte, int(keyLen)+int(valLen))
	if _, err = io.ReadFull(r, kv); err != nil {
		return
	}
	key = string(kv[:keyLen])
	value = string(kv[keyLen:])
	return
}
