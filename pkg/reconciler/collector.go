package reconciler

import (
	"github.com/rs/zerolog"

	"github.com/agentstation/starmap/pkg/catalogs"
	"github.com/agentstation/starmap/pkg/logging"
	"github.com/agentstation/starmap/pkg/sources"
)

// collector encapsulates data collection logic.
type collector struct {
	sources []sources.Observation
	primary sources.ID
	logger  *zerolog.Logger
}

// newCollector creates a new data collector.
func newCollector(srcs []sources.Observation, primary sources.ID) *collector {
	return &collector{
		sources: srcs,
		primary: primary,
		logger:  logging.Default(),
	}
}

// collectProviders gathers providers from all sources.
func (c *collector) collectProviders() map[sources.ID][]*catalogs.Provider {
	result := make(map[sources.ID][]*catalogs.Provider)

	for _, src := range c.sources {
		catalog := src.Catalog
		if catalog == nil {
			continue
		}

		providers := catalog.Providers().List()
		if len(providers) > 0 {
			providerList := make([]*catalogs.Provider, 0, len(providers))
			for _, p := range providers {
				providerList = append(providerList, &p)
			}
			result[src.SourceID] = providerList
		}
	}

	return result
}

// collectModelsForProvider gathers models for a specific provider.
func (c *collector) collectModelsForProvider(
	provider *catalogs.Provider,
	primaryCatalog *catalogs.Catalog,
) map[sources.ID][]*catalogs.Model {
	result := make(map[sources.ID][]*catalogs.Model)

	for _, src := range c.sources {
		models := c.providerModels(src, provider, primaryCatalog)
		if len(models) > 0 {
			result[src.SourceID] = models
		}
	}

	return result
}

// providerModels extracts models for a provider from a source.
func (c *collector) providerModels(src sources.Observation, provider *catalogs.Provider, primaryCatalog *catalogs.Catalog) []*catalogs.Model {
	catalog := src.Catalog
	if catalog == nil {
		return nil
	}

	sourceName := src.SourceID

	// Find provider in source (check ID and aliases)
	sourceProvider := c.findProvider(catalog, provider.ID, provider.Aliases)

	var models []*catalogs.Model

	// Get models directly from provider
	if sourceProvider != nil && sourceProvider.Models != nil {
		for _, model := range sourceProvider.Models {
			models = append(models, model)
		}

		c.logger.Debug().
			Str("source", string(sourceName)).
			Str("provider", string(provider.ID)).
			Int("models_in_provider", len(models)).
			Msg("Found models in provider")
	}

	// For enrichment sources, add models that primary serves
	if c.isPrimaryFiltering() && sourceName != c.primary {
		models = c.enrichWithPrimaryModels(
			provider,
			models,
			primaryCatalog,
		)
	}

	return models
}

// findProvider looks up provider by ID or aliases.
func (c *collector) findProvider(catalog catalogs.Reader, id catalogs.ProviderID, aliases []catalogs.ProviderID) *catalogs.Provider {
	if provider, exists := catalog.Providers().Get(id); exists {
		return provider
	}

	for _, alias := range aliases {
		if provider, exists := catalog.Providers().Get(alias); exists {
			return provider
		}
	}

	return nil
}

// isPrimaryFiltering returns true if primary source filtering is enabled.
func (c *collector) isPrimaryFiltering() bool {
	return c.primary != ""
}

// enrichWithPrimaryModels adds models that primary source serves.
func (c *collector) enrichWithPrimaryModels(
	provider *catalogs.Provider,
	existingModels []*catalogs.Model,
	primaryCatalog *catalogs.Catalog,
) []*catalogs.Model {
	if primaryCatalog == nil {
		return existingModels
	}

	// Find primary provider
	primaryProvider := c.findProvider(primaryCatalog, provider.ID, provider.Aliases)
	if primaryProvider == nil || primaryProvider.Models == nil {
		return nil
	}

	primaryModelIDs := make(map[string]struct{}, len(primaryProvider.Models))
	for id := range primaryProvider.Models {
		primaryModelIDs[id] = struct{}{}
	}
	included := make(map[string]struct{}, len(existingModels))
	filtered := make([]*catalogs.Model, 0, len(existingModels))

	c.logger.Debug().
		Str("source", "enrichment").
		Str("provider", string(provider.ID)).
		Int("provider_models", len(existingModels)).
		Msg("Filtering non-primary source models by primary authority")

	for _, model := range existingModels {
		if model == nil {
			continue
		}
		if _, ok := primaryModelIDs[model.ID]; !ok {
			continue
		}
		if _, ok := included[model.ID]; ok {
			continue
		}
		included[model.ID] = struct{}{}
		filtered = append(filtered, model)
	}

	return filtered
}

// sourceTypes extracts source types from sources.
func (c *collector) sourceTypes() []sources.ID {
	types := make([]sources.ID, 0, len(c.sources))
	for _, src := range c.sources {
		types = append(types, src.SourceID)
	}
	return types
}

// primaryCatalog finds and returns the primary catalog.
func (c *collector) primaryCatalog() *catalogs.Catalog {
	if c.primary == "" {
		return nil
	}

	for _, src := range c.sources {
		if src.SourceID == c.primary {
			return src.Catalog
		}
	}

	return nil
}

// baseCatalog returns the first available catalog for comparison.
func (c *collector) baseCatalog() *catalogs.Catalog {
	for _, src := range c.sources {
		if catalog := src.Catalog; catalog != nil {
			return catalog
		}
	}
	return nil
}
