package operations

import (
	"fmt"

	"github.com/agentstation/starmap/pkg/catalogs"
)

// CopyOption configures how a catalog copy operation behaves
type CopyOption func(*copyConfig)

// copyConfig holds configuration for copy operations
type copyConfig struct {
	skipProviders bool
	skipAuthors   bool
	skipModels    bool
	skipEndpoints bool
}

// WithSkipProviders skips copying providers during catalog copy
func WithSkipProviders() CopyOption {
	return func(c *copyConfig) {
		c.skipProviders = true
	}
}

// WithSkipAuthors skips copying authors during catalog copy
func WithSkipAuthors() CopyOption {
	return func(c *copyConfig) {
		c.skipAuthors = true
	}
}

// WithSkipModels skips copying models during catalog copy
func WithSkipModels() CopyOption {
	return func(c *copyConfig) {
		c.skipModels = true
	}
}

// WithSkipEndpoints skips copying endpoints during catalog copy
func WithSkipEndpoints() CopyOption {
	return func(c *copyConfig) {
		c.skipEndpoints = true
	}
}

// CopyToTarget creates a deep copy of the source catalog into the target catalog.
// All existing items in the target catalog are cleared first.
// The target catalog will contain copies of all providers, authors, models, and endpoints from the source.
func CopyToTarget(source, target catalogs.Catalog, opts ...CopyOption) error {
	// Apply options
	cfg := &copyConfig{}
	for _, opt := range opts {
		opt(cfg)
	}

	// Clear the target catalog first
	if !cfg.skipProviders {
		target.Providers().Clear()
	}
	if !cfg.skipAuthors {
		target.Authors().Clear()
	}
	if !cfg.skipModels {
		target.Models().Clear()
	}
	if !cfg.skipEndpoints {
		target.Endpoints().Clear()
	}

	// Now sync all data from source to target (this creates copies)
	return Sync(source, target)
}

// CopyWith creates a deep copy of the catalog using the provided constructor function.
// The constructor function should create a new empty catalog instance.
// Options can be provided to customize the copy behavior.
func CopyWith(c catalogs.Catalog, newCatalog func() catalogs.Catalog, opts ...CopyOption) (catalogs.Catalog, error) {
	// Apply options
	cfg := &copyConfig{}
	for _, opt := range opts {
		opt(cfg)
	}

	// Create a new empty catalog using the constructor
	catalog := newCatalog()

	// Deep copy all providers - create new instances to ensure independence
	if !cfg.skipProviders {
		if providersMap := c.Providers().Map(); len(providersMap) > 0 {
			copiedProviders := make(map[catalogs.ProviderID]*catalogs.Provider, len(providersMap))
			for id, provider := range providersMap {
				// Create a new Provider instance (deep copy the struct)
				providerCopy := *provider
				copiedProviders[id] = &providerCopy
			}
			if err := catalog.Providers().SetBatch(copiedProviders); err != nil {
				return nil, fmt.Errorf("failed to copy providers: %w", err)
			}
		}
	}

	// Deep copy all authors - create new instances to ensure independence
	if !cfg.skipAuthors {
		if authorsMap := c.Authors().Map(); len(authorsMap) > 0 {
			copiedAuthors := make(map[catalogs.AuthorID]*catalogs.Author, len(authorsMap))
			for id, author := range authorsMap {
				// Create a new Author instance (deep copy the struct)
				authorCopy := *author
				copiedAuthors[id] = &authorCopy
			}
			if err := catalog.Authors().SetBatch(copiedAuthors); err != nil {
				return nil, fmt.Errorf("failed to copy authors: %w", err)
			}
		}
	}

	// Deep copy all models - create new instances to ensure independence
	if !cfg.skipModels {
		if modelsMap := c.Models().Map(); len(modelsMap) > 0 {
			copiedModels := make(map[string]*catalogs.Model, len(modelsMap))
			for id, model := range modelsMap {
				// Create a new Model instance (deep copy the struct)
				modelCopy := *model
				copiedModels[id] = &modelCopy
			}
			if err := catalog.Models().SetBatch(copiedModels); err != nil {
				return nil, fmt.Errorf("failed to copy models: %w", err)
			}
		}
	}

	// Deep copy all endpoints - create new instances to ensure independence
	if !cfg.skipEndpoints {
		if endpointsMap := c.Endpoints().Map(); len(endpointsMap) > 0 {
			copiedEndpoints := make(map[string]*catalogs.Endpoint, len(endpointsMap))
			for id, endpoint := range endpointsMap {
				// Create a new Endpoint instance (deep copy the struct)
				endpointCopy := *endpoint
				copiedEndpoints[id] = &endpointCopy
			}
			if err := catalog.Endpoints().SetBatch(copiedEndpoints); err != nil {
				return nil, fmt.Errorf("failed to copy endpoints: %w", err)
			}
		}
	}

	return catalog, nil
}
