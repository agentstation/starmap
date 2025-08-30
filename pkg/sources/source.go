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
//	fetcher := NewProviderFetcher()
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
