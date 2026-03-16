package lsm

// BloomFilter is a space-efficient probabilistic set membership test.
//
// It can answer "definitely not in set" or "maybe in set" — never a false
// negative, but with a configurable false-positive rate.
//
// ── Algorithm ────────────────────────────────────────────────────────────────
//
// We use the double-hashing (Kirsch-Mitzenmacher) trick to derive k independent
// hash positions from two base hashes, avoiding k separate hash function calls:
//
//   h_i(key) = (h1(key) + i * h2(key)) % m        for i in [0, k)
//
// h1 is FNV-1a-64, h2 is a simple multiply-xor mix of the same key.
// This gives excellent distribution with no external dependencies.
//
// ── Sizing guide ─────────────────────────────────────────────────────────────
//
//   NewBloomFilter(n, fpRate) computes optimal m and k for you:
//
//     m = -n * ln(fpRate) / ln(2)²   (number of bits)
//     k = (m/n) * ln(2)              (number of hash functions)
//
// Typical settings:
//   n=100_000, fpRate=0.01 → m≈958505 bits (~117 KB), k=7
//   n=100_000, fpRate=0.001 → m≈1437758 bits (~175 KB), k=10
//
// ── Persistence ──────────────────────────────────────────────────────────────
//
// BloomFilter.Bytes() serialises the bit array to []byte.
// BloomFilterFromBytes() reconstructs it, storing k alongside.
// SSTables persist the filter in a ".bloom" sidecar file next to ".sst".
//
// ─────────────────────────────────────────────────────────────────────────────

import (
	"encoding/binary"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"strings"
)

// BloomFilter is a fixed-size Bloom filter.
type BloomFilter struct {
	bits []byte // bit array, length = ceil(m/8)
	m    uint64 // number of bits
	k    uint32 // number of hash functions
}

// NewBloomFilter creates a new, empty Bloom filter sized for n expected items
// and a target false-positive rate fpRate (e.g. 0.01 = 1%).
//
// Both n and fpRate must be positive; fpRate must be < 1.
func NewBloomFilter(n int, fpRate float64) *BloomFilter {
	if n <= 0 {
		n = 1
	}
	if fpRate <= 0 || fpRate >= 1 {
		fpRate = 0.01
	}
	// Optimal number of bits.
	m := uint64(math.Ceil(-float64(n) * math.Log(fpRate) / (math.Log(2) * math.Log(2))))
	if m < 8 {
		m = 8
	}
	// Optimal number of hash functions.
	k := uint32(math.Round(float64(m) / float64(n) * math.Log(2)))
	if k < 1 {
		k = 1
	}

	return &BloomFilter{
		bits: make([]byte, (m+7)/8),
		m:    m,
		k:    k,
	}
}

// Add inserts key into the filter.
func (f *BloomFilter) Add(key string) {
	h1, h2 := hashPair(key)
	for i := uint32(0); i < f.k; i++ {
		pos := (h1 + uint64(i)*h2) % f.m
		f.bits[pos/8] |= 1 << (pos % 8)
	}
}

// MayContain returns false if key is definitely not in the set,
// or true if it might be (with a small false-positive probability).
func (f *BloomFilter) MayContain(key string) bool {
	h1, h2 := hashPair(key)
	for i := uint32(0); i < f.k; i++ {
		pos := (h1 + uint64(i)*h2) % f.m
		if f.bits[pos/8]&(1<<(pos%8)) == 0 {
			return false // definitely absent
		}
	}
	return true
}

// FalsePositiveRate returns the theoretical FP rate given the current number
// of set bits.  Useful in tests and diagnostics.
func (f *BloomFilter) FalsePositiveRate() float64 {
	// Count set bits.
	var set uint64
	for _, b := range f.bits {
		for b != 0 {
			set++
			b &= b - 1
		}
	}
	// P(fp) ≈ (1 - e^(-k*n/m))^k ; approximate via fill ratio.
	fillRatio := float64(set) / float64(f.m)
	return math.Pow(fillRatio, float64(f.k))
}

// ── Serialisation ─────────────────────────────────────────────────────────────

// headerSize is the fixed binary header: [8B m][4B k].
const bloomHeaderSize = 12

// Bytes serialises the filter to a compact byte slice:
//
//	[8B m (uint64 LE)][4B k (uint32 LE)][bit array bytes]
func (f *BloomFilter) Bytes() []byte {
	b := make([]byte, bloomHeaderSize+len(f.bits))
	binary.LittleEndian.PutUint64(b[0:8], f.m)
	binary.LittleEndian.PutUint32(b[8:12], f.k)
	copy(b[12:], f.bits)
	return b
}

// BloomFilterFromBytes reconstructs a BloomFilter from the output of Bytes().
func BloomFilterFromBytes(data []byte) (*BloomFilter, error) {
	if len(data) < bloomHeaderSize {
		return nil, fmt.Errorf("bloom: data too short (%d bytes)", len(data))
	}
	m := binary.LittleEndian.Uint64(data[0:8])
	k := binary.LittleEndian.Uint32(data[8:12])
	if m == 0 || k == 0 {
		return nil, fmt.Errorf("bloom: invalid m=%d k=%d", m, k)
	}
	expectedBitBytes := (m + 7) / 8
	if uint64(len(data)-bloomHeaderSize) < expectedBitBytes {
		return nil, fmt.Errorf("bloom: bit array too short (want %d, got %d)",
			expectedBitBytes, len(data)-bloomHeaderSize)
	}
	bits := make([]byte, expectedBitBytes)
	copy(bits, data[bloomHeaderSize:])
	return &BloomFilter{bits: bits, m: m, k: k}, nil
}

// ── Sidecar file helpers ──────────────────────────────────────────────────────

// bloomPath returns the path of the sidecar Bloom filter file for an SSTable.
// Convention: replace ".sst" suffix with ".bloom"; if no ".sst" suffix, append.
func bloomPath(sstPath string) string {
	if strings.HasSuffix(sstPath, ".sst") {
		return strings.TrimSuffix(sstPath, ".sst") + ".bloom"
	}
	return sstPath + ".bloom"
}

// WriteBloomFile serialises filter and writes it to the sidecar path next to
// the SSTable at sstPath.
func WriteBloomFile(sstPath string, filter *BloomFilter) error {
	p := bloomPath(sstPath)
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		return fmt.Errorf("bloom: mkdir: %w", err)
	}
	return os.WriteFile(p, filter.Bytes(), 0o644)
}

// ReadBloomFile loads the sidecar Bloom filter for the SSTable at sstPath.
// Returns (nil, nil) if the sidecar file does not exist — callers treat that
// as "no filter available, must read the SSTable".
func ReadBloomFile(sstPath string) (*BloomFilter, error) {
	data, err := os.ReadFile(bloomPath(sstPath))
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("bloom: read: %w", err)
	}
	return BloomFilterFromBytes(data)
}

// ── Hash functions ────────────────────────────────────────────────────────────

// hashPair returns two independent 64-bit hashes of key using:
//
//	h1 = FNV-1a-64
//	h2 = a multiplicative mix (distinct enough for double-hashing)
func hashPair(key string) (h1, h2 uint64) {
	// FNV-1a 64-bit.
	const fnvPrime = 1099511628211
	const fnvOffset = 14695981039346656037
	h1 = fnvOffset
	for i := 0; i < len(key); i++ {
		h1 ^= uint64(key[i])
		h1 *= fnvPrime
	}

	// Simple polynomial mix for h2 (must be odd to stay coprime with m).
	h2 = 0
	for i := 0; i < len(key); i++ {
		h2 = h2*31 + uint64(key[i])
	}
	h2 = h2*0x9e3779b97f4a7c15 + 1 // ensure non-zero, odd
	return h1, h2
}
