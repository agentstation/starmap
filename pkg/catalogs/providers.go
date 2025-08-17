package catalogs

import (
	"fmt"
	"maps"
	"sync"
)

// Providers is a concurrent safe map of providers.
type Providers struct {
	mu        sync.RWMutex
	providers map[ProviderID]*Provider
}

// ProvidersOption defines a function that configures a Providers instance.
type ProvidersOption func(*Providers)

// WithProvidersCapacity sets the initial capacity of the providers map.
func WithProvidersCapacity(capacity int) ProvidersOption {
	return func(p *Providers) {
		p.providers = make(map[ProviderID]*Provider, capacity)
	}
}

// WithProvidersMap initializes the map with existing providers.
func WithProvidersMap(providers map[ProviderID]*Provider) ProvidersOption {
	return func(p *Providers) {
		if providers != nil {
			p.providers = make(map[ProviderID]*Provider, len(providers))
			maps.Copy(p.providers, providers)
		}
	}
}

// NewProviders creates a new Providers map with optional configuration.
func NewProviders(opts ...ProvidersOption) *Providers {
	p := &Providers{
		providers: make(map[ProviderID]*Provider),
	}

	for _, opt := range opts {
		opt(p)
	}

	return p
}

// Get returns a provider by id and whether it exists.
func (p *Providers) Get(id ProviderID) (*Provider, bool) {
	p.mu.RLock()
	provider, ok := p.providers[id]
	p.mu.RUnlock()
	return provider, ok
}

// Set sets a provider by id. Returns an error if provider is nil.
func (p *Providers) Set(id ProviderID, provider *Provider) error {
	if provider == nil {
		return fmt.Errorf("provider cannot be nil")
	}

	p.mu.Lock()
	defer p.mu.Unlock()
	p.providers[id] = provider
	return nil
}

// Add adds a provider, returning an error if it already exists.
func (p *Providers) Add(provider *Provider) error {
	if provider == nil {
		return fmt.Errorf("provider cannot be nil")
	}

	p.mu.Lock()
	defer p.mu.Unlock()

	if _, exists := p.providers[provider.ID]; exists {
		return fmt.Errorf("provider with ID %s already exists", provider.ID)
	}

	p.providers[provider.ID] = provider
	return nil
}

// Delete removes a provider by id. Returns an error if the provider doesn't exist.
func (p *Providers) Delete(id ProviderID) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	if _, exists := p.providers[id]; !exists {
		return fmt.Errorf("provider with ID %s not found", id)
	}

	delete(p.providers, id)
	return nil
}

// Exists checks if a provider exists without returning it.
func (p *Providers) Exists(id ProviderID) bool {
	p.mu.RLock()
	_, exists := p.providers[id]
	p.mu.RUnlock()
	return exists
}

// Len returns the number of providers.
func (p *Providers) Len() int {
	p.mu.RLock()
	length := len(p.providers)
	p.mu.RUnlock()
	return length
}

// List returns a slice of all providers.
func (p *Providers) List() []*Provider {
	p.mu.RLock()
	providers := make([]*Provider, len(p.providers))
	i := 0
	for _, provider := range p.providers {
		providers[i] = provider
		i++
	}
	p.mu.RUnlock()
	return providers
}

// Map returns a copy of all providers.
func (p *Providers) Map() map[ProviderID]*Provider {
	p.mu.RLock()
	defer p.mu.RUnlock()

	result := make(map[ProviderID]*Provider, len(p.providers))
	maps.Copy(result, p.providers)
	return result
}

// ForEach applies a function to each provider. The function should not modify the provider.
// If the function returns false, iteration stops early.
func (p *Providers) ForEach(fn func(id ProviderID, provider *Provider) bool) {
	p.mu.RLock()
	defer p.mu.RUnlock()

	for id, provider := range p.providers {
		if !fn(id, provider) {
			break
		}
	}
}

// Clear removes all providers.
func (p *Providers) Clear() {
	p.mu.Lock()
	defer p.mu.Unlock()
	// Clear existing map instead of allocating new one
	for k := range p.providers {
		delete(p.providers, k)
	}
}

// AddBatch adds multiple providers in a single operation.
// Only adds providers that don't already exist - fails if a provider ID already exists.
// Returns a map of provider IDs to errors for any failed additions.
func (p *Providers) AddBatch(providers []*Provider) map[ProviderID]error {
	if len(providers) == 0 {
		return nil
	}

	p.mu.Lock()
	defer p.mu.Unlock()

	errors := make(map[ProviderID]error)

	// First pass: validate all providers
	for _, provider := range providers {
		if provider == nil {
			continue // Skip nil providers
		}
		if _, exists := p.providers[provider.ID]; exists {
			errors[provider.ID] = fmt.Errorf("provider with ID %s already exists", provider.ID)
		}
	}

	// Second pass: add valid providers
	for _, provider := range providers {
		if provider == nil {
			continue
		}
		if _, hasError := errors[provider.ID]; !hasError {
			p.providers[provider.ID] = provider
		}
	}

	if len(errors) == 0 {
		return nil
	}
	return errors
}

// SetBatch sets multiple providers in a single operation.
// Overwrites existing providers or adds new ones (upsert behavior).
// Returns an error if any provider is nil.
func (p *Providers) SetBatch(providers map[ProviderID]*Provider) error {
	if len(providers) == 0 {
		return nil
	}

	// Validate all providers first
	for id, provider := range providers {
		if provider == nil {
			return fmt.Errorf("provider for ID %s cannot be nil", id)
		}
	}

	p.mu.Lock()
	defer p.mu.Unlock()

	for id, provider := range providers {
		p.providers[id] = provider
	}

	return nil
}

// DeleteBatch removes multiple providers by their IDs.
// Returns a map of errors for providers that couldn't be deleted (not found).
func (p *Providers) DeleteBatch(ids []ProviderID) map[ProviderID]error {
	if len(ids) == 0 {
		return nil
	}

	p.mu.Lock()
	defer p.mu.Unlock()

	errors := make(map[ProviderID]error)
	for _, id := range ids {
		if _, exists := p.providers[id]; !exists {
			errors[id] = fmt.Errorf("provider with ID %s not found", id)
		} else {
			delete(p.providers, id)
		}
	}

	if len(errors) == 0 {
		return nil
	}
	return errors
}
