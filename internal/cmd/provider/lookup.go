// Package provider provides common provider operations for CLI commands.
package provider

import (
	"github.com/agentstation/starmap/pkg/catalogs"
	"github.com/agentstation/starmap/pkg/errors"
)

// Get retrieves a provider by ID or alias from the catalog.
// This handles the common pattern of provider lookup with proper error handling.
// Silently resolves aliases to canonical provider IDs.
func Get(catalog catalogs.Catalog, providerID string) (*catalogs.Provider, error) {
	provider, found := catalog.Providers().Resolve(catalogs.ProviderID(providerID))
	if !found {
		return nil, &errors.NotFoundError{
			Resource: "provider",
			ID:       providerID,
		}
	}
	return provider, nil
}

// List returns all providers from the catalog.
// Convenience function for consistent provider listing.
func List(catalog catalogs.Catalog) []*catalogs.Provider {
	providers := catalog.Providers().List()
	result := make([]*catalogs.Provider, len(providers))
	for i, provider := range providers {
		result[i] = &provider
	}
	return result
}

// GetWithValidation retrieves a provider and validates it has a client implementation.
// Used by commands that need to interact with provider APIs.
func GetWithValidation(catalog catalogs.Catalog, providerID string, hasClientFunc func(catalogs.ProviderID) bool) (*catalogs.Provider, error) {
	provider, err := Get(catalog, providerID)
	if err != nil {
		return nil, err
	}

	if !hasClientFunc(provider.ID) {
		return nil, &errors.ValidationError{
			Field:   "provider",
			Value:   providerID,
			Message: "no API client implementation available",
		}
	}

	return provider, nil
}

// FilterWithClients returns only providers that have API client implementations.
// Used by commands that need to work with multiple providers that have clients.
func FilterWithClients(providers []*catalogs.Provider, hasClientFunc func(catalogs.ProviderID) bool) []*catalogs.Provider {
	var validProviders []*catalogs.Provider
	for _, provider := range providers {
		if hasClientFunc(provider.ID) {
			validProviders = append(validProviders, provider)
		}
	}
	return validProviders
}
