package lsm

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

// Store implements store.Backend using the LSM-Tree components:
// WAL for crash recovery, MemTable for fast writes, and SSTables
// (with Bloom filters) + Compaction for durable reads.
type Store struct {
	dir string

	mu        sync.RWMutex
	wal       *WAL
	mem       *MemTable
	sstables  []*SSTableReader // newest first
	compactor *Compactor

	flushThreshold  int64 // MemTable flush size in bytes
	compactionCount int
	closeOnce       sync.Once

	bloomFalsePositives uint64
	bloomTruePositives  uint64

	isCompacting             int32
	lastCompactionDurationMs uint64
}

// Config parameterises the LSM Store.
type Config struct {
	// Dir is where all WAL and SSTable files are stored.
	Dir string
	// MemTableFlushBytes is the threshold (approximate) at which the MemTable
	// is flushed to an SSTable. Defaults to 4 MB.
	MemTableFlushBytes int64
	// CompactionThreshold is how many SSTables trigger a merge. Defaults to 4.
	CompactionThreshold int
}

// NewStore opens or creates an LSM store at the given directory.
// It replays the WAL if one exists to restore the in-memory state.
func NewStore(cfg Config) (*Store, error) {
	if cfg.MemTableFlushBytes <= 0 {
		cfg.MemTableFlushBytes = 4 * 1024 * 1024 // 4 MB default
	}
	if cfg.CompactionThreshold <= 0 {
		cfg.CompactionThreshold = 4
	}

	if err := os.MkdirAll(cfg.Dir, 0o755); err != nil {
		return nil, fmt.Errorf("lsm: mkdir: %w", err)
	}

	s := &Store{
		dir:            cfg.Dir,
		mem:            NewMemTable(),
		flushThreshold: cfg.MemTableFlushBytes,
	}

	// 1. Load existing SSTables.
	if err := s.loadSSTables(); err != nil {
		return nil, err
	}

	// 2. Open and replay WAL.
	walPath := filepath.Join(cfg.Dir, "wal.log")
	wal, err := OpenWAL(walPath)
	if err != nil {
		s.closeAllSSTables()
		return nil, err
	}
	s.wal = wal

	records, err := s.wal.Replay()
	if err != nil {
		s.wal.Close()
		s.closeAllSSTables()
		return nil, fmt.Errorf("lsm: wal replay: %w", err)
	}
	s.mem.RestoreFromWAL(records)

	// 3. Start Compactor.
	cCfg := CompactorConfig{
		Dir:       cfg.Dir,
		Threshold: cfg.CompactionThreshold,
		OnCompactionStart: func() {
			atomic.StoreInt32(&s.isCompacting, 1)
		},
		OnCompacted: func(inputs []string, output string, durationMs int64) {
			atomic.StoreInt32(&s.isCompacting, 0)
			atomic.StoreUint64(&s.lastCompactionDurationMs, uint64(durationMs))
			s.replaceSSTables(inputs, output)
		},
		OnError: func(err error) {
			atomic.StoreInt32(&s.isCompacting, 0)
			fmt.Fprintf(os.Stderr, "lsm: compaction error: %v\n", err)
		},
	}
	s.compactor = NewCompactor(cCfg)

	// Register existing tables with the compactor.
	// (c.tables expects newest first, which matches s.sstables)
	for _, r := range s.sstables {
		s.compactor.Add(r.Path())
	}
	s.compactor.Start()

	return s, nil
}

// Get implements store.Backend.
func (s *Store) Get(key string) (string, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	// 1. Check MemTable.
	if val, ok := s.mem.Get(key); ok {
		return val, true
	}
	if s.mem.IsDeleted(key) {
		return "", false // tombstone shadows disk
	}

	// 2. Check SSTables (newest to oldest).
	for _, table := range s.sstables {
		hasBloom := false
		bloomMayContain := false

		bf, err := ReadBloomFile(table.Path())
		if err == nil && bf != nil {
			hasBloom = true
			if !bf.MayContain(key) {
				continue // skip this SSTable entirely
			}
			bloomMayContain = true
		}

		val, found := table.Get(key)
		if found {
			if hasBloom && bloomMayContain {
				atomic.AddUint64(&s.bloomTruePositives, 1)
			}
			return val, true
		}
		if table.IsTombstone(key) {
			if hasBloom && bloomMayContain {
				atomic.AddUint64(&s.bloomTruePositives, 1)
			}
			return "", false // tombstone shadows older SSTables
		}

		// If bloomMayContain was true but we didn't find it or a tombstone, it's a false positive.
		if hasBloom && bloomMayContain {
			atomic.AddUint64(&s.bloomFalsePositives, 1)
		}
	}

	return "", false
}

// Set implements store.Backend.
func (s *Store) Set(key, value string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	// 1. Write to WAL.
	if err := s.wal.WriteSet(key, value); err != nil {
		fmt.Fprintf(os.Stderr, "lsm: wal write error (Set): %v\n", err)
		return
	}

	// 2. Apply to MemTable.
	s.mem.Set(key, value)

	// 3. Check flush threshold.
	if s.mem.ApproximateSize() >= s.flushThreshold {
		s.flushMemTable()
	}
}

// Delete implements store.Backend.
func (s *Store) Delete(key string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()

	// We return `bool` (did the key exist before deleting?), which requires
	// a look-up. In a pure write-optimised LSM, `Delete` is usually blind
	// (just writes a tombstone), but since `store.Backend` requires a bool
	// return, we emulate it.
	// To avoid deadlocks since we hold mu.Lock, we do the check lock-free
	// by directly calling internal methods using the existing RWMutex rules (or
	// since we hold the exclusive lock we don't need read locks, but the
	// methods take their own locks so we must drop ours to call public Get).
	s.mu.Unlock()
	_, existed := s.Get(key)
	s.mu.Lock()

	// 1. Write tombstone to WAL.
	if err := s.wal.WriteDel(key); err != nil {
		fmt.Fprintf(os.Stderr, "lsm: wal write error (Del): %v\n", err)
		return false
	}

	// 2. Write tombstone to MemTable.
	s.mem.Delete(key)

	// 3. Check flush threshold.
	if s.mem.ApproximateSize() >= s.flushThreshold {
		s.flushMemTable()
	}

	return existed
}

// ListKeys implements store.Backend.
// It merges keys from MemTable and all SSTables, dropping tombstones.
func (s *Store) ListKeys() []string {
	s.mu.RLock()
	defer s.mu.RUnlock()

	keyMap := make(map[string]bool)

	// Scan oldest to newest so newest values/tombstones overwrite older ones.
	for i := len(s.sstables) - 1; i >= 0; i-- {
		entries, _ := s.sstables[i].Entries()
		for _, e := range entries {
			if e.Deleted {
				delete(keyMap, e.Key)
			} else {
				keyMap[e.Key] = true
			}
		}
	}

	// Read MemTable last (highest priority).
	for _, e := range s.mem.Entries() {
		if e.Deleted {
			delete(keyMap, e.Key)
		} else {
			keyMap[e.Key] = true
		}
	}

	keys := make([]string, 0, len(keyMap))
	for k := range keyMap {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

// Count implements store.Backend.
func (s *Store) Count() int {
	return len(s.ListKeys())
}

// Stats returns internal metrics about the LSM tree.
func (s *Store) Stats() map[string]interface{} {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var totalDiskUsage int64

	// WAL file size
	if walInfo, err := os.Stat(filepath.Join(s.dir, "wal.log")); err == nil {
		totalDiskUsage += walInfo.Size()
	}

	// SSTables & Bloom filters sizes
	for _, sst := range s.sstables {
		if info, err := os.Stat(sst.Path()); err == nil {
			totalDiskUsage += info.Size()
		}
		if info, err := os.Stat(bloomPath(sst.Path())); err == nil {
			totalDiskUsage += info.Size()
		}
	}

	fp := atomic.LoadUint64(&s.bloomFalsePositives)
	tp := atomic.LoadUint64(&s.bloomTruePositives)
	fpRate := 0.0
	if fp+tp > 0 {
		fpRate = float64(fp) / float64(fp+tp)
	}

	return map[string]interface{}{
		"type":                        "lsm",
		"memtable_size_b":             s.mem.ApproximateSize(),
		"memtable_keys":               s.mem.Len(),
		"flush_threshold_b":           s.flushThreshold,
		"sstable_count":               len(s.sstables),
		"compaction_count":            s.compactionCount,
		"is_compacting":               atomic.LoadInt32(&s.isCompacting) == 1,
		"last_compaction_duration_ms": atomic.LoadUint64(&s.lastCompactionDurationMs),
		"total_disk_usage_b":          totalDiskUsage,
		"bloom_false_positive_rate":   fpRate,
		"bloom_false_positives_total": fp,
		"bloom_true_positives_total":  tp,
		"approx_total_keys":           s.mem.Len() + int(totalDiskUsage/100), // very rough estimate
	}
}

// Close gracefully shuts down the compactor, flushes the final MemTable,
// and closes all open file handles.
func (s *Store) Close() error {
	var err error
	s.closeOnce.Do(func() {
		s.compactor.Stop()

		s.mu.Lock()
		defer s.mu.Unlock()

		if s.mem.Len() > 0 {
			s.flushMemTable()
		}

		if walErr := s.wal.Close(); walErr != nil {
			err = walErr
		}
		s.closeAllSSTables()
	})
	return err
}

// ── Internal Helpers (callers must hold s.mu.Lock) ───────────────────────────

// flushMemTable writes the current MemTable to a new SSTable, switches the
// store to a new empty MemTable, and resets the WAL.
func (s *Store) flushMemTable() {
	if s.mem.Len() == 0 {
		return
	}

	// Timestamp-based file naming for SSTables: easy newest-first sorting.
	sstName := fmt.Sprintf("%020d.sst", time.Now().UnixNano())
	sstPath := filepath.Join(s.dir, sstName)

	w, err := NewSSTableWriter(sstPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "lsm: failed to create SSTable %s: %v\n", sstPath, err)
		return
	}

	entries := s.mem.Entries()
	bf := NewBloomFilter(len(entries), 0.01)

	for _, e := range entries {
		_ = w.WriteEntry(e)
		bf.Add(e.Key)
	}
	w.Flush()
	_ = WriteBloomFile(sstPath, bf)

	// Open the new SSTable for reading.
	r, err := OpenSSTable(sstPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "lsm: failed to open flushed SSTable %s: %v\n", sstPath, err)
		return
	}

	// Make it the newest SSTable.
	s.sstables = append([]*SSTableReader{r}, s.sstables...)
	s.compactor.Add(sstPath)

	// Reset MemTable and WAL.
	s.mem = NewMemTable()
	if err := s.wal.Reset(); err != nil {
		fmt.Fprintf(os.Stderr, "lsm: failed to reset WAL: %v\n", err)
	}
}

// loadSSTables reads the directory for .sst files, opens them, and populates
// s.sstables in descending name order (newest first).
func (s *Store) loadSSTables() error {
	entries, err := os.ReadDir(s.dir)
	if err != nil {
		return fmt.Errorf("lsm: read dir: %w", err)
	}

	var paths []string
	for _, e := range entries {
		if !e.IsDir() && strings.HasSuffix(e.Name(), ".sst") {
			paths = append(paths, filepath.Join(s.dir, e.Name()))
		}
	}

	// Sort descending so the newest timestamp is first.
	sort.Sort(sort.Reverse(sort.StringSlice(paths)))

	for _, p := range paths {
		r, err := OpenSSTable(p)
		if err != nil {
			return fmt.Errorf("lsm: open %s: %w", p, err)
		}
		s.sstables = append(s.sstables, r)
	}
	return nil
}

// replaceSSTables is the callback from the Compactor. It replaces the input
// SSTable readers with a single output reader.
func (s *Store) replaceSSTables(inputs []string, output string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Open the new compacted SSTable.
	outReader, err := OpenSSTable(output)
	if err != nil {
		fmt.Fprintf(os.Stderr, "lsm: cannot open compacted output %s: %v\n", output, err)
		return
	}

	// Create a new slice.
	var newTables []*SSTableReader

	// Add the output table first (it contains the newest data among the compacted set).
	// Strictly speaking, we should put it exactly where the newest input was.
	// Since Compactor merges the N *oldest* or contiguous tables, we find the
	// index of the newest input.
	inputPaths := make(map[string]bool)
	for _, p := range inputs {
		inputPaths[p] = true
	}

	inserted := false
	var toClose []*SSTableReader

	for _, t := range s.sstables {
		if inputPaths[t.Path()] {
			toClose = append(toClose, t)
			if !inserted {
				newTables = append(newTables, outReader)
				inserted = true
			}
		} else {
			newTables = append(newTables, t)
		}
	}

	if !inserted {
		newTables = append(newTables, outReader)
	}

	s.sstables = newTables

	// Close old readers outside the lock just in case.
	go func(readers []*SSTableReader) {
		// Wait a tiny bit for in-flight requests that grabbed the reader pointer
		// to finish their local scan (simple hazard-pointer approximation).
		time.Sleep(100 * time.Millisecond)
		for _, r := range readers {
			_ = r.Close()
		}
	}(toClose)
}

func (s *Store) closeAllSSTables() {
	for _, r := range s.sstables {
		_ = r.Close()
	}
	s.sstables = nil
}
