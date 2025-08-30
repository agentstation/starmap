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

// OnModelAdded is a hook that registers a callback for when models are added
func (s *starmap) OnModelAdded(fn ModelAddedHook) {
	s.hooks.onModelAdded(fn)
}

// OnModelUpdated is a hook that registers a callback for when models are updated
func (s *starmap) OnModelUpdated(fn ModelUpdatedHook) {
	s.hooks.onModelUpdated(fn)
}

// OnModelRemoved is a hook that registers a callback for when models are removed
func (s *starmap) OnModelRemoved(fn ModelRemovedHook) {
	s.hooks.onModelRemoved(fn)
}

// hooks manages event callbacks for catalog changes
type hooks struct {
	mu           sync.RWMutex
	modelAdded   []ModelAddedHook
	modelUpdated []ModelUpdatedHook
	modelRemoved []ModelRemovedHook
}

// newHooks creates a new hooks instance
func newHooks() *hooks {
	return &hooks{}
}

// onModelAdded registers a callback for when models are added
func (h *hooks) onModelAdded(fn ModelAddedHook) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.modelAdded = append(h.modelAdded, fn)
}

// onModelUpdated registers a callback for when models are updated
func (h *hooks) onModelUpdated(fn ModelUpdatedHook) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.modelUpdated = append(h.modelUpdated, fn)
}

// onModelRemoved registers a callback for when models are removed
func (h *hooks) onModelRemoved(fn ModelRemovedHook) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.modelRemoved = append(h.modelRemoved, fn)
}

// triggerCatalogUpdate compares old and new catalogs and triggers appropriate hooks
func (h *hooks) triggerCatalogUpdate(oldCatalog, newCatalog catalogs.Reader) {
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
				for _, hook := range h.modelUpdated {
					hook(oldModel, *newModel)
				}
			}
		} else {
			// Model was added
			for _, hook := range h.modelAdded {
				hook(*newModel)
			}
		}
	}

	// Check for removed models
	for _, oldModel := range oldModels.List() {
		if _, exists := newModelMap[oldModel.ID]; !exists {
			for _, hook := range h.modelRemoved {
				hook(*oldModel)
			}
		}
	}
}
