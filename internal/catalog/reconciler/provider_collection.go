package reconciler

import (
	"github.com/agentstation/starmap/pkg/catalogs"
	"github.com/agentstation/starmap/pkg/sources"
)

type providerViewKey struct {
	source        sources.ID
	observationID string
}

// restrictProviderCollection builds a provider view without changing original evidence.
// Selection uses original provider identities before baseline aliases become canonical IDs.
// Authored definitions and receipts continue to use the original observation.
func (c *collector) restrictProviderCollection(source sources.ID, baseline *catalogs.Catalog) error {
	if source == "" {
		return nil
	}
	c.providerViews = make(map[providerViewKey]*catalogs.Catalog)
	for _, observation := range c.sources {
		if observation.SourceID != source {
			continue
		}
		view := catalogs.NewEmpty()
		if err := setBaselineProviders(view, observation.Catalog, baseline, func(provider catalogs.ProviderID) bool {
			return c.providerSelection.permits(observation, provider)
		}); err != nil {
			return err
		}
		catalog, err := catalogs.NewObservationCatalog(view)
		if err != nil {
			return err
		}
		c.providerViews[providerViewKey{source: source, observationID: observation.ID}] = catalog
	}
	return nil
}

func (c *collector) providerCatalog(observation sources.Observation) *catalogs.Catalog {
	if view, exists := c.providerViews[providerViewKey{source: observation.SourceID, observationID: observation.ID}]; exists {
		return view
	}
	return observation.Catalog
}

func (c *collector) permitsProvider(observation sources.Observation, provider catalogs.ProviderID) bool {
	if _, exists := c.providerViews[providerViewKey{source: observation.SourceID, observationID: observation.ID}]; exists {
		return true
	}
	return c.providerSelection.permits(observation, provider)
}
