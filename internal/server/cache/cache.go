// Package cache provides an in-memory caching layer for the HTTP server.
// It uses patrickmn/go-cache for TTL-based caching with LRU-like eviction.
package cache

import (
	"sync"
	"time"

	gocache "github.com/patrickmn/go-cache"
)

// Cache wraps go-cache with additional features for HTTP caching.
type Cache struct {
	mu           sync.RWMutex
	store        *gocache.Cache
	generationID string
	sequence     uint64
}

// New creates a new cache with the given TTL and cleanup interval.
// defaultTTL is the default expiration time for cache entries.
// cleanupInterval is how often expired items are removed from memory.
func New(defaultTTL, cleanupInterval time.Duration) *Cache {
	return &Cache{
		store: gocache.New(defaultTTL, cleanupInterval),
	}
}

// Get retrieves a value from the cache.
func (c *Cache) Get(key string) (any, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.store.Get(key)
}

// Set stores a value in the cache with default TTL.
func (c *Cache) Set(key string, value any) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	c.store.Set(key, value, gocache.DefaultExpiration)
}

// SetWithTTL stores a value in the cache with custom TTL.
func (c *Cache) SetWithTTL(key string, value any, ttl time.Duration) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	c.store.Set(key, value, ttl)
}

// Delete removes a value from the cache.
func (c *Cache) Delete(key string) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	c.store.Delete(key)
}

// Clear removes all items from the cache.
func (c *Cache) Clear() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.store.Flush()
}

// ActivateGeneration atomically changes the visible namespace and removes old
// entries. Repeating the same generation is a no-op.
func (c *Cache) ActivateGeneration(sequence uint64, generationID string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.activateLocked(sequence, generationID)
}

// GetGeneration retrieves a key only from generationID. The first request may
// initialize an empty cache, but an in-flight old request cannot reactivate a
// superseded namespace.
func (c *Cache) GetGeneration(sequence uint64, generationID, key string) (any, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.activateLocked(sequence, generationID)
	if c.sequence != sequence || c.generationID != generationID {
		return nil, false
	}
	return c.store.Get(key)
}

// SetGeneration stores a key in the active generation namespace.
func (c *Cache) SetGeneration(sequence uint64, generationID, key string, value any) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.activateLocked(sequence, generationID)
	if c.sequence != sequence || c.generationID != generationID {
		return
	}
	c.store.Set(key, value, gocache.DefaultExpiration)
}

// GenerationID returns the active cache namespace.
func (c *Cache) GenerationID() string {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.generationID
}

func (c *Cache) activateLocked(sequence uint64, generationID string) {
	if sequence <= c.sequence {
		return
	}
	c.sequence = sequence
	c.generationID = generationID
	c.store.Flush()
}

// ItemCount returns the number of items in the cache.
func (c *Cache) ItemCount() int {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.store.ItemCount()
}

// Stats returns cache statistics.
type Stats struct {
	ItemCount    int    `json:"item_count"`
	GenerationID string `json:"generation_id,omitempty"`
	Sequence     uint64 `json:"sequence"`
}

// GetStats returns current cache statistics.
func (c *Cache) GetStats() Stats {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return Stats{
		ItemCount: c.store.ItemCount(), GenerationID: c.generationID, Sequence: c.sequence,
	}
}
