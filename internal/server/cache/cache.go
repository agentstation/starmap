// Package cache provides an in-memory caching layer for the HTTP server.
// It uses patrickmn/go-cache for TTL-based caching with LRU-like eviction.
package cache

import (
	"time"

	gocache "github.com/patrickmn/go-cache"
)

// Cache wraps go-cache with additional features for HTTP caching.
type Cache struct {
	store *gocache.Cache
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
	return c.store.Get(key)
}

// Set stores a value in the cache with default TTL.
func (c *Cache) Set(key string, value any) {
	c.store.Set(key, value, gocache.DefaultExpiration)
}

// SetWithTTL stores a value in the cache with custom TTL.
func (c *Cache) SetWithTTL(key string, value any, ttl time.Duration) {
	c.store.Set(key, value, ttl)
}

// Delete removes a value from the cache.
func (c *Cache) Delete(key string) {
	c.store.Delete(key)
}

// Clear removes all items from the cache.
func (c *Cache) Clear() {
	c.store.Flush()
}

// ItemCount returns the number of items in the cache.
func (c *Cache) ItemCount() int {
	return c.store.ItemCount()
}

// Stats returns cache statistics.
type Stats struct {
	ItemCount int `json:"item_count"`
}

// GetStats returns current cache statistics.
func (c *Cache) GetStats() Stats {
	return Stats{
		ItemCount: c.store.ItemCount(),
	}
}
