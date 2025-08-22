package provider

import (
	"context"
	"fmt"

	"github.com/agentstation/starmap/internal/sources/registry"
	"github.com/agentstation/starmap/pkg/catalogs"
	"github.com/agentstation/starmap/pkg/sources"
)

func init() {
	// Register a single instance that handles all providers
	registry.Register(&ProviderAPISource{
		priority: 90,
		name:     "Provider API",
	})
}

// ProviderAPISource wraps provider API clients as a Source
type ProviderAPISource struct {
	provider *catalogs.Provider
	client   catalogs.Client
	priority int
	name     string
}

// Type returns the source type
func (s *ProviderAPISource) Type() sources.Type {
	return sources.ProviderAPI
}

// Configure prepares the source with runtime configuration
func (s *ProviderAPISource) Configure(config sources.SourceConfig) error {
	if config.SyncOptions != nil && config.SyncOptions.DisableProviderAPI {
		s.provider = nil
		s.client = nil
		return nil
	}

	if config.Provider == nil {
		s.provider = nil
		s.client = nil
		return nil
	}

	// Get client for this provider
	clientResult, err := config.Provider.Client(catalogs.WithAllowMissingAPIKey(true))
	if err != nil || clientResult.Client == nil {
		s.provider = nil
		s.client = nil
		return nil // Not an error, just unavailable
	}

	s.provider = config.Provider
	s.client = clientResult.Client
	s.name = fmt.Sprintf("Provider API (%s)", config.Provider.ID)
	return nil
}

// Reset clears any configuration
func (s *ProviderAPISource) Reset() {
	s.provider = nil
	s.client = nil
}

// Clone creates a copy of this source for concurrent use
func (s *ProviderAPISource) Clone() sources.Source {
	return &ProviderAPISource{
		provider: s.provider,
		client:   s.client,
		priority: s.priority,
		name:     s.name,
	}
}

// Name returns a human-readable name for this source
func (s *ProviderAPISource) Name() string {
	return s.name
}

// Priority returns the priority of this source
func (s *ProviderAPISource) Priority() int {
	return s.priority
}

// FetchModels fetches models from the provider API
func (s *ProviderAPISource) FetchModels(ctx context.Context, providerID catalogs.ProviderID) ([]catalogs.Model, error) {
	if s.provider.ID != providerID {
		return nil, fmt.Errorf("provider ID mismatch: expected %s, got %s", providerID, s.provider.ID)
	}

	if s.client == nil {
		return nil, fmt.Errorf("no client available for provider %s", providerID)
	}

	// Use existing provider client infrastructure
	models, err := s.client.ListModels(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch models from provider API: %w", err)
	}

	return models, nil
}

// FetchProvider fetches provider information from the API (limited info available)
func (s *ProviderAPISource) FetchProvider(ctx context.Context, providerID catalogs.ProviderID) (*catalogs.Provider, error) {
	if s.provider.ID != providerID {
		return nil, fmt.Errorf("provider ID mismatch: expected %s, got %s", providerID, s.provider.ID)
	}

	// Return the actual provider object which may have been updated by the client
	// (e.g., publishers discovered during ListModels call)
	return s.provider, nil
}

// FieldAuthorities returns the field authorities where Provider API is authoritative
func (s *ProviderAPISource) FieldAuthorities() []sources.FieldAuthority {
	return sources.FilterAuthoritiesBySource(sources.DefaultModelFieldAuthorities, sources.ProviderAPI)
}

// IsAvailable returns true if the provider API client is available
func (s *ProviderAPISource) IsAvailable() bool {
	return s.client != nil && s.provider != nil
}

// SetPriority updates the priority of this source
func (s *ProviderAPISource) SetPriority(priority int) {
	s.priority = priority
}
