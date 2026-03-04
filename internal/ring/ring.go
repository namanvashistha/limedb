package ring

import (
	"fmt"
	"math"
	"sort"
	"sync"

	"github.com/cespare/xxhash"
)

// ConsistentHashRing manages consistent hashing for distributed key-value storage.
type ConsistentHashRing struct {
	mu                  sync.RWMutex
	ring                map[uint64]string // hash -> nodeUrl
	sortedHashes        []uint64          // sorted keys of the ring for binary search
	virtualNodesPerNode int
	nodes               map[string]bool // Set of physical nodes
}

// New creates a new hash ring.
func New(virtualNodesPerNode int) *ConsistentHashRing {
	return &ConsistentHashRing{
		ring:                make(map[uint64]string),
		virtualNodesPerNode: virtualNodesPerNode,
		nodes:               make(map[string]bool),
	}
}

// AddNode adds a physical node to the ring with virtual nodes.
func (r *ConsistentHashRing) AddNode(nodeUrl string) {
	r.mu.Lock()
	defer r.mu.Unlock()

	if r.nodes[nodeUrl] {
		return
	}

	r.nodes[nodeUrl] = true

	// Always add at least 1 slot so the node owns some hash space.
	// VIRTUAL_NODES=0 means "1 real slot per node, no virtual replicas".
	slots := r.virtualNodesPerNode
	if slots == 0 {
		slots = 1
	}
	for i := 0; i < slots; i++ {
		virtualNodeKey := fmt.Sprintf("%s:%d", nodeUrl, i)
		h := r.hash(virtualNodeKey)
		r.ring[h] = nodeUrl
		r.sortedHashes = append(r.sortedHashes, h)
	}

	sort.Slice(r.sortedHashes, func(i, j int) bool {
		return r.sortedHashes[i] < r.sortedHashes[j]
	})
}

// RemoveNode removes a physical node from the ring.
func (r *ConsistentHashRing) RemoveNode(nodeUrl string) {
	r.mu.Lock()
	defer r.mu.Unlock()

	if !r.nodes[nodeUrl] {
		return
	}

	delete(r.nodes, nodeUrl)

	// Rebuild ring and sortedHashes to remove all virtual nodes
	newRing := make(map[uint64]string)
	var newSortedHashes []uint64

	for h, url := range r.ring {
		if url != nodeUrl {
			newRing[h] = url
			newSortedHashes = append(newSortedHashes, h)
		}
	}

	r.ring = newRing
	r.sortedHashes = newSortedHashes
	sort.Slice(r.sortedHashes, func(i, j int) bool {
		return r.sortedHashes[i] < r.sortedHashes[j]
	})
}

// GetNode returns the node responsible for the given key.
func (r *ConsistentHashRing) GetNode(key string) string {
	r.mu.RLock()
	defer r.mu.RUnlock()

	if len(r.ring) == 0 {
		return ""
	}

	h := r.hash(key)
	fmt.Println(h)

	// Binary search for the first node with hash >= h
	idx := sort.Search(len(r.sortedHashes), func(i int) bool {
		return r.sortedHashes[i] >= h
	})

	if idx == len(r.sortedHashes) {
		// Wrap around
		idx = 0
	}

	return r.ring[r.sortedHashes[idx]]
}

// GetNodes returns all physical nodes in the ring.
func (r *ConsistentHashRing) GetNodes() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()

	nodes := make([]string, 0, len(r.nodes))
	for node := range r.nodes {
		nodes = append(nodes, node)
	}
	return nodes
}

// GetRingStats returns statistics about the ring.
func (r *ConsistentHashRing) GetRingStats() map[string]interface{} {
	r.mu.RLock()
	defer r.mu.RUnlock()

	stats := make(map[string]interface{})
	stats["totalNodes"] = len(r.nodes)
	stats["virtualNodes"] = len(r.ring)
	stats["virtualNodesPerNode"] = r.virtualNodesPerNode

	distribution := make(map[string]int)
	for _, node := range r.ring {
		distribution[node]++
	}
	stats["virtualNodeDistribution"] = distribution

	return stats
}

// GetNodeRanges returns hash ranges for each node.
func (r *ConsistentHashRing) GetNodeRanges() map[string][]map[string]interface{} {
	r.mu.RLock()
	defer r.mu.RUnlock()

	nodeRanges := make(map[string][]map[string]interface{})
	if len(r.ring) == 0 {
		return nodeRanges
	}

	for node := range r.nodes {
		nodeRanges[node] = []map[string]interface{}{}
	}

	// sortedHashes is already sorted
	for i, currentHash := range r.sortedHashes {
		currentNode := r.ring[currentHash]

		var rangeStart uint64
		if i == 0 {
			// First node wraps around from the last node
			lastHash := r.sortedHashes[len(r.sortedHashes)-1]
			rangeStart = lastHash + 1
		} else {
			rangeStart = r.sortedHashes[i-1] + 1
		}

		rangeEnd := currentHash

		// Calculate size (handling wrap-around implicitly with int64 overflow/underflow logic? No, need explicit math)
		// Java: rangeEnd - rangeStart + 1
		// In Java, this can overflow/underflow but it represents the count.
		// Here we just store the values.

		rng := map[string]interface{}{
			"start": rangeStart,
			"end":   rangeEnd,
			"hash":  currentHash,
			"size":  rangeEnd - rangeStart + 1,
		}

		nodeRanges[currentNode] = append(nodeRanges[currentNode], rng)
	}

	return nodeRanges
}

// GetNodeRangesDegrees returns hash ranges converted to 360 degrees.
func (r *ConsistentHashRing) GetNodeRangesDegrees() map[string][]map[string]interface{} {
	r.mu.RLock()
	defer r.mu.RUnlock()

	nodeRanges := make(map[string][]map[string]interface{})
	if len(r.ring) == 0 {
		return nodeRanges
	}

	for node := range r.nodes {
		nodeRanges[node] = []map[string]interface{}{}
	}

	minHash := float64(math.MinInt64)
	maxHash := float64(math.MaxInt64)
	totalHashSpace := maxHash - minHash

	for i, currentHash := range r.sortedHashes {
		currentNode := r.ring[currentHash]

		var rangeStart uint64
		if i == 0 {
			lastHash := r.sortedHashes[len(r.sortedHashes)-1]
			rangeStart = lastHash + 1
		} else {
			rangeStart = r.sortedHashes[i-1] + 1
		}

		rangeEnd := currentHash

		startDegrees := ((float64(rangeStart) - minHash) / totalHashSpace) * 360.0
		endDegrees := ((float64(rangeEnd) - minHash) / totalHashSpace) * 360.0
		sizeDegrees := endDegrees - startDegrees

		if i == 0 && rangeStart > rangeEnd {
			// Wrap around
			sizeDegrees = (360.0 - startDegrees) + endDegrees
		}

		rng := map[string]interface{}{
			"startHash":    rangeStart,
			"endHash":      rangeEnd,
			"hash":         currentHash,
			"startDegrees": math.Round(startDegrees*100) / 100,
			"endDegrees":   math.Round(endDegrees*100) / 100,
			"sizeDegrees":  math.Round(sizeDegrees*100) / 100,
			"sizeHash":     rangeEnd - rangeStart + 1,
		}

		nodeRanges[currentNode] = append(nodeRanges[currentNode], rng)
	}

	return nodeRanges
}

func (r *ConsistentHashRing) hash(key string) uint64 {
	return xxhash.Sum64String(key)
}
