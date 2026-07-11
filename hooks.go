package starmap

import (
	"reflect"
	"sync"
	"sync/atomic"
	"time"

	"github.com/agentstation/starmap/pkg/catalogs"
)

// Hook function types for model events.
type (
	// CatalogPublishedEvent identifies one durably committed immutable catalog.
	// Catalog is safe to retain and share across goroutines.
	CatalogPublishedEvent struct {
		GenerationID string
		SyncRunID    string
		Sequence     uint64
		Catalog      *catalogs.Catalog
	}

	// CatalogPublishedHook is called after a catalog generation is durably
	// committed and atomically published.
	CatalogPublishedHook func(CatalogPublishedEvent) error

	// ModelAddedHook is called when a model is added to the catalog.
	ModelAddedHook func(model catalogs.Model)

	// ModelUpdatedHook is called when a model is updated in the catalog.
	ModelUpdatedHook func(old, updated catalogs.Model)

	// ModelRemovedHook is called when a model is removed from the catalog.
	ModelRemovedHook func(model catalogs.Model)
)

// HookDeliveryStats reports isolated callback delivery health.
type HookDeliveryStats struct {
	Completed   uint64
	Failures    uint64
	Panics      uint64
	Dropped     uint64
	LastLatency time.Duration
	MaxLatency  time.Duration
}

const defaultHookDeliveryConcurrency = 16

// OnCatalogPublished registers a callback for durable catalog publication.
func (c *Client) OnCatalogPublished(fn CatalogPublishedHook) { c.hooks.OnCatalogPublished(fn) }

// HookStats returns a lock-free snapshot of callback delivery health.
func (c *Client) HookStats() HookDeliveryStats { return c.hooks.statsSnapshot() }

// OnModelAdded registers a callback for when models are added.
func (c *Client) OnModelAdded(fn ModelAddedHook) { c.hooks.OnModelAdded(fn) }

// OnModelUpdated registers a callback for when models are updated.
func (c *Client) OnModelUpdated(fn ModelUpdatedHook) { c.hooks.OnModelUpdated(fn) }

// OnModelRemoved registers a callback for when models are removed.
func (c *Client) OnModelRemoved(fn ModelRemovedHook) { c.hooks.OnModelRemoved(fn) }

// hooks manages event callbacks for catalog changes.
type hooks struct {
	mu               sync.RWMutex
	modelAdded       []ModelAddedHook
	modelUpdated     []ModelUpdatedHook
	modelRemoved     []ModelRemovedHook
	catalogPublished []CatalogPublishedHook
	deliverySlots    chan struct{}
	completed        atomic.Uint64
	failures         atomic.Uint64
	panics           atomic.Uint64
	dropped          atomic.Uint64
	lastLatency      atomic.Int64
	maxLatency       atomic.Int64
}

// newHooks creates a new hooks instance.
func newHooks() *hooks {
	return &hooks{deliverySlots: make(chan struct{}, defaultHookDeliveryConcurrency)}
}

// OnModelAdded is a hook that registers a callback for when models are added.
func (h *hooks) OnModelAdded(fn ModelAddedHook) { h.onModelAdded(fn) }

// OnModelUpdated is a hook that registers a callback for when models are updated.
func (h *hooks) OnModelUpdated(fn ModelUpdatedHook) { h.onModelUpdated(fn) }

// OnModelRemoved is a hook that registers a callback for when models are removed.
func (h *hooks) OnModelRemoved(fn ModelRemovedHook) { h.onModelRemoved(fn) }

// OnCatalogPublished registers a durable publication callback.
func (h *hooks) OnCatalogPublished(fn CatalogPublishedHook) {
	if fn == nil {
		return
	}
	h.mu.Lock()
	h.catalogPublished = append(h.catalogPublished, fn)
	h.mu.Unlock()
}

func (h *hooks) dispatchUpdate(old, updated *catalogs.Catalog, event CatalogPublishedEvent) {
	select {
	case h.deliverySlots <- struct{}{}:
	default:
		h.dropped.Add(1)
		return
	}
	go func() {
		defer func() { <-h.deliverySlots }()
		h.mu.RLock()
		publicationHooks := append([]CatalogPublishedHook(nil), h.catalogPublished...)
		h.mu.RUnlock()
		var publicationGroup sync.WaitGroup
		publicationGroup.Add(len(publicationHooks))
		for _, hook := range publicationHooks {
			hook := hook
			go func() {
				defer publicationGroup.Done()
				h.invoke(func() error { return hook(event) })
			}()
		}
		// Model-diff callbacks retain publication ordering, but independent
		// publication observers cannot head-of-line block one another.
		publicationGroup.Wait()
		h.triggerUpdate(old, updated)
	}()
}

// onModelAdded registers a callback for when models are added.
func (h *hooks) onModelAdded(fn ModelAddedHook) {
	if fn == nil {
		return
	}
	h.mu.Lock()
	h.modelAdded = append(h.modelAdded, fn)
	h.mu.Unlock()
}

// onModelUpdated registers a callback for when models are updated.
func (h *hooks) onModelUpdated(fn ModelUpdatedHook) {
	if fn == nil {
		return
	}
	h.mu.Lock()
	h.modelUpdated = append(h.modelUpdated, fn)
	h.mu.Unlock()
}

// onModelRemoved registers a callback for when models are removed.
func (h *hooks) onModelRemoved(fn ModelRemovedHook) {
	if fn == nil {
		return
	}
	h.mu.Lock()
	h.modelRemoved = append(h.modelRemoved, fn)
	h.mu.Unlock()
}

// triggerUpdate compares old and new catalogs and triggers appropriate hooks.
func (h *hooks) triggerUpdate(old, updated catalogs.Reader) {
	h.mu.RLock()
	modelAdded := append([]ModelAddedHook(nil), h.modelAdded...)
	modelUpdated := append([]ModelUpdatedHook(nil), h.modelUpdated...)
	modelRemoved := append([]ModelRemovedHook(nil), h.modelRemoved...)
	h.mu.RUnlock()

	// Get old and new models for comparison
	oldModels := old.Models().List()
	newModels := updated.Models().List()

	// Create maps for efficient lookup
	oldModelMap := make(map[string]catalogs.Model)
	for _, model := range oldModels {
		oldModelMap[model.ID] = model
	}

	newModelMap := make(map[string]catalogs.Model)
	for _, model := range newModels {
		newModelMap[model.ID] = model
	}

	// Detect changes and trigger hooks
	for _, newModel := range newModels {
		if oldModel, exists := oldModelMap[newModel.ID]; exists {
			// Check if model was updated
			if !reflect.DeepEqual(oldModel, newModel) {
				for _, hook := range modelUpdated {
					h.invoke(func() error {
						hook(oldModel, newModel)
						return nil
					})
				}
			}
		} else {
			// Model was added
			for _, hook := range modelAdded {
				h.invoke(func() error {
					hook(newModel)
					return nil
				})
			}
		}
	}

	// Check for removed models
	for _, oldModel := range oldModels {
		if _, exists := newModelMap[oldModel.ID]; !exists {
			for _, hook := range modelRemoved {
				h.invoke(func() error {
					hook(oldModel)
					return nil
				})
			}
		}
	}
}

func (h *hooks) invoke(fn func() error) {
	started := time.Now()
	panicked := false
	failed := false
	func() {
		defer func() {
			if recover() != nil {
				panicked = true
				failed = true
			}
		}()
		if err := fn(); err != nil {
			failed = true
		}
	}()
	latency := time.Since(started)
	h.completed.Add(1)
	h.lastLatency.Store(int64(latency))
	updateMaxDuration(&h.maxLatency, latency)
	if failed {
		h.failures.Add(1)
	}
	if panicked {
		h.panics.Add(1)
	}
}

func updateMaxDuration(target *atomic.Int64, value time.Duration) {
	for {
		current := target.Load()
		if int64(value) <= current || target.CompareAndSwap(current, int64(value)) {
			return
		}
	}
}

func (h *hooks) statsSnapshot() HookDeliveryStats {
	return HookDeliveryStats{
		Completed:   h.completed.Load(),
		Failures:    h.failures.Load(),
		Panics:      h.panics.Load(),
		Dropped:     h.dropped.Load(),
		LastLatency: time.Duration(h.lastLatency.Load()),
		MaxLatency:  time.Duration(h.maxLatency.Load()),
	}
}
