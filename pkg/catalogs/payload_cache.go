package catalogs

import (
	"bytes"
	"sync"
	"sync/atomic"
)

// catalogPayloadCache retains one validated payload for one immutable catalog.
// Returned bytes belong to the caller. Mutable readers never use this cache.
type catalogPayloadCache struct {
	mu   sync.Mutex
	data atomic.Pointer[[]byte]
}

func (c *catalogPayloadCache) encode(catalog *Catalog) ([]byte, error) {
	if data := c.data.Load(); data != nil {
		return bytes.Clone(*data), nil
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	if data := c.data.Load(); data != nil {
		return bytes.Clone(*data), nil
	}
	data, err := encodeCatalogPayload(catalog)
	if err != nil {
		return nil, err
	}
	c.data.Store(&data)
	return bytes.Clone(data), nil
}
