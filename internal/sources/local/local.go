package local

import (
	"context"

	"github.com/agentstation/starmap/pkg/catalogs"
	"github.com/agentstation/starmap/pkg/errors"
	"github.com/agentstation/starmap/pkg/sources"
)

// Source loads a catalog from either a file path or embedded catalog.
type Source struct {
	catalogPath string
	catalog     catalogs.Catalog
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

// Type returns the type of this source.
func (s *Source) Type() sources.Type {
	// For local source, we always return the constant name
	// The path details can be logged separately if needed
	return sources.LocalCatalog
}

// Setup initializes the source with dependencies.
func (s *Source) Setup(_ *catalogs.Providers) error {
	// LocalSource doesn't need any dependencies
	return nil
}

// Fetch returns catalog data from configured source.
func (s *Source) Fetch(_ context.Context, _ ...sources.Option) error {
	// Use configured path if set
	if s.catalogPath != "" {
		var err error
		s.catalog, err = catalogs.New(catalogs.WithFiles(s.catalogPath))
		if err != nil {
			return errors.WrapResource("load", "catalog", s.catalogPath, err)
		}
		s.catalog.SetMergeStrategy(catalogs.MergeReplaceAll)
		return nil
	}

	// Default to embedded catalog
	var err error
	s.catalog, err = catalogs.New(catalogs.WithEmbedded())
	if err != nil {
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
