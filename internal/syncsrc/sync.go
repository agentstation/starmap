package syncsrc

import (
	"fmt"

	"github.com/agentstation/starmap/internal/sources/registry"
	"github.com/agentstation/starmap/pkg/catalogs"
	"github.com/agentstation/starmap/pkg/sources"
)

// Build creates a source pipeline with registered sources
func Build(catalog catalogs.Catalog, opts ...Option) (*sources.Pipeline, error) {
	cfg := DefaultConfig(catalog)

	// Apply options
	for _, opt := range opts {
		opt(cfg)
	}

	// Get all registered sources
	registeredSources := registry.GetAllSources()

	// Configure each source and collect available ones
	var activeSources []sources.Source

	for _, source := range registeredSources {
		// Clone the source for this pipeline
		sourceClone := source.Clone()

		// Configure with runtime options
		config := sources.SourceConfig{
			Catalog:     catalog,
			Provider:    cfg.Provider,
			SyncOptions: cfg.SyncOptions,
		}

		if err := sourceClone.Configure(config); err != nil {
			// Log error but continue with other sources
			continue
		}

		if sourceClone.IsAvailable() {
			activeSources = append(activeSources, sourceClone)
		}
	}

	if len(activeSources) == 0 {
		return nil, fmt.Errorf("no sources available for pipeline")
	}

	// Create pipeline with configured sources
	pipeline := sources.NewPipeline(activeSources...)

	// Apply custom field authorities if provided
	if cfg.SyncOptions != nil && len(cfg.SyncOptions.CustomFieldAuthorities) > 0 {
		modelAuth := append(sources.DefaultModelFieldAuthorities,
			cfg.SyncOptions.CustomFieldAuthorities...)
		providerAuth := sources.DefaultProviderFieldAuthorities
		pipeline = pipeline.WithCustomAuthorities(modelAuth, providerAuth)
	}

	return pipeline, nil
}
