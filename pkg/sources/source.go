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

// Type represents the type/name of a data source.
type Type string

// String returns the string representation of a source name.
func (sn Type) String() string {
	return string(sn)
}

// Common source names.
const (
	ProviderAPI   Type = "Provider APIs"
	ModelsDevGit  Type = "models.dev (git)"
	ModelsDevHTTP Type = "models.dev (http)"
	LocalCatalog  Type = "Local Catalog"
)

// Source represents a data source for catalog information.
type Source interface {
	// Type returns the type of this source
	Type() Type

	// Setup initializes the source with dependencies (called once before Fetch)
	Setup(providers *catalogs.Providers) error

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
