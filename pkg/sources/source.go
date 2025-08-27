package sources

import (
	"context"

	"github.com/agentstation/starmap/pkg/catalogs"
)

// SourceName represents the name/type of a data source
type SourceName string

// String returns the string representation of a source name
func (sn SourceName) String() string {
	return string(sn)
}

// Common source names
const (
	ProviderAPI   SourceName = "Provider APIs"
	ModelsDevGit  SourceName = "models.dev (git)"
	ModelsDevHTTP SourceName = "models.dev (http)"
	LocalCatalog  SourceName = "Local Catalog"
)

// Source represents a data source for catalog information
type Source interface {
	// Name returns the name of this source
	Name() SourceName

	// Setup initializes the source with dependencies (called once before Fetch)
	Setup(providers *catalogs.Providers) error

	// Fetch retrieves data from this source
	// Sources handle their own concurrency internally
	Fetch(ctx context.Context, opts ...SourceOption) (catalogs.Catalog, error)

	// Cleanup releases any resources (called after all Fetch operations)
	Cleanup() error
}
