package modelsdev

import (
	"github.com/agentstation/starmap/pkg/catalogs"
	"github.com/agentstation/starmap/pkg/errors"
)

// processFetch handles the common logic for fetching models from models.dev API.
func processFetch(catalog catalogs.Catalog, api *API) (int, error) {

	// Set the default merge strategy for models.dev catalog (enhances with pricing/limits)
	catalog.SetMergeStrategy(catalogs.MergeEnrichEmpty)

	// Add providers with their models that have pricing/limits data from models.dev
	added := 0
	for _, mdProvider := range *api {
		// Convert provider ID from models.dev format
		providerID := catalogs.ProviderID(mdProvider.ID)

		// Get or create provider in catalog
		provider, err := catalog.Provider(providerID)
		if err != nil {
			// Provider doesn't exist, create a minimal one
			provider = catalogs.Provider{
				ID:   providerID,
				Name: mdProvider.ID, // Use ID as name for now
			}
		}

		// Initialize models map if needed
		if provider.Models == nil {
			provider.Models = make(map[string]*catalogs.Model)
		}

		// Add models with pricing/limits data
		for _, mdModel := range mdProvider.Models {
			// Only include models that have pricing or limits data
			if (mdModel.Cost != nil && (mdModel.Cost.Input != nil || mdModel.Cost.Output != nil)) ||
				mdModel.Limit.Context > 0 || mdModel.Limit.Output > 0 {
				// Convert to starmap model with pricing/limits
				model := ConvertToStarmapModel(mdModel)
				provider.Models[model.ID] = model
				added++
			}
		}

		// Update provider in catalog if we added any models
		if len(provider.Models) > 0 {
			if err := catalog.SetProvider(provider); err != nil {
				return added, errors.WrapResource("set", "provider", string(provider.ID), err)
			}
		}
	}

	return added, nil
}
