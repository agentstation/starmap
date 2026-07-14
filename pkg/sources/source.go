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
	"time"

	"github.com/agentstation/starmap/pkg/catalogmeta"
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
// ID is the source-domain spelling of catalogmeta.SourceID.
// This allows existing code to continue using sources.ID while benefiting from
// the shared type definitions in pkg/catalogmeta.
type ID = catalogmeta.SourceID

// Common source identifiers - exported as package-level constants for convenience.
const (
	ProvidersID           = catalogmeta.ProvidersID
	ModelsDevGitID        = catalogmeta.ModelsDevGitID
	ModelsDevHTTPID       = catalogmeta.ModelsDevHTTPID
	LocalCatalogID        = catalogmeta.LocalCatalogID
	AmazonBedrockID       = catalogmeta.AmazonBedrockID
	MicrosoftFoundryID    = catalogmeta.MicrosoftFoundryID
	OCIGenerativeAIID     = catalogmeta.OCIGenerativeAIID
	DatabricksWorkspaceID = catalogmeta.DatabricksWorkspaceID
	WatsonxDeploymentsID  = catalogmeta.WatsonxDeploymentsID
	validationCannotBeNil = "cannot be nil"
	validationIsRequired  = "is required"
)

// IDs returns all available source identifiers.
// Delegates to catalogmeta.SourceIDs() to maintain consistency.
func IDs() []ID {
	return catalogmeta.SourceIDs()
}

// Source observes catalog information from one configured upstream.
//
// Implementations must be safe for repeated and concurrent Observe calls.
// Observe returns the complete result of that call directly and must not require
// a prior call or publish mutable result state through the Source.
type Source interface {
	// ID returns the stable identity of this source.
	ID() ID

	// Name returns a human-friendly name for this source
	Name() string

	// Observe retrieves and returns one immutable source result directly. Calls
	// must not depend on prior Observe calls or publish result state on Source.
	Observe(ctx context.Context, opts ...Option) (Observation, error)

	// Cleanup releases resources after all Observe calls have completed.
	Cleanup() error

	// Dependencies returns the list of external dependencies this source requires
	Dependencies() []Dependency

	// IsOptional returns true if the sync can succeed without this source
	IsOptional() bool
}

// Observation is one immutable direct source result. EvidenceChecksum binds
// the normalized canonical catalog payload; raw upstream evidence retention is
// a separate storage policy.
type Observation struct {
	ID               string                         `json:"id" yaml:"id"`
	SourceID         ID                             `json:"source" yaml:"source"`
	ObservedAt       time.Time                      `json:"observed_at" yaml:"observed_at"`
	Revision         Revision                       `json:"revision" yaml:"revision"`
	Completeness     ObservationCompleteness        `json:"completeness" yaml:"completeness"`
	Status           ObservationStatus              `json:"status" yaml:"status"`
	Records          ObservationRecordCounts        `json:"records" yaml:"records"`
	Metrics          catalogmeta.ObservationMetrics `json:"metrics" yaml:"metrics"`
	Issues           []ObservationIssue             `json:"issues,omitempty" yaml:"issues,omitempty"`
	EvidenceChecksum string                         `json:"evidence_checksum" yaml:"evidence_checksum"`
	Catalog          *catalogs.Catalog              `json:"-" yaml:"-"`
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

// ResourceType is the source-domain spelling of catalogmeta.ResourceType.
// This allows existing code to continue using sources.ResourceType while benefiting from
// the shared type definitions in pkg/catalogmeta.
type ResourceType = catalogmeta.ResourceType

// Common resource type identifiers - exported as package-level constants for convenience.
const (
	ResourceTypeModel            = catalogmeta.ResourceTypeModel
	ResourceTypeProvider         = catalogmeta.ResourceTypeProvider
	ResourceTypeAuthor           = catalogmeta.ResourceTypeAuthor
	ResourceTypeModelDefinition  = catalogmeta.ResourceTypeModelDefinition
	ResourceTypeProviderOffering = catalogmeta.ResourceTypeProviderOffering
)
