package local

import (
	"context"

	"github.com/agentstation/starmap/pkg/catalogs"
	"github.com/agentstation/starmap/pkg/errors"
	"github.com/agentstation/starmap/pkg/sources"
)

// Source loads a catalog from either a file path or embedded catalog
type Source struct {
	catalogPath string
}

// New creates a new local source
func New(opts ...Option) *Source {
	s := &Source{}
	for _, opt := range opts {
		opt(s)
	}
	return s
}

// Option configures a local source
type Option func(*Source)

// WithCatalogPath sets the catalog path
func WithCatalogPath(path string) Option {
	return func(s *Source) {
		s.catalogPath = path
	}
}

// Name returns the name of this source
func (s *Source) Name() sources.SourceName {
	// For local source, we always return the constant name
	// The path details can be logged separately if needed
	return sources.LocalCatalog
}

// Setup initializes the source with dependencies
func (s *Source) Setup(providers *catalogs.Providers) error {
	// LocalSource doesn't need any dependencies
	return nil
}

// Fetch returns catalog data from configured source
func (s *Source) Fetch(ctx context.Context, opts ...sources.SourceOption) (catalogs.Catalog, error) {
	// Apply options
	options := sources.ApplyOptions(opts...)

	// Check for runtime override in context
	if options.Context != nil {
		if path, ok := options.Context["inputPath"].(string); ok && path != "" {
			catalog, err := catalogs.New(catalogs.WithFiles(path))
			if err != nil {
				return nil, errors.WrapResource("load", "catalog", path, err)
			}
			catalog.SetMergeStrategy(catalogs.MergeReplaceAll)
			return catalog, nil
		}
	}

	// Use configured path if set
	if s.catalogPath != "" {
		catalog, err := catalogs.New(catalogs.WithFiles(s.catalogPath))
		if err != nil {
			return nil, errors.WrapResource("load", "catalog", s.catalogPath, err)
		}
		catalog.SetMergeStrategy(catalogs.MergeReplaceAll)
		return catalog, nil
	}

	// Default to embedded catalog
	catalog, err := catalogs.New(catalogs.WithEmbedded())
	if err != nil {
		return nil, errors.WrapResource("load", "embedded catalog", "", err)
	}
	catalog.SetMergeStrategy(catalogs.MergeReplaceAll)
	return catalog, nil
}

// Cleanup releases any resources
func (s *Source) Cleanup() error {
	// LocalSource doesn't hold any resources
	return nil
}
