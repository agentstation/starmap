package catalogs

import (
	"fmt"
	"maps"
	"sync"
)

// Models is a concurrent safe map of models.
type Models struct {
	mu     sync.RWMutex
	models map[string]*Model
}

// ModelsOption defines a function that configures a Models instance.
type ModelsOption func(*Models)

// WithModelsCapacity sets the initial capacity of the models map.
func WithModelsCapacity(capacity int) ModelsOption {
	return func(m *Models) {
		m.models = make(map[string]*Model, capacity)
	}
}

// WithModelsMap initializes the map with existing models.
func WithModelsMap(models map[string]*Model) ModelsOption {
	return func(m *Models) {
		if models != nil {
			m.models = make(map[string]*Model, len(models))
			maps.Copy(m.models, models)
		}
	}
}

// NewModels creates a new Models map with optional configuration.
func NewModels(opts ...ModelsOption) *Models {
	m := &Models{
		models: make(map[string]*Model),
	}

	for _, opt := range opts {
		opt(m)
	}

	return m
}

// Get returns a model by id and whether it exists.
func (m *Models) Get(id string) (*Model, bool) {
	m.mu.RLock()
	model, ok := m.models[id]
	m.mu.RUnlock()
	return model, ok
}

// Set sets a model by id. Returns an error if model is nil.
func (m *Models) Set(id string, model *Model) error {
	if model == nil {
		return fmt.Errorf("model cannot be nil")
	}

	m.mu.Lock()
	defer m.mu.Unlock()
	m.models[id] = model
	return nil
}

// Add adds a model, returning an error if it already exists.
func (m *Models) Add(model *Model) error {
	if model == nil {
		return fmt.Errorf("model cannot be nil")
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	if _, exists := m.models[model.ID]; exists {
		return fmt.Errorf("model with ID %s already exists", model.ID)
	}

	m.models[model.ID] = model
	return nil
}

// Delete removes a model by id. Returns an error if the model doesn't exist.
func (m *Models) Delete(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, exists := m.models[id]; !exists {
		return fmt.Errorf("model with ID %s not found", id)
	}

	delete(m.models, id)
	return nil
}

// Exists checks if a model exists without returning it.
func (m *Models) Exists(id string) bool {
	m.mu.RLock()
	_, exists := m.models[id]
	m.mu.RUnlock()
	return exists
}

// Len returns the number of models.
func (m *Models) Len() int {
	m.mu.RLock()
	length := len(m.models)
	m.mu.RUnlock()
	return length
}

// List returns a slice of all models.
func (m *Models) List() []*Model {
	m.mu.RLock()
	models := make([]*Model, len(m.models))
	i := 0
	for _, model := range m.models {
		models[i] = model
		i++
	}
	m.mu.RUnlock()
	return models
}

// Map returns a copy of all models.
func (m *Models) Map() map[string]*Model {
	m.mu.RLock()
	defer m.mu.RUnlock()

	result := make(map[string]*Model, len(m.models))
	maps.Copy(result, m.models)
	return result
}

// ForEach applies a function to each model. The function should not modify the model.
// If the function returns false, iteration stops early.
func (m *Models) ForEach(fn func(id string, model *Model) bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	for id, model := range m.models {
		if !fn(id, model) {
			break
		}
	}
}

// Clear removes all models.
func (m *Models) Clear() {
	m.mu.Lock()
	defer m.mu.Unlock()
	// Clear existing map instead of allocating new one
	for k := range m.models {
		delete(m.models, k)
	}
}

// AddBatch adds multiple models in a single operation.
// Only adds models that don't already exist - fails if a model ID already exists.
// Returns a map of model IDs to errors for any failed additions.
func (m *Models) AddBatch(models []*Model) map[string]error {
	if len(models) == 0 {
		return nil
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	errors := make(map[string]error)

	// First pass: validate all models
	for _, model := range models {
		if model == nil {
			continue // Skip nil models
		}
		if _, exists := m.models[model.ID]; exists {
			errors[model.ID] = fmt.Errorf("model with ID %s already exists", model.ID)
		}
	}

	// Second pass: add valid models
	for _, model := range models {
		if model == nil {
			continue
		}
		if _, hasError := errors[model.ID]; !hasError {
			m.models[model.ID] = model
		}
	}

	if len(errors) == 0 {
		return nil
	}
	return errors
}

// SetBatch sets multiple models in a single operation.
// Overwrites existing models or adds new ones (upsert behavior).
// Returns an error if any model is nil.
func (m *Models) SetBatch(models map[string]*Model) error {
	if len(models) == 0 {
		return nil
	}

	// Validate all models first
	for id, model := range models {
		if model == nil {
			return fmt.Errorf("model for ID %s cannot be nil", id)
		}
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	for id, model := range models {
		m.models[id] = model
	}

	return nil
}

// DeleteBatch removes multiple models by their IDs.
// Returns a map of errors for models that couldn't be deleted (not found).
func (m *Models) DeleteBatch(ids []string) map[string]error {
	if len(ids) == 0 {
		return nil
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	errors := make(map[string]error)
	for _, id := range ids {
		if _, exists := m.models[id]; !exists {
			errors[id] = fmt.Errorf("model with ID %s not found", id)
		} else {
			delete(m.models, id)
		}
	}

	if len(errors) == 0 {
		return nil
	}
	return errors
}
