package store

// Backend defines the interface for a key-value store backend.
// Implement this interface to plug in any storage engine (in-memory, bbolt, badger, etc.)
type Backend interface {
	Get(key string) (string, bool)
	Set(key, value string)
	Delete(key string) bool
	ListKeys() []string
	Count() int
}
