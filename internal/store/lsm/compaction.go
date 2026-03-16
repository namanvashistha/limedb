package lsm

// Compaction — k-way merge of SSTables
//
// Why compaction?
//   Writing always goes to the MemTable (fast RAM write).  When the MemTable
//   is full it is flushed as a new immutable SSTable.  Over time this creates
//   many small SSTable files.  Without compaction:
//     • Reads degrade:  we must search each SSTable in turn.
//     • Disk grows:     stale values and tombstones are never reclaimed.
//
// Strategy — Size-Tiered Compaction (STCS):
//   When the number of SSTables >= CompactionThreshold (default 4) the
//   Compactor merges ALL of them into one new SSTable by k-way merge.
//   This is the same strategy Cassandra uses at L0.
//
// Merge rules (newest-wins):
//   • Inputs are passed newest → oldest.  The first occurrence of a key wins.
//   • Tombstones are preserved unless DropTombstones=true, which is only safe
//     when ALL SSTables in the engine are being compacted in one pass (nothing
//     older can be "underneath" the tombstone).
//
// Concurrency:
//   MergeSSTables is a pure function — no shared state.
//   Compactor wraps it in a background goroutine with a trigger channel.

import (
	"container/heap"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// ── k-way merge ───────────────────────────────────────────────────────────────

// MergeOptions controls how MergeSSTables behaves.
type MergeOptions struct {
	// DropTombstones removes deleted entries from the output instead of
	// propagating them. Only set this to true when you are merging every
	// SSTable in the engine (full compaction) so there is no older data
	// that the tombstone needs to mask.
	DropTombstones bool

	// ExpectedKeys is used to size the Bloom filter for the output SSTable.
	// If 0 it defaults to the total input entry count.
	ExpectedKeys int

	// FPRate is the Bloom filter false-positive rate. Defaults to 0.01.
	FPRate float64
}

// MergeSSTables performs a k-way merge of the given SSTable files (newest
// → oldest) and writes the result to outputPath.  It also generates a Bloom
// filter sidecar at the corresponding .bloom path.
//
// The input SSTable files are NOT deleted by this function — the caller
// decides when it is safe to remove them (after the new SSTable is durable).
func MergeSSTables(inputs []string, outputPath string, opts MergeOptions) error {
	if len(inputs) == 0 {
		return fmt.Errorf("compaction: no input SSTables")
	}

	// Open all input readers.
	readers := make([]*SSTableReader, 0, len(inputs))
	defer func() {
		for _, r := range readers {
			_ = r.Close()
		}
	}()
	for _, p := range inputs {
		r, err := OpenSSTable(p)
		if err != nil {
			return fmt.Errorf("compaction: open %s: %w", p, err)
		}
		readers = append(readers, r)
	}

	// Load all entries into per-SSTable slices (index = priority; 0 = newest).
	// For huge tables a streaming iterator would be better, but this keeps the
	// implementation self-contained and easy to reason about.
	allEntries := make([][]Entry, len(readers))
	for i, r := range readers {
		entries, err := r.Entries()
		if err != nil {
			return fmt.Errorf("compaction: read entries from %s: %w", inputs[i], err)
		}
		allEntries[i] = entries
	}

	// k-way merge via a min-heap.
	merged := kWayMerge(allEntries)

	// Filter tombstones if requested.
	if opts.DropTombstones {
		filtered := merged[:0]
		for _, e := range merged {
			if !e.Deleted {
				filtered = append(filtered, e)
			}
		}
		merged = filtered
	}

	// Size bloom filter.
	n := opts.ExpectedKeys
	if n == 0 {
		n = len(merged)
	}
	if n == 0 {
		n = 1
	}
	fpRate := opts.FPRate
	if fpRate == 0 {
		fpRate = 0.01
	}

	// Write output SSTable + Bloom filter.
	w, err := NewSSTableWriter(outputPath)
	if err != nil {
		return fmt.Errorf("compaction: create output: %w", err)
	}
	bf := NewBloomFilter(n, fpRate)
	for _, e := range merged {
		if err := w.WriteEntry(e); err != nil {
			return fmt.Errorf("compaction: write entry: %w", err)
		}
		bf.Add(e.Key)
	}
	if err := w.Flush(); err != nil {
		return fmt.Errorf("compaction: flush: %w", err)
	}
	if err := WriteBloomFile(outputPath, bf); err != nil {
		return fmt.Errorf("compaction: write bloom: %w", err)
	}
	return nil
}

// ── k-way merge heap ──────────────────────────────────────────────────────────

// mergeItem is one element in the k-way merge heap.
type mergeItem struct {
	entry    Entry
	priority int // lower = newer SSTable (wins on key collision)
	idx      int // current position within its SSTable slice
	table    int // which input SSTable this item came from
}

type mergeHeap []mergeItem

func (h mergeHeap) Len() int      { return len(h) }
func (h mergeHeap) Swap(i, j int) { h[i], h[j] = h[j], h[i] }
func (h mergeHeap) Less(i, j int) bool {
	if h[i].entry.Key != h[j].entry.Key {
		return h[i].entry.Key < h[j].entry.Key
	}
	// Same key: lower priority number (=newer) wins; it goes first so we emit
	// it and then skip all older duplicates.
	return h[i].priority < h[j].priority
}
func (h *mergeHeap) Push(x interface{}) { *h = append(*h, x.(mergeItem)) }
func (h *mergeHeap) Pop() interface{} {
	old := *h
	n := len(old)
	x := old[n-1]
	*h = old[:n-1]
	return x
}

// kWayMerge merges k sorted entry slices (allEntries[0] = newest) into a
// single sorted slice with newest-wins deduplication.
func kWayMerge(allEntries [][]Entry) []Entry {
	h := &mergeHeap{}
	heap.Init(h)

	// Seed the heap with the first entry from each SSTable.
	cursors := make([]int, len(allEntries))
	for t, entries := range allEntries {
		if len(entries) > 0 {
			heap.Push(h, mergeItem{
				entry:    entries[0],
				priority: t,
				idx:      0,
				table:    t,
			})
			cursors[t] = 1
		}
	}

	var result []Entry
	var lastKey string

	for h.Len() > 0 {
		item := heap.Pop(h).(mergeItem)

		// Advance the cursor for this SSTable.
		t := item.table
		if cursors[t] < len(allEntries[t]) {
			heap.Push(h, mergeItem{
				entry:    allEntries[t][cursors[t]],
				priority: t,
				idx:      cursors[t],
				table:    t,
			})
			cursors[t]++
		}

		// Skip stale duplicates: only emit the first occurrence of each key
		// (which is the newest, since lower priority = newer = wins in Less).
		if item.entry.Key == lastKey {
			continue
		}

		lastKey = item.entry.Key
		result = append(result, item.entry)
	}

	return result
}

// ── Background Compactor ──────────────────────────────────────────────────────

// CompactorConfig controls the Compactor's behaviour.
type CompactorConfig struct {
	// Dir is the directory where new SSTable files are created.
	Dir string

	// Threshold is the minimum number of SSTables that triggers a compaction.
	// Defaults to 4 if 0.
	Threshold int

	// PollInterval is how often the compactor checks the SSTable count when
	// nothing has been explicitly signalled. Defaults to 10s if 0.
	PollInterval time.Duration

	// OnCompactionStart is called before MergeSSTables begins to allow the engine to set state.
	OnCompactionStart func()

	// OnCompacted is called after a successful compaction with the list of
	// input paths that were merged, the single output path that replaced
	// them, and the duration of the compaction in milliseconds.
	OnCompacted func(inputs []string, output string, durationMs int64)

	// OnError is called when compaction fails. If nil, errors are silently
	// dropped (not ideal for production; hook in your logger here).
	OnError func(err error)
}

// Compactor runs size-tiered compaction in the background.
// Create one with NewCompactor, then call Start() / Stop().
type Compactor struct {
	cfg      CompactorConfig
	mu       sync.Mutex
	tables   []string // SSTable paths, newest first; managed by caller via Add
	seqMu    sync.Mutex
	seq      int // monotonically increasing output file name counter
	triggerC chan struct{}
	stopC    chan struct{}
	doneC    chan struct{}
}

// NewCompactor creates a Compactor with the given config.
func NewCompactor(cfg CompactorConfig) *Compactor {
	if cfg.Threshold <= 0 {
		cfg.Threshold = 4
	}
	if cfg.PollInterval <= 0 {
		cfg.PollInterval = 10 * time.Second
	}
	return &Compactor{
		cfg:      cfg,
		triggerC: make(chan struct{}, 1),
		stopC:    make(chan struct{}),
		doneC:    make(chan struct{}),
	}
}

// Add registers a newly-flushed SSTable with the compactor (newest first).
// It also pokes the background goroutine if the threshold is now met.
func (c *Compactor) Add(path string) {
	c.mu.Lock()
	c.tables = append([]string{path}, c.tables...) // prepend = newest first
	n := len(c.tables)
	c.mu.Unlock()

	if n >= c.cfg.Threshold {
		select {
		case c.triggerC <- struct{}{}:
		default: // already queued
		}
	}
}

// Tables returns a snapshot of the current SSTable list (newest first).
func (c *Compactor) Tables() []string {
	c.mu.Lock()
	defer c.mu.Unlock()
	out := make([]string, len(c.tables))
	copy(out, c.tables)
	return out
}

// Start launches the background compaction goroutine.
func (c *Compactor) Start() {
	go c.loop()
}

// Stop signals the background goroutine to exit and waits for it.
func (c *Compactor) Stop() {
	close(c.stopC)
	<-c.doneC
}

// Trigger manually pokes the compactor to run immediately (useful in tests).
func (c *Compactor) Trigger() {
	select {
	case c.triggerC <- struct{}{}:
	default:
	}
}

// loop is the background goroutine body.
func (c *Compactor) loop() {
	defer close(c.doneC)
	ticker := time.NewTicker(c.cfg.PollInterval)
	defer ticker.Stop()

	for {
		select {
		case <-c.stopC:
			return
		case <-ticker.C:
			c.maybeCompact()
		case <-c.triggerC:
			c.maybeCompact()
		}
	}
}

// maybeCompact runs one compaction cycle if the threshold is met.
func (c *Compactor) maybeCompact() {
	c.mu.Lock()
	if len(c.tables) < c.cfg.Threshold {
		c.mu.Unlock()
		return
	}
	inputs := make([]string, len(c.tables))
	copy(inputs, c.tables)
	c.mu.Unlock()

	// Generate a unique output path.
	c.seqMu.Lock()
	c.seq++
	seq := c.seq
	c.seqMu.Unlock()

	outputPath := filepath.Join(c.cfg.Dir, fmt.Sprintf("compact-%06d.sst", seq))

	if c.cfg.OnCompactionStart != nil {
		c.cfg.OnCompactionStart()
	}
	start := time.Now()

	// isFullCompaction=true means no older SSTables exist → safe to drop tombstones.
	err := MergeSSTables(inputs, outputPath, MergeOptions{DropTombstones: true})
	if err != nil {
		if c.cfg.OnError != nil {
			c.cfg.OnError(fmt.Errorf("compaction: %w", err))
		}
		return
	}

	// Swap the table list atomically: replace all inputs with the single output.
	c.mu.Lock()
	c.tables = []string{outputPath}
	c.mu.Unlock()

	// Notify engine (it will update its own registry and delete old files).
	if c.cfg.OnCompacted != nil {
		c.cfg.OnCompacted(inputs, outputPath, time.Since(start).Milliseconds())
	}

	// Delete the input files now that the new SSTable is durable.
	for _, p := range inputs {
		_ = os.Remove(p)
		_ = os.Remove(bloomPath(p))
	}
}
