package catalogs

import (
	"fmt"
	"maps"
	"sync"
)

// Endpoints is a concurrent safe map of endpoints.
type Endpoints struct {
	mu        sync.RWMutex
	endpoints map[string]*Endpoint
}

// EndpointsOption defines a function that configures an Endpoints instance.
type EndpointsOption func(*Endpoints)

// WithEndpointsCapacity sets the initial capacity of the endpoints map.
func WithEndpointsCapacity(capacity int) EndpointsOption {
	return func(e *Endpoints) {
		e.endpoints = make(map[string]*Endpoint, capacity)
	}
}

// WithEndpointsMap initializes the map with existing endpoints.
func WithEndpointsMap(endpoints map[string]*Endpoint) EndpointsOption {
	return func(e *Endpoints) {
		if endpoints != nil {
			e.endpoints = make(map[string]*Endpoint, len(endpoints))
			maps.Copy(e.endpoints, endpoints)
		}
	}
}

// NewEndpoints creates a new Endpoints map with optional configuration.
func NewEndpoints(opts ...EndpointsOption) *Endpoints {
	e := &Endpoints{
		endpoints: make(map[string]*Endpoint),
	}

	for _, opt := range opts {
		opt(e)
	}

	return e
}

// Get returns an endpoint by id and whether it exists.
func (e *Endpoints) Get(id string) (*Endpoint, bool) {
	e.mu.RLock()
	endpoint, ok := e.endpoints[id]
	e.mu.RUnlock()
	return endpoint, ok
}

// Set sets an endpoint by id. Returns an error if endpoint is nil.
func (e *Endpoints) Set(id string, endpoint *Endpoint) error {
	if endpoint == nil {
		return fmt.Errorf("endpoint cannot be nil")
	}

	e.mu.Lock()
	defer e.mu.Unlock()
	e.endpoints[id] = endpoint
	return nil
}

// Add adds an endpoint, returning an error if it already exists.
func (e *Endpoints) Add(endpoint *Endpoint) error {
	if endpoint == nil {
		return fmt.Errorf("endpoint cannot be nil")
	}

	e.mu.Lock()
	defer e.mu.Unlock()

	if _, exists := e.endpoints[endpoint.ID]; exists {
		return fmt.Errorf("endpoint with ID %s already exists", endpoint.ID)
	}

	e.endpoints[endpoint.ID] = endpoint
	return nil
}

// Delete removes an endpoint by id. Returns an error if the endpoint doesn't exist.
func (e *Endpoints) Delete(id string) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	if _, exists := e.endpoints[id]; !exists {
		return fmt.Errorf("endpoint with ID %s not found", id)
	}

	delete(e.endpoints, id)
	return nil
}

// Exists checks if an endpoint exists without returning it.
func (e *Endpoints) Exists(id string) bool {
	e.mu.RLock()
	_, exists := e.endpoints[id]
	e.mu.RUnlock()
	return exists
}

// Len returns the number of endpoints.
func (e *Endpoints) Len() int {
	e.mu.RLock()
	length := len(e.endpoints)
	e.mu.RUnlock()
	return length
}

// List returns a slice of all endpoints.
func (e *Endpoints) List() []*Endpoint {
	e.mu.RLock()
	endpoints := make([]*Endpoint, len(e.endpoints))
	i := 0
	for _, endpoint := range e.endpoints {
		endpoints[i] = endpoint
		i++
	}
	e.mu.RUnlock()
	return endpoints
}

// Map returns a copy of all endpoints.
func (e *Endpoints) Map() map[string]*Endpoint {
	e.mu.RLock()
	defer e.mu.RUnlock()

	result := make(map[string]*Endpoint, len(e.endpoints))
	maps.Copy(result, e.endpoints)
	return result
}

// ForEach applies a function to each endpoint. The function should not modify the endpoint.
// If the function returns false, iteration stops early.
func (e *Endpoints) ForEach(fn func(id string, endpoint *Endpoint) bool) {
	e.mu.RLock()
	defer e.mu.RUnlock()

	for id, endpoint := range e.endpoints {
		if !fn(id, endpoint) {
			break
		}
	}
}

// Clear removes all endpoints.
func (e *Endpoints) Clear() {
	e.mu.Lock()
	defer e.mu.Unlock()
	// Clear existing map instead of allocating new one
	for k := range e.endpoints {
		delete(e.endpoints, k)
	}
}

// AddBatch adds multiple endpoints in a single operation.
// Only adds endpoints that don't already exist - fails if an endpoint ID already exists.
// Returns a map of endpoint IDs to errors for any failed additions.
func (e *Endpoints) AddBatch(endpoints []*Endpoint) map[string]error {
	if len(endpoints) == 0 {
		return nil
	}

	e.mu.Lock()
	defer e.mu.Unlock()

	errors := make(map[string]error)

	// First pass: validate all endpoints
	for _, endpoint := range endpoints {
		if endpoint == nil {
			continue // Skip nil endpoints
		}
		if _, exists := e.endpoints[endpoint.ID]; exists {
			errors[endpoint.ID] = fmt.Errorf("endpoint with ID %s already exists", endpoint.ID)
		}
	}

	// Second pass: add valid endpoints
	for _, endpoint := range endpoints {
		if endpoint == nil {
			continue
		}
		if _, hasError := errors[endpoint.ID]; !hasError {
			e.endpoints[endpoint.ID] = endpoint
		}
	}

	if len(errors) == 0 {
		return nil
	}
	return errors
}

// SetBatch sets multiple endpoints in a single operation.
// Overwrites existing endpoints or adds new ones (upsert behavior).
// Returns an error if any endpoint is nil.
func (e *Endpoints) SetBatch(endpoints map[string]*Endpoint) error {
	if len(endpoints) == 0 {
		return nil
	}

	// Validate all endpoints first
	for id, endpoint := range endpoints {
		if endpoint == nil {
			return fmt.Errorf("endpoint for ID %s cannot be nil", id)
		}
	}

	e.mu.Lock()
	defer e.mu.Unlock()

	for id, endpoint := range endpoints {
		e.endpoints[id] = endpoint
	}

	return nil
}

// DeleteBatch removes multiple endpoints by their IDs.
// Returns a map of errors for endpoints that couldn't be deleted (not found).
func (e *Endpoints) DeleteBatch(ids []string) map[string]error {
	if len(ids) == 0 {
		return nil
	}

	e.mu.Lock()
	defer e.mu.Unlock()

	errors := make(map[string]error)
	for _, id := range ids {
		if _, exists := e.endpoints[id]; !exists {
			errors[id] = fmt.Errorf("endpoint with ID %s not found", id)
		} else {
			delete(e.endpoints, id)
		}
	}

	if len(errors) == 0 {
		return nil
	}
	return errors
}
