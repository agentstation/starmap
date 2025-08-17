package starmap

import (
	"reflect"
	"sync"

	"github.com/agentstation/starmap/pkg/catalogs"
)

// Hook function types for model events
type (
	// ModelAddedHook is called when a model is added to the catalog
	ModelAddedHook func(model catalogs.Model)

	// ModelUpdatedHook is called when a model is updated in the catalog
	ModelUpdatedHook func(old, new catalogs.Model)

	// ModelRemovedHook is called when a model is removed from the catalog
	ModelRemovedHook func(model catalogs.Model)
)

// hooks manages event callbacks for catalog changes
type hooks struct {
	mu             sync.RWMutex
	onModelAdded   []ModelAddedHook
	onModelUpdated []ModelUpdatedHook
	onModelRemoved []ModelRemovedHook
}

// newHooks creates a new hooks instance
func newHooks() *hooks {
	return &hooks{}
}

// OnModelAdded registers a callback for when models are added
func (h *hooks) OnModelAdded(fn ModelAddedHook) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.onModelAdded = append(h.onModelAdded, fn)
}

// OnModelUpdated registers a callback for when models are updated
func (h *hooks) OnModelUpdated(fn ModelUpdatedHook) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.onModelUpdated = append(h.onModelUpdated, fn)
}

// OnModelRemoved registers a callback for when models are removed
func (h *hooks) OnModelRemoved(fn ModelRemovedHook) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.onModelRemoved = append(h.onModelRemoved, fn)
}

// triggerCatalogUpdate compares old and new catalogs and triggers appropriate hooks
func (h *hooks) triggerCatalogUpdate(oldCatalog, newCatalog catalogs.Catalog) {
	h.mu.RLock()
	defer h.mu.RUnlock()

	// Get old and new models for comparison
	oldModels := oldCatalog.Models()
	newModels := newCatalog.Models()

	// Create maps for efficient lookup
	oldModelMap := make(map[string]catalogs.Model)
	for _, model := range oldModels.List() {
		oldModelMap[model.ID] = *model
	}

	newModelMap := make(map[string]catalogs.Model)
	for _, model := range newModels.List() {
		newModelMap[model.ID] = *model
	}

	// Detect changes and trigger hooks
	for _, newModel := range newModels.List() {
		if oldModel, exists := oldModelMap[newModel.ID]; exists {
			// Check if model was updated
			if !reflect.DeepEqual(oldModel, *newModel) {
				for _, hook := range h.onModelUpdated {
					hook(oldModel, *newModel)
				}
			}
		} else {
			// Model was added
			for _, hook := range h.onModelAdded {
				hook(*newModel)
			}
		}
	}

	// Check for removed models
	for _, oldModel := range oldModels.List() {
		if _, exists := newModelMap[oldModel.ID]; !exists {
			for _, hook := range h.onModelRemoved {
				hook(*oldModel)
			}
		}
	}
}
