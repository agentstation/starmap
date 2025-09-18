package reconciler

import (
	"context"
	"sort"

	"github.com/agentstation/starmap/pkg/catalogs"
	"github.com/agentstation/starmap/pkg/sources"
)

// mockSource is a test implementation of sources.Source.
type mockSource struct {
	sourceType sources.ID
	catalog    catalogs.Catalog
}

// NewMockSource creates a new mock source for testing.
func NewMockSource(sourceType sources.ID, catalog catalogs.Catalog) sources.Source {
	return &mockSource{
		sourceType: sourceType,
		catalog:    catalog,
	}
}

// Type returns the type of this source.
func (m *mockSource) ID() sources.ID {
	return m.sourceType
}

// Setup initializes the source with dependencies.
func (m *mockSource) Setup(_ *catalogs.Providers) error {
	return nil
}

// Fetch retrieves data from this source.
func (m *mockSource) Fetch(_ context.Context, _ ...sources.Option) error {
	// Mock source already has its catalog, no fetching needed
	return nil
}

// Catalog returns the catalog of this source.
func (m *mockSource) Catalog() catalogs.Catalog {
	return m.catalog
}

// Cleanup releases any resources.
func (m *mockSource) Cleanup() error {
	return nil
}

// ConvertCatalogsMapToSources converts the old map format to sources slice for testing.
func ConvertCatalogsMapToSources(srcs map[sources.ID]catalogs.Catalog) []sources.Source {
	// Get all source types and sort them for deterministic ordering
	types := make([]sources.ID, 0, len(srcs))
	for sourceType := range srcs {
		types = append(types, sourceType)
	}
	sort.Slice(types, func(i, j int) bool {
		return string(types[i]) < string(types[j])
	})

	// Create sources in sorted order
	sources := make([]sources.Source, 0, len(srcs))
	for _, sourceType := range types {
		sources = append(sources, NewMockSource(sourceType, srcs[sourceType]))
	}
	return sources
}
