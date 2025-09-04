package reconciler

import (
	"github.com/agentstation/starmap/pkg/catalogs"
	"github.com/agentstation/starmap/pkg/sources"
)

// filter handles primary source filtering logic
type filter struct {
	primary        sources.Type
	primaryCatalog catalogs.Catalog
}

// newFilter creates a new filter
func newFilter(primary sources.Type, catalog catalogs.Catalog) *filter {
	return &filter{
		primary:        primary,
		primaryCatalog: catalog,
	}
}

// isEnabled returns true if filtering is enabled
func (f *filter) isEnabled() bool {
	return f.primary != "" && f.primaryCatalog != nil
}

// filterProviders filters providers by primary source
func (f *filter) filterProviders(providers []catalogs.Provider) []catalogs.Provider {
	if !f.isEnabled() {
		return providers
	}

	var filtered []catalogs.Provider
	for _, provider := range providers {
		if f.providerExistsInPrimary(provider) {
			filtered = append(filtered, provider)
		}
	}

	return filtered
}

// providerExistsInPrimary checks if provider exists in primary catalog
func (f *filter) providerExistsInPrimary(provider catalogs.Provider) bool {
	if !f.isEnabled() {
		return true
	}

	// Check main ID
	if _, exists := f.primaryCatalog.Providers().Get(provider.ID); exists {
		return true
	}

	// Check aliases
	for _, alias := range provider.Aliases {
		if _, exists := f.primaryCatalog.Providers().Get(alias); exists {
			return true
		}
	}

	return false
}
