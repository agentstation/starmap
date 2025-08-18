package validation

import (
	"fmt"

	"github.com/agentstation/starmap/internal/sources/providers"
	"github.com/agentstation/starmap/pkg/catalogs"
)

// ProviderValidationStatus represents the status of a provider's API key configuration
type ProviderValidationStatus struct {
	Provider     *catalogs.Provider
	HasAPIKey    bool
	IsRequired   bool
	IsConfigured bool
	Error        error
}

// ProviderValidationReport contains the results of validating provider access
type ProviderValidationReport struct {
	Configured  []ProviderValidationStatus // Providers with configured API keys
	Missing     []ProviderValidationStatus // Providers missing required API keys
	Optional    []ProviderValidationStatus // Providers with optional or no API key requirement
	Unsupported []ProviderValidationStatus // Providers without client implementation
}

// ValidateProviderAccess checks all providers in the catalog for API key availability.
// This helps users understand which providers they can use based on their configuration.
func ValidateProviderAccess(catalog catalogs.Catalog) (*ProviderValidationReport, error) {

	report := &ProviderValidationReport{
		Configured:  []ProviderValidationStatus{},
		Missing:     []ProviderValidationStatus{},
		Optional:    []ProviderValidationStatus{},
		Unsupported: []ProviderValidationStatus{},
	}

	// Get list of supported providers (ones with client implementations)
	supportedProviders := make(map[catalogs.ProviderID]bool)
	for _, pid := range providers.ListSupportedProviders() {
		supportedProviders[pid] = true
	}

	// Check each provider using the new Provider.Validate() method
	providers := catalog.Providers().List()
	for _, provider := range providers {
		result := provider.Validate(supportedProviders)

		// Convert to legacy ProviderValidationStatus format
		status := ProviderValidationStatus{
			Provider:     provider,
			HasAPIKey:    result.HasAPIKey,
			IsRequired:   result.IsRequired,
			IsConfigured: result.IsConfigured,
			Error:        result.Error,
		}

		// Categorize based on validation result status
		switch result.Status {
		case catalogs.ProviderValidationStatusConfigured:
			report.Configured = append(report.Configured, status)
		case catalogs.ProviderValidationStatusMissing:
			report.Missing = append(report.Missing, status)
		case catalogs.ProviderValidationStatusOptional:
			report.Optional = append(report.Optional, status)
		case catalogs.ProviderValidationStatusUnsupported:
			report.Unsupported = append(report.Unsupported, status)
		}
	}

	return report, nil
}

// PrintProviderReport prints a formatted report of provider validation status
func PrintProviderReport(report *ProviderValidationReport) {
	if len(report.Configured) > 0 {
		fmt.Println("\n✅ Configured Providers (ready to use):")
		for _, status := range report.Configured {
			fmt.Printf("  - %s (%s)\n", status.Provider.Name, status.Provider.ID)
		}
	}

	if len(report.Missing) > 0 {
		fmt.Println("\n❌ Missing Required API Keys:")
		for _, status := range report.Missing {
			fmt.Printf("  - %s: Set %s environment variable\n",
				status.Provider.Name, status.Provider.APIKey.Name)
			if status.Error != nil {
				fmt.Printf("    Error: %v\n", status.Error)
			}
		}
	}

	if len(report.Optional) > 0 {
		fmt.Println("\n⚪ Optional/No API Key Required:")
		for _, status := range report.Optional {
			fmt.Printf("  - %s", status.Provider.Name)
			if status.HasAPIKey && !status.IsConfigured {
				fmt.Printf(" (optional key %s not set)", status.Provider.APIKey.Name)
			}
			fmt.Println()
		}
	}

	if len(report.Unsupported) > 0 {
		fmt.Println("\n⚠️ No Client Implementation Yet:")
		for _, status := range report.Unsupported {
			fmt.Printf("  - %s\n", status.Provider.Name)
		}
	}
}
