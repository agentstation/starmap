package starmap

import (
	"reflect"
	"sync"

	"github.com/agentstation/starmap/pkg/catalogs"
)

// Compile-time interface check to ensure proper implementation.
var _ Hooks = (*hooks)(nil)

// Hooks provides event callback registration for catalog changes.
type Hooks interface {
	// OnModelAdded registers a callback for when models are added
	OnModelAdded(ModelAddedHook)

	// OnModelUpdated registers a callback for when models are updated
	OnModelUpdated(ModelUpdatedHook)

	// OnModelRemoved registers a callback for when models are removed
	OnModelRemoved(ModelRemovedHook)
}

// Hook function types for model events.
type (
	// ModelAddedHook is called when a model is added to the catalog.
	ModelAddedHook func(model catalogs.Model)

	// ModelUpdatedHook is called when a model is updated in the catalog.
	ModelUpdatedHook func(old, updated catalogs.Model)

	// ModelRemovedHook is called when a model is removed from the catalog.
	ModelRemovedHook func(model catalogs.Model)
)

// OnModelAdded registers a callback for when models are added.
func (c *client) OnModelAdded(fn ModelAddedHook) { c.hooks.OnModelAdded(fn) }

// OnModelUpdated registers a callback for when models are updated.
func (c *client) OnModelUpdated(fn ModelUpdatedHook) { c.hooks.OnModelUpdated(fn) }

// OnModelRemoved registers a callback for when models are removed.
func (c *client) OnModelRemoved(fn ModelRemovedHook) { c.hooks.OnModelRemoved(fn) }

// hooks manages event callbacks for catalog changes.
type hooks struct {
	mu           sync.RWMutex
	modelAdded   []ModelAddedHook
	modelUpdated []ModelUpdatedHook
	modelRemoved []ModelRemovedHook
}

// newHooks creates a new hooks instance.
func newHooks() *hooks { return &hooks{} }

// OnModelAdded is a hook that registers a callback for when models are added.
func (h *hooks) OnModelAdded(fn ModelAddedHook) { h.onModelAdded(fn) }

// OnModelUpdated is a hook that registers a callback for when models are updated.
func (h *hooks) OnModelUpdated(fn ModelUpdatedHook) { h.onModelUpdated(fn) }

// OnModelRemoved is a hook that registers a callback for when models are removed.
func (h *hooks) OnModelRemoved(fn ModelRemovedHook) { h.onModelRemoved(fn) }

// onModelAdded registers a callback for when models are added.
func (h *hooks) onModelAdded(fn ModelAddedHook) {
	h.mu.Lock()
	h.modelAdded = append(h.modelAdded, fn)
	h.mu.Unlock()
}

// onModelUpdated registers a callback for when models are updated.
func (h *hooks) onModelUpdated(fn ModelUpdatedHook) {
	h.mu.Lock()
	h.modelUpdated = append(h.modelUpdated, fn)
	h.mu.Unlock()
}

// onModelRemoved registers a callback for when models are removed.
func (h *hooks) onModelRemoved(fn ModelRemovedHook) {
	h.mu.Lock()
	h.modelRemoved = append(h.modelRemoved, fn)
	h.mu.Unlock()
}

// triggerUpdate compares old and new catalogs and triggers appropriate hooks.
func (h *hooks) triggerUpdate(old, updated catalogs.Reader) {
	h.mu.RLock()
	defer h.mu.RUnlock()

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
				for _, hook := range h.modelUpdated {
					hook(oldModel, newModel)
				}
			}
		} else {
			// Model was added
			for _, hook := range h.modelAdded {
				hook(newModel)
			}
		}
	}

	// Check for removed models
	for _, oldModel := range oldModels {
		if _, exists := newModelMap[oldModel.ID]; !exists {
			for _, hook := range h.modelRemoved {
				hook(oldModel)
			}
		}
	}
}
