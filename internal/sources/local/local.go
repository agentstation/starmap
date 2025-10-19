package local

import (
	"context"

	"github.com/agentstation/starmap/pkg/catalogs"
	"github.com/agentstation/starmap/pkg/errors"
	"github.com/agentstation/starmap/pkg/sources"
)

// Source loads a catalog from either a file path or embedded catalog.
type Source struct {
	catalogPath     string
	catalog         catalogs.Catalog
	catalogProvided bool // Track if catalog was provided via WithCatalog option
}

// New creates a new local source.
func New(opts ...Option) *Source {
	s := &Source{}
	for _, opt := range opts {
		opt(s)
	}
	return s
}

// Option configures a local source.
type Option func(*Source)

// WithCatalogPath sets the catalog path.
func WithCatalogPath(path string) Option {
	return func(s *Source) {
		s.catalogPath = path
	}
}

// WithCatalog sets a pre-loaded catalog to reuse.
// This allows reusing an already-merged catalog instead of loading again.
func WithCatalog(catalog catalogs.Catalog) Option {
	return func(s *Source) {
		s.catalog = catalog
		s.catalogProvided = true
	}
}

// ID returns the ID of this source.
func (s *Source) ID() sources.ID {
	// For local source, we always return the constant name
	// The path details can be logged separately if needed
	return sources.LocalCatalogID
}

// Name returns the human-friendly name of this source.
func (s *Source) Name() string { return "Local Catalog" }

// Fetch returns catalog data from configured source.
func (s *Source) Fetch(_ context.Context, _ ...sources.Option) error {
	// If catalog was provided via WithCatalog option, reuse it
	if s.catalogProvided {
		// Catalog already set, nothing to fetch
		return nil
	}

	// Otherwise, load using NewLocal logic
	var err error
	s.catalog, err = catalogs.NewLocal(s.catalogPath)
	if err != nil {
		if s.catalogPath != "" {
			return errors.WrapResource("load", "catalog", s.catalogPath, err)
		}
		return errors.WrapResource("load", "embedded catalog", "", err)
	}
	s.catalog.SetMergeStrategy(catalogs.MergeReplaceAll)
	return nil
}

// Catalog returns the catalog of this source.
func (s *Source) Catalog() catalogs.Catalog {
	return s.catalog
}

// Cleanup releases any resources.
func (s *Source) Cleanup() error {
	// LocalSource doesn't hold any resources
	return nil
}

// Dependencies returns the list of external dependencies.
// Local source has no external dependencies.
func (s *Source) Dependencies() []sources.Dependency {
	return nil
}

// IsOptional returns whether this source is optional.
// Local source is optional - we can fall back to embedded catalog.
func (s *Source) IsOptional() bool {
	return true
}
