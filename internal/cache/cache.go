package cache

import (
	"encoding/binary"
	"errors"
	"sync"
	"time"

	bolt "go.etcd.io/bbolt"
)

// Store provides a simple persistent KV cache with TTL semantics.
// It is safe for concurrent use by multiple goroutines.
type Store struct {
	db         *bolt.DB
	bucket     []byte
	defaultTTL time.Duration
	mu         sync.RWMutex
}

type Options struct {
	// Bucket is the name of the Bolt bucket to use.
	Bucket string
	// DefaultTTL is used when Put is called with ttl <= 0.
	DefaultTTL time.Duration
}

var (
	ErrNotFound = errors.New("cache: not found")
	ErrExpired  = errors.New("cache: expired")
)

// Open initializes or opens a Store at the given path.
func Open(path string, opts Options) (*Store, error) {
	db, err := bolt.Open(path, 0o600, &bolt.Options{Timeout: 1 * time.Second})
	if err != nil {
		return nil, err
	}
	bucket := []byte("cache")
	if opts.Bucket != "" {
		bucket = []byte(opts.Bucket)
	}
	if err := db.Update(func(tx *bolt.Tx) error {
		_, err := tx.CreateBucketIfNotExists(bucket)
		return err
	}); err != nil {
		_ = db.Close()
		return nil, err
	}
	return &Store{db: db, bucket: bucket, defaultTTL: opts.DefaultTTL}, nil
}

// Close closes the underlying database.
func (s *Store) Close() error {
	if s == nil || s.db == nil {
		return nil
	}
	return s.db.Close()
}

// Put stores value with an absolute expiration computed as now+ttl.
// If ttl <= 0, DefaultTTL is used; if DefaultTTL <= 0, the item never expires.
func (s *Store) Put(key string, value []byte, ttl time.Duration) error {
	expiresAt := int64(0)
	if ttl <= 0 {
		ttl = s.defaultTTL
	}
	if ttl > 0 {
		expiresAt = time.Now().Add(ttl).Unix()
	}
	// Layout: 8 bytes big endian expiresAt || raw value
	buf := make([]byte, 8+len(value))
	binary.BigEndian.PutUint64(buf[:8], uint64(expiresAt))
	copy(buf[8:], value)

	s.mu.Lock()
	defer s.mu.Unlock()
	return s.db.Update(func(tx *bolt.Tx) error {
		b := tx.Bucket(s.bucket)
		return b.Put([]byte(key), buf)
	})
}

// Get returns cached value if present and not expired.
func (s *Store) Get(key string) ([]byte, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	var out []byte
	var expired bool
	var exists bool
	if err := s.db.View(func(tx *bolt.Tx) error {
		b := tx.Bucket(s.bucket)
		v := b.Get([]byte(key))
		if v == nil {
			return nil
		}
		exists = true
		expiresAt := int64(binary.BigEndian.Uint64(v[:8]))
		if expiresAt > 0 && time.Now().Unix() > expiresAt {
			expired = true
			return nil
		}
		out = append([]byte(nil), v[8:]...)
		return nil
	}); err != nil {
		return nil, err
	}
	if !exists {
		return nil, ErrNotFound
	}
	if expired {
		return nil, ErrExpired
	}
	return out, nil
}

// Delete removes a key.
func (s *Store) Delete(key string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.db.Update(func(tx *bolt.Tx) error {
		return tx.Bucket(s.bucket).Delete([]byte(key))
	})
}
