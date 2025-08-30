package cmd

import (
	"fmt"
	"sort"

	"github.com/agentstation/starmap"
	"github.com/agentstation/starmap/pkg/catalogs"
	"github.com/agentstation/starmap/pkg/errors"
	"github.com/agentstation/starmap/pkg/sources"
	"github.com/spf13/cobra"
)

var providersFlagKeys bool

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

	providersCmd.Flags().BoolVar(&providersFlagKeys, "keys", false, "Show if API keys are configured (keys are not displayed)")
}

func runProviders(cmd *cobra.Command, args []string) error {
	// Create starmap instance (now properly loads embedded catalog by default)
	sm, err := starmap.New()
	if err != nil {
		return errors.WrapResource("create", "starmap", "", err)
	}

	// Get catalog from starmap
	catalog, err := sm.Catalog()
	if err != nil {
		return errors.WrapResource("get", "catalog", "", err)
	}

	// Get all providers from the catalog
	providersList := catalog.Providers().List()
	if len(providersList) == 0 {
		fmt.Println("No providers found in catalog")
		return nil
	}

	// Sort providers by ID
	sort.Slice(providersList, func(i, j int) bool {
		return providersList[i].ID < providersList[j].ID
	})

	// Create provider fetcher to check support
	fetcher := sources.NewProviderFetcher()
	
	// Build supported providers map by checking each provider for client availability
	supportedMap := make(map[catalogs.ProviderID]bool)
	for _, provider := range providersList {
		// Check if provider has a client implementation
		if fetcher.HasClient(provider.ID) {
			supportedMap[provider.ID] = true
		}
	}

	fmt.Printf("Found %d providers in catalog:\n\n", len(providersList))
	for _, provider := range providersList {
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
		if providersFlagKeys {
			switch result.Status {
			case catalogs.ProviderValidationStatusConfigured:
				fmt.Printf(" (ready)")
			case catalogs.ProviderValidationStatusOptional:
				if result.HasAPIKey {
					fmt.Printf(" (optional key configured)")
				} else {
					fmt.Printf(" (no auth needed)")
				}
			case catalogs.ProviderValidationStatusMissing:
				if result.Error != nil {
					fmt.Printf(" (%s)", result.Error.Error())
				} else {
					fmt.Printf(" (missing configuration)")
				}
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

		if len(provider.EnvVars) > 0 {
			fmt.Printf("   Environment Variables:\n")
			for _, envVar := range provider.EnvVars {
				status := "optional"
				if envVar.Required {
					status = "required"
				}
				fmt.Printf("     %s (%s)", envVar.Name, status)
				if envVar.Description != "" {
					fmt.Printf(" - %s", envVar.Description)
				}
				fmt.Println()
			}
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
	fmt.Println("  ✅ = Ready to use (client available and all required configuration set)")
	fmt.Println("  ⚪ = Available (client available, no required configuration or optional auth not set)")
	fmt.Println("  ❌ = Missing configuration (client available but missing required API keys or environment variables)")
	fmt.Println("  ⚠️  = No client implementation yet")

	return nil
}
