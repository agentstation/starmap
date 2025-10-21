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
	"sync"

	"github.com/agentstation/starmap/pkg/catalogs"
	"github.com/agentstation/starmap/pkg/types"
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
// ID is a type alias for types.SourceID to maintain backward compatibility.
// This allows existing code to continue using sources.ID while benefiting from
// the shared type definitions in pkg/types.
type ID = types.SourceID

// Common source identifiers - exported as package-level constants for convenience.
const (
	ProvidersID     = types.ProvidersID
	ModelsDevGitID  = types.ModelsDevGitID
	ModelsDevHTTPID = types.ModelsDevHTTPID
	LocalCatalogID  = types.LocalCatalogID
)

// IDs returns all available source identifiers.
// Delegates to types.SourceIDs() to maintain consistency.
func IDs() []ID {
	return types.SourceIDs()
}

// Source represents a data source for catalog information.
type Source interface {
	// Type returns the type of this source
	ID() ID

	// Name returns a human-friendly name for this source
	Name() string

	// Fetch retrieves data from this source
	// Sources handle their own concurrency internally
	Fetch(ctx context.Context, opts ...Option) error

	// Catalog returns the catalog of this source
	Catalog() catalogs.Catalog

	// Cleanup releases any resources (called after all Fetch operations)
	Cleanup() error

	// Dependencies returns the list of external dependencies this source requires
	Dependencies() []Dependency

	// IsOptional returns true if the sync can succeed without this source
	IsOptional() bool
}

// Dependency represents an external tool or runtime required by a source.
type Dependency struct {
	// Core identification
	Name        string // Machine name: "bun", "git", "docker"
	DisplayName string // Human-readable: "Bun JavaScript runtime"
	Required    bool   // false = source is optional or has fallback

	// Checking availability
	CheckCommands []string // Try in order: ["bun", "bunx"]
	MinVersion    string   // Optional: "1.0.0"

	// Installation
	InstallURL         string // https://bun.sh/docs/installation
	AutoInstallCommand string // Optional: "curl -fsSL https://bun.sh/install | bash"

	// User messaging
	Description       string // "Builds models.dev data locally (same as HTTP source)"
	WhyNeeded         string // "Required to build api.json from TypeScript source"
	AlternativeSource string // "models_dev_http provides same data without dependencies"
}

// DependencyStatus represents the availability status of a dependency.
type DependencyStatus struct {
	Available  bool   // Whether the dependency is available
	Version    string // Version string if available and detectable
	Path       string // Full path to executable if found
	CheckError error  // Error from check command if not available
}

// ResourceType is a type alias for types.ResourceType to maintain backward compatibility.
// This allows existing code to continue using sources.ResourceType while benefiting from
// the shared type definitions in pkg/types.
type ResourceType = types.ResourceType

// Common resource type identifiers - exported as package-level constants for convenience.
const (
	ResourceTypeModel    = types.ResourceTypeModel
	ResourceTypeProvider = types.ResourceTypeProvider
	ResourceTypeAuthor   = types.ResourceTypeAuthor
)
