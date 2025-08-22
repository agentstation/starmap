package registry

import (
	"sync"

	"github.com/agentstation/starmap/pkg/sources"
)

// Note: No longer using dependency injection pattern
// Sources package accesses registry directly

// registry holds registered source instances
var (
	mu       sync.RWMutex
	registry = make(map[sources.Type]sources.Source)
)

// Register adds a source instance to the registry
// This follows the Go pattern from database/sql
func Register(source sources.Source) {
	mu.Lock()
	defer mu.Unlock()

	if source == nil {
		panic("sources: Register source is nil")
	}

	sourceType := source.Type()
	if _, dup := registry[sourceType]; dup {
		panic("sources: Register called twice for source " + sourceType.String())
	}

	registry[sourceType] = source
}

// GetSource returns a registered source by type
func GetSource(sourceType sources.Type) (sources.Source, bool) {
	mu.RLock()
	defer mu.RUnlock()
	source, ok := registry[sourceType]
	return source, ok
}

// Sources returns all registered sources
func Sources() []sources.Source {
	mu.RLock()
	defer mu.RUnlock()

	sourcesSlice := make([]sources.Source, 0, len(registry))
	for _, source := range registry {
		sourcesSlice = append(sourcesSlice, source)
	}
	return sourcesSlice
}

// HasSource checks if a source type has a registered source
func HasSource(sourceType sources.Type) bool {
	mu.RLock()
	defer mu.RUnlock()
	_, exists := registry[sourceType]
	return exists
}

// RegisteredSourceTypes returns all source types that have registered sources
func RegisteredSourceTypes() []sources.Type {
	mu.RLock()
	defer mu.RUnlock()

	types := make([]sources.Type, 0, len(registry))
	for sourceType := range registry {
		types = append(types, sourceType)
	}

	return types
}
