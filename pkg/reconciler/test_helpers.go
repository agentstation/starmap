package reconciler

import (
	"sort"

	"github.com/agentstation/starmap/pkg/catalogs"
	"github.com/agentstation/starmap/pkg/sources"
)

// NewMockSource creates a direct observation for testing.
func NewMockSource(sourceType sources.ID, catalog *catalogs.Builder) sources.Observation {
	snapshot, err := catalog.Build()
	if err != nil {
		return sources.Observation{SourceID: sourceType}
	}
	return sources.Observation{SourceID: sourceType, Catalog: snapshot}
}

// ConvertCatalogsMapToSources converts the old map format to sources slice for testing.
func ConvertCatalogsMapToSources(srcs map[sources.ID]*catalogs.Builder) []sources.Observation {
	// Get all source types and sort them for deterministic ordering
	types := make([]sources.ID, 0, len(srcs))
	for sourceType := range srcs {
		types = append(types, sourceType)
	}
	sort.Slice(types, func(i, j int) bool {
		return string(types[i]) < string(types[j])
	})

	// Create sources in sorted order
	observations := make([]sources.Observation, 0, len(srcs))
	for _, sourceType := range types {
		observations = append(observations, NewMockSource(sourceType, srcs[sourceType]))
	}
	return observations
}
