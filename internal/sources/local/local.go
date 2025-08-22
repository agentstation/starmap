package local

import (
	"context"
	"fmt"

	"github.com/agentstation/starmap/internal/catalogs/persistence"
	"github.com/agentstation/starmap/internal/sources/registry"
	"github.com/agentstation/starmap/pkg/catalogs"
	"github.com/agentstation/starmap/pkg/sources"
)

func init() {
	// Register a single instance
	registry.Register(&Local{
		priority: 80,
		name:     "Local Catalog",
	})
}

// Local wraps the existing catalog as a Source
type Local struct {
	catalog  catalogs.Catalog
	priority int
	name     string
}

// Type returns the source type
func (s *Local) Type() sources.Type {
	return sources.LocalCatalog
}

// Configure prepares the source with runtime configuration
func (s *Local) Configure(config sources.SourceConfig) error {
	if config.SyncOptions != nil && config.SyncOptions.DisableLocalCatalog {
		s.catalog = nil
		return nil
	}
	s.catalog = config.Catalog
	return nil
}

// Reset clears any configuration
func (s *Local) Reset() {
	s.catalog = nil
}

// Clone creates a copy of this source for concurrent use
func (s *Local) Clone() sources.Source {
	return &Local{
		catalog:  s.catalog,
		priority: s.priority,
		name:     s.name,
	}
}

// Name returns a human-readable name for this source
func (s *Local) Name() string {
	return s.name
}

// Priority returns the priority of this source
func (s *Local) Priority() int {
	return s.priority
}

// FetchModels fetches models from the local catalog
func (s *Local) FetchModels(ctx context.Context, providerID catalogs.ProviderID) ([]catalogs.Model, error) {
	if s.catalog == nil {
		return nil, fmt.Errorf("local catalog not available")
	}

	// Get existing models from the catalog for this provider
	existingModels, err := persistence.GetProviderModels(s.catalog, providerID)
	if err != nil {
		// Not necessarily an error - provider might not exist in catalog yet
		return nil, nil
	}

	// Convert map to slice
	var models []catalogs.Model
	for _, model := range existingModels {
		models = append(models, model)
	}

	return models, nil
}

// FetchProvider fetches provider information from the local catalog
func (s *Local) FetchProvider(ctx context.Context, providerID catalogs.ProviderID) (*catalogs.Provider, error) {
	if s.catalog == nil {
		return nil, fmt.Errorf("local catalog not available")
	}

	// Get provider from catalog
	provider, found := s.catalog.Providers().Get(providerID)
	if !found {
		// Not an error - provider might not exist yet
		return nil, nil
	}

	// Return the provider pointer directly
	return provider, nil
}

// FieldAuthorities returns the field authorities where local catalog is authoritative
func (s *Local) FieldAuthorities() []sources.FieldAuthority {
	return sources.FilterAuthoritiesBySource(sources.DefaultModelFieldAuthorities, sources.LocalCatalog)
}

// IsAvailable returns true if the local catalog is available
func (s *Local) IsAvailable() bool {
	return s.catalog != nil
}

// SetPriority updates the priority of this source
func (s *Local) SetPriority(priority int) {
	s.priority = priority
}
