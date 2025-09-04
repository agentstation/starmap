package catalogs

import (
	"fmt"
)

// ProviderValidationEntry represents the validation status of a single provider
type ProviderValidationEntry struct {
	Provider     *Provider
	HasAPIKey    bool
	IsRequired   bool
	IsConfigured bool
	Error        error
}

// ProviderValidationReport contains the results of validating provider access
type ProviderValidationReport struct {
	Configured  []ProviderValidationEntry // Providers with configured API keys
	Missing     []ProviderValidationEntry // Providers missing required API keys
	Optional    []ProviderValidationEntry // Providers with optional or no API key requirement
	Unsupported []ProviderValidationEntry // Providers without client implementation
}

// ValidateAllProviders checks all providers in the catalog for API key availability.
// This helps users understand which providers they can use based on their configuration.
// The supportedProviders parameter should be a list of provider IDs that have client implementations.
func ValidateAllProviders(catalog Catalog, supportedProviders []ProviderID) (*ProviderValidationReport, error) {
	report := &ProviderValidationReport{
		Configured:  []ProviderValidationEntry{},
		Missing:     []ProviderValidationEntry{},
		Optional:    []ProviderValidationEntry{},
		Unsupported: []ProviderValidationEntry{},
	}

	// Convert list to map for quick lookup
	supportedMap := make(map[ProviderID]bool)
	for _, pid := range supportedProviders {
		supportedMap[pid] = true
	}

	// Check each provider using the Provider.Validate() method
	providersList := catalog.Providers().List()
	for _, provider := range providersList {
		result := provider.Validate(supportedMap)

		// Convert to ProviderValidationEntry format
		entry := ProviderValidationEntry{
			Provider:     provider,
			HasAPIKey:    result.HasAPIKey,
			IsRequired:   result.IsAPIKeyRequired,
			IsConfigured: result.IsConfigured,
			Error:        result.Error,
		}

		// Categorize based on validation result status
		switch result.Status {
		case ProviderValidationStatusConfigured:
			report.Configured = append(report.Configured, entry)
		case ProviderValidationStatusMissing:
			report.Missing = append(report.Missing, entry)
		case ProviderValidationStatusOptional:
			report.Optional = append(report.Optional, entry)
		case ProviderValidationStatusUnsupported:
			report.Unsupported = append(report.Unsupported, entry)
		}
	}

	return report, nil
}

// Print outputs a formatted report of provider validation status
func (r *ProviderValidationReport) Print() {
	if len(r.Configured) > 0 {
		fmt.Println("\n✅ Configured Providers (ready to use):")
		for _, entry := range r.Configured {
			fmt.Printf("  - %s (%s)\n", entry.Provider.Name, entry.Provider.ID)
		}
	}

	if len(r.Missing) > 0 {
		fmt.Println("\n❌ Missing Required API Keys:")
		for _, entry := range r.Missing {
			fmt.Printf("  - %s: Set %s environment variable\n",
				entry.Provider.Name, entry.Provider.APIKey.Name)
			if entry.Error != nil {
				fmt.Printf("    Error: %v\n", entry.Error)
			}
		}
	}

	if len(r.Optional) > 0 {
		fmt.Println("\n⚪ Optional/No API Key Required:")
		for _, entry := range r.Optional {
			fmt.Printf("  - %s", entry.Provider.Name)
			if entry.HasAPIKey && !entry.IsConfigured {
				fmt.Printf(" (optional key %s not set)", entry.Provider.APIKey.Name)
			}
			fmt.Println()
		}
	}

	if len(r.Unsupported) > 0 {
		fmt.Println("\n⚠️ No Client Implementation Yet:")
		for _, entry := range r.Unsupported {
			fmt.Printf("  - %s\n", entry.Provider.Name)
		}
	}
}

// PrintProviderValidationReport prints a formatted report of provider validation status
// This is a convenience function that calls the Print method on the report
func PrintProviderValidationReport(report *ProviderValidationReport) {
	report.Print()
}
