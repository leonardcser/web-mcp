package cache

import "time"

// KV defines the minimal key-value cache contract with TTL semantics.
// Implementations must be safe for concurrent use by multiple goroutines.
type KV interface {
	Get(key string) ([]byte, error)
	Put(key string, value []byte, ttl time.Duration) error
	Delete(key string) error
}
