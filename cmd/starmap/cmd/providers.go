package cmd

import (
	"fmt"
	"sort"

	"github.com/agentstation/starmap"
	"github.com/agentstation/starmap/pkg/catalogs"
	"github.com/spf13/cobra"
)

var showAPIKeys bool

// providersCmd represents the providers command
var providersCmd = &cobra.Command{
	Use:   "providers",
	Short: "List all available providers",
	Long: `List displays all AI model providers configured in the catalog.
	
For each provider, it shows:
  - Provider ID and display name
  - Location (headquarters)
  - Required API key environment variable
  - Documentation URL
  - Client implementation status`,
	RunE: runProviders,
}

func init() {
	rootCmd.AddCommand(providersCmd)

	providersCmd.Flags().BoolVar(&showAPIKeys, "show-keys", false, "Show if API keys are configured (keys are not displayed)")
}

func runProviders(cmd *cobra.Command, args []string) error {
	// Create starmap instance (now properly loads embedded catalog by default)
	sm, err := starmap.New()
	if err != nil {
		return fmt.Errorf("creating starmap: %w", err)
	}

	// Get catalog from starmap
	catalog, err := sm.Catalog()
	if err != nil {
		return fmt.Errorf("getting catalog: %w", err)
	}

	// Get all providers from the catalog
	providers := catalog.Providers().List()
	if len(providers) == 0 {
		fmt.Println("No providers found in catalog")
		return nil
	}

	// Sort providers by ID
	sort.Slice(providers, func(i, j int) bool {
		return providers[i].ID < providers[j].ID
	})

	// Build supported providers map by checking each provider for client availability
	supportedMap := make(map[catalogs.ProviderID]bool)
	for _, provider := range providers {
		// Try to get client (with missing API key allowed to test if client exists)
		result, _ := provider.GetClient(catalogs.WithAllowMissingAPIKey(true))
		if result != nil && result.Client != nil {
			supportedMap[provider.ID] = true
		}
	}

	fmt.Printf("Found %d providers in catalog:\n\n", len(providers))
	for _, provider := range providers {
		// Use new Provider.Validate() method for comprehensive status
		result := provider.Validate(supportedMap)

		// Status indicator based on validation result
		var status string
		switch result.Status {
		case catalogs.ProviderValidationStatusConfigured:
			status = "✅"
		case catalogs.ProviderValidationStatusOptional:
			status = "⚪"
		case catalogs.ProviderValidationStatusMissing:
			status = "❌"
		case catalogs.ProviderValidationStatusUnsupported:
			status = "⚠️"
		}

		fmt.Printf("%s %s - %s", status, provider.ID, provider.Name)

		// Show validation status if showing keys
		if showAPIKeys {
			switch result.Status {
			case catalogs.ProviderValidationStatusConfigured:
				fmt.Printf(" (ready)")
			case catalogs.ProviderValidationStatusOptional:
				if result.HasAPIKey {
					fmt.Printf(" (optional key not set)")
				} else {
					fmt.Printf(" (no key needed)")
				}
			case catalogs.ProviderValidationStatusMissing:
				fmt.Printf(" (missing API key)")
			case catalogs.ProviderValidationStatusUnsupported:
				fmt.Printf(" (no client)")
			}
		}
		fmt.Println()

		if provider.Headquarters != nil {
			fmt.Printf("   Location: %s\n", *provider.Headquarters)
		}

		if provider.APIKey != nil {
			fmt.Printf("   API Key: %s", provider.APIKey.Name)
			if provider.IsAPIKeyRequired() {
				fmt.Printf(" (required)")
			} else {
				fmt.Printf(" (optional)")
			}
			fmt.Println()
		}

		if provider.Catalog != nil && provider.Catalog.DocsURL != nil {
			fmt.Printf("   Docs: %s\n", *provider.Catalog.DocsURL)
		}

		if provider.ChatCompletions != nil && provider.ChatCompletions.URL != nil {
			fmt.Printf("   API: %s\n", *provider.ChatCompletions.URL)
		}

		fmt.Println()
	}

	fmt.Println("Legend:")
	fmt.Println("  ✅ = Ready to use (client available and API key configured)")
	fmt.Println("  ⚪ = Available (client available, no API key needed or optional)")
	fmt.Println("  ❌ = Missing API key (client available but required API key not set)")
	fmt.Println("  ⚠️  = No client implementation yet")

	return nil
}
