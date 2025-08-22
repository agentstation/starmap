package sync

import (
	"fmt"

	"github.com/agentstation/starmap/internal/sources/registry"
	"github.com/agentstation/starmap/pkg/catalogs"
	"github.com/agentstation/starmap/pkg/sources"
)

// Pipeline creates a sync pipeline with registered sources
func Pipeline(catalog catalogs.Catalog, opts ...Option) (*sources.Pipeline, error) {
	cfg := defaultConfig(catalog)

	// Apply options
	for _, opt := range opts {
		opt(cfg)
	}

	// filter out inactive sources
	var active []sources.Source
	for _, source := range registry.Sources() {

		// Clone the source for this pipeline
		clone := source.Clone()

		// Configure with runtime options
		config := sources.SourceConfig{
			Catalog:     catalog,
			Provider:    cfg.Provider,
			SyncOptions: cfg.SyncOptions,
		}

		if err := clone.Configure(config); err != nil {
			// Log error but continue with other sources
			continue
		}

		if clone.IsAvailable() {
			active = append(active, clone)
		}
	}

	if len(active) == 0 {
		return nil, fmt.Errorf("no sources available for pipeline")
	}

	// Create pipeline with configured sources
	pipeline := sources.NewPipeline(active...)

	// Apply custom field authorities if provided
	if cfg.SyncOptions != nil && len(cfg.SyncOptions.CustomFieldAuthorities) > 0 {
		modelAuth := append(sources.DefaultModelFieldAuthorities,
			cfg.SyncOptions.CustomFieldAuthorities...)
		providerAuth := sources.DefaultProviderFieldAuthorities
		pipeline = pipeline.WithCustomAuthorities(modelAuth, providerAuth)
	}

	return pipeline, nil
}
