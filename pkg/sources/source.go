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
	"time"

	"github.com/agentstation/starmap/pkg/catalogs"
	"github.com/agentstation/starmap/pkg/catalogs/evidence"
)

// ID is the source-owned spelling of the shared evidence source identity.
type ID = evidence.SourceID

// Source identifiers use names that read clearly at the Source API.
const (
	ProvidersID       = evidence.ProvidersID
	ModelsDevGitID    = evidence.ModelsDevGitID
	ModelsDevHTTPID   = evidence.ModelsDevHTTPID
	LocalCatalogID    = evidence.LocalCatalogID
	ReleaseArtifactID = evidence.ReleaseArtifactID
	EmbeddedCatalogID = evidence.EmbeddedCatalogID
)

// IDs returns all available source identifiers.
func IDs() []ID {
	return evidence.SourceIDs()
}

// Source observes catalog information from one configured upstream.
//
// Implementations must be safe for repeated and concurrent Observe calls.
// Observe returns the complete result of that call directly and must not require
// a prior call or publish mutable result state through the Source.
type Source interface {
	// ID returns the stable identity of this source.
	ID() ID

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
	ID               string                  `json:"id" yaml:"id"`
	SourceID         ID                      `json:"source" yaml:"source"`
	ObservedAt       time.Time               `json:"observed_at" yaml:"observed_at"`
	Revision         Revision                `json:"revision" yaml:"revision"`
	Completeness     ObservationCompleteness `json:"completeness" yaml:"completeness"`
	Status           ObservationStatus       `json:"status" yaml:"status"`
	Records          ObservationRecordCounts `json:"records" yaml:"records"`
	Issues           []ObservationIssue      `json:"issues,omitempty" yaml:"issues,omitempty"`
	EvidenceChecksum string                  `json:"evidence_checksum" yaml:"evidence_checksum"`
	Catalog          *catalogs.Catalog       `json:"-" yaml:"-"`
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
