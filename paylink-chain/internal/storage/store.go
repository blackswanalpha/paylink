package storage

// KVStore is the interface for persistent key-value storage.
type KVStore interface {
	Get(key []byte) ([]byte, error)
	Set(key, value []byte) error
	Delete(key []byte) error
	Has(key []byte) (bool, error)

	// Batch operations
	NewBatch() Batch

	// Iteration
	Iterate(prefix []byte, fn func(key, value []byte) bool) error

	Close() error
}

// Batch represents an atomic batch of writes.
type Batch interface {
	Set(key, value []byte)
	Delete(key []byte)
	Flush() error
}
