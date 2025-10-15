// Package sources defines interfaces and types for catalog data sources.
// Sources are responsible for fetching and synchronizing model data from
// various providers including local files, provider APIs, and external repositories.
//
// The package provides a unified interface for different data sources while
// supporting merge strategies, authorities for data precedence, and flexible
// configuration options.
//
// Example usage:
//
//	// Create a provider fetcher
//	fetcher := NewProviderFetcher(providers)
//
//	// Fetch models from a provider
//	models, err := fetcher.FetchModels(ctx, provider)
//	if err != nil {
//	    log.Fatal(err)
//	}
//
//	// Check if a provider is supported
//	if fetcher.HasClient(providerID) {
//	    // Provider has a client implementation
//	}
package sources

import (
	"context"
	"slices"
	"sync"

	"github.com/agentstation/starmap/pkg/catalogs"
)

// Sources is a thread-safe container for managing multiple data sources.
type Sources struct {
	mu      sync.RWMutex
	sources map[ID]Source
}

// NewSources creates a new Sources instance.
func NewSources() *Sources {
	return &Sources{
		sources: make(map[ID]Source),
	}
}

// Get returns a source by ID.
func (s *Sources) Get(id ID) (Source, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	src, found := s.sources[id]
	return src, found
}

// Set sets a source by ID.
func (s *Sources) Set(id ID, src Source) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.sources[id] = src
}

// Delete deletes a source by ID.
func (s *Sources) Delete(id ID) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.sources, id)
}

// Len returns the number of sources.
func (s *Sources) Len() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return len(s.sources)
}

// List returns a slice of all sources.
func (s *Sources) List() []Source {
	s.mu.RLock()
	defer s.mu.RUnlock()
	sources := make([]Source, 0, len(s.sources))
	for _, src := range s.sources {
		sources = append(sources, src)
	}
	return sources
}

// IDs returns a slice of all source IDs.
func (s *Sources) IDs() []ID {
	s.mu.RLock()
	defer s.mu.RUnlock()
	ids := make([]ID, 0, len(s.sources))
	for id := range s.sources {
		ids = append(ids, id)
	}
	return ids
}

// ID represents the identifier of a data source.
type ID string

// String returns the string representation of a source name.
func (id ID) String() string {
	return string(id)
}

// Common source names.
const (
	ProvidersID     ID = "providers"
	ModelsDevGitID  ID = "models_dev_git"
	ModelsDevHTTPID ID = "models_dev_http"
	LocalCatalogID  ID = "local_catalog"
)

// IDs returns all available source types.
// This provides a convenient way to iterate over all Type values.
func IDs() []ID {
	return []ID{
		ProvidersID,
		ModelsDevGitID,
		ModelsDevHTTPID,
		LocalCatalogID,
	}
}

// IsValid returns true if the ID is one of the defined constants.
// Uses IDs() to ensure consistency with the authoritative id list.
func (id ID) IsValid() bool {
	return slices.Contains(IDs(), id)
}

// Source represents a data source for catalog information.
type Source interface {
	// Type returns the type of this source
	ID() ID

	// Fetch retrieves data from this source
	// Sources handle their own concurrency internally
	Fetch(ctx context.Context, opts ...Option) error

	// Catalog returns the catalog of this source
	Catalog() catalogs.Catalog

	// Cleanup releases any resources (called after all Fetch operations)
	Cleanup() error
}

// ResourceType identifies the type of resource being merged.
type ResourceType string

const (
	// ResourceTypeModel represents a model resource.
	ResourceTypeModel ResourceType = "model"
	// ResourceTypeProvider represents a provider resource.
	ResourceTypeProvider ResourceType = "provider"
	// ResourceTypeAuthor represents an author resource.
	ResourceTypeAuthor ResourceType = "author"
)
