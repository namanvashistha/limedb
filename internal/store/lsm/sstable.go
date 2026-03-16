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
//       [4B key_len][4B val_len][1B flags][key_bytes][val_bytes]
//
//       flags bit 0: 1 = tombstone (deleted). When set, val_len is 0.
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
)

const (
	// ssMagic is written in the footer so we can detect corrupt / truncated files.
	ssMagic = uint64(0x4C696D65_53535401) // "LimeSST\x01"

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

	// [4B key_len][4B val_len][1B flags][key_bytes][val_bytes]
	keyBytes := []byte(e.Key)
	n, err := writeRecord(w.f, keyBytes, valBytes, flags)
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
		return fmt.Errorf("sstable: bad magic 0x%X (corrupt or wrong format)", magic)
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
// Returns (value, true) if found and not a tombstone.
// Returns ("", false) if absent.
// Returns (tombstone sentinel, true) — actually no: callers receive ("", false)
// for tombstones, but IsTombstone lets compaction distinguish "not found" from
// "found but deleted".
func (r *SSTableReader) Get(key string) (string, bool) {
	val, found, _ := r.get(key)
	return val, found
}

// IsTombstone reports whether key is present as a tombstone in this SSTable.
// This is used during compaction to decide whether to propagate or drop a record.
func (r *SSTableReader) IsTombstone(key string) bool {
	_, _, isTomb := r.get(key)
	return isTomb
}

// get is the internal implementation that also exposes the tombstone flag.
func (r *SSTableReader) get(key string) (value string, found bool, isTombstone bool) {
	if len(r.index) == 0 {
		return "", false, false
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
		return "", false, false
	}

	for {
		if endOffset > 0 {
			cur, _ := r.f.Seek(0, io.SeekCurrent)
			if cur >= endOffset {
				break
			}
		}

		k, v, flags, err := readRecord(r.f)
		if err != nil {
			break
		}

		if k == key {
			if flags&flagTombstone != 0 {
				return "", false, true // tombstone
			}
			return v, true, false
		}
		// Since data is sorted, if we've passed the target key, stop early.
		if k > key {
			break
		}
	}
	return "", false, false
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
		k, v, flags, err := readRecord(r.f)
		if err != nil {
			if err == io.EOF || err == io.ErrUnexpectedEOF {
				break
			}
			return nil, err
		}
		entries = append(entries, Entry{
			Key:     k,
			Value:   v,
			Deleted: flags&flagTombstone != 0,
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
func writeRecord(w io.Writer, key, val []byte, flags byte) (int, error) {
	total := 0

	keyLen := uint32(len(key))
	valLen := uint32(len(val))

	if err := binary.Write(w, binary.LittleEndian, keyLen); err != nil {
		return total, err
	}
	total += 4

	if err := binary.Write(w, binary.LittleEndian, valLen); err != nil {
		return total, err
	}
	total += 4

	if _, err := w.Write([]byte{flags}); err != nil {
		return total, err
	}
	total++

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
func readRecord(r io.Reader) (key, value string, flags byte, err error) {
	var keyLen, valLen uint32
	if err = binary.Read(r, binary.LittleEndian, &keyLen); err != nil {
		return
	}
	if err = binary.Read(r, binary.LittleEndian, &valLen); err != nil {
		return
	}

	flagBuf := make([]byte, 1)
	if _, err = io.ReadFull(r, flagBuf); err != nil {
		return
	}
	flags = flagBuf[0]

	keyBuf := make([]byte, keyLen)
	if _, err = io.ReadFull(r, keyBuf); err != nil {
		return
	}
	key = string(keyBuf)

	if valLen > 0 {
		valBuf := make([]byte, valLen)
		if _, err = io.ReadFull(r, valBuf); err != nil {
			return
		}
		value = string(valBuf)
	}
	return
}
