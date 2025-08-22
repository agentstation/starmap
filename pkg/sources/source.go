package sources

import (
	"context"

	"github.com/agentstation/starmap/pkg/catalogs"
)

// Type represents different types of data sources
type Type string

const (
	ProviderAPI   Type = "provider_api"
	ModelsDevGit  Type = "models_dev_git"
	ModelsDevHTTP Type = "models_dev_http"
	LocalCatalog  Type = "local_catalog"
	DatabaseUI    Type = "database_ui"
)

// String returns the string representation
func (st Type) String() string {
	return string(st)
}

// Source represents a data source for catalog information
type Source interface {
	// Type returns the type of this source
	Type() Type

	// Name returns a human-readable name for this source
	Name() string

	// Priority returns the default priority for this source (higher = more important)
	Priority() int

	// Configure prepares the source with runtime configuration
	// Returns nil if configuration successful, error otherwise
	Configure(config SourceConfig) error

	// Reset clears any configuration
	Reset()

	// IsAvailable returns true if properly configured and ready
	IsAvailable() bool

	// Clone creates a copy of this source for concurrent use
	Clone() Source

	// FetchModels fetches models from this source for a given provider
	FetchModels(ctx context.Context, providerID catalogs.ProviderID) ([]catalogs.Model, error)

	// FetchProvider fetches provider information from this source
	FetchProvider(ctx context.Context, providerID catalogs.ProviderID) (*catalogs.Provider, error)

	// FieldAuthorities returns the field authorities for this source
	FieldAuthorities() []FieldAuthority
}

// SourceConfig provides runtime configuration for sources
type SourceConfig struct {
	Catalog     catalogs.Catalog
	Provider    *catalogs.Provider // For provider-specific sources
	SyncOptions *SyncOptions       // Contains all sync configuration
}
