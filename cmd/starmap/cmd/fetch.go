package cmd

import (
	"context"
	"fmt"
	"sort"
	"strings"

	"github.com/agentstation/starmap"
	"github.com/agentstation/starmap/pkg/catalogs"
	"github.com/spf13/cobra"
)

var fetchProvider string

// fetchCmd represents the fetch command
var fetchCmd = &cobra.Command{
	Use:   "fetch",
	Short: "Fetch live models from a provider API",
	Long: `Fetch retrieves the current list of available models directly from a
provider's API. This requires the appropriate API key to be configured
either through environment variables or the configuration file.

Supported providers include: openai, anthropic, google-ai-studio, google-vertex, groq`,
	Example: `  starmap fetch --provider openai
  starmap fetch -p anthropic
  starmap fetch --provider groq`,
	RunE: runFetch,
}

func init() {
	rootCmd.AddCommand(fetchCmd)

	fetchCmd.Flags().StringVarP(&fetchProvider, "provider", "p", "", "Provider to fetch models from (required)")
	if err := fetchCmd.MarkFlagRequired("provider"); err != nil {
		panic(fmt.Sprintf("Failed to mark provider flag as required: %v", err))
	}
}

func runFetch(cmd *cobra.Command, args []string) error {
	if fetchProvider == "" {
		return fmt.Errorf("provider flag is required")
	}

	// Convert string to ProviderID
	pid := catalogs.ProviderID(fetchProvider)

	// Get catalog
	sm, err := starmap.New()
	if err != nil {
		return fmt.Errorf("creating starmap: %w", err)
	}

	catalog, err := sm.Catalog()
	if err != nil {
		return fmt.Errorf("getting catalog: %w", err)
	}

	// Get provider from catalog
	provider, found := catalog.Providers().Get(pid)
	if !found {
		return fmt.Errorf("provider %s not found in catalog", fetchProvider)
	}

	// Load API key and environment variables from environment
	provider.LoadAPIKey()
	provider.LoadEnvVars()

	// Get client for provider
	result, err := provider.Client()
	if err != nil {
		return fmt.Errorf("getting client for %s: %w", fetchProvider, err)
	}
	if result.Error != nil {
		return fmt.Errorf("client error for %s: %w", fetchProvider, result.Error)
	}
	client := result.Client

	// Fetch models from API
	ctx := context.Background()
	models, err := client.ListModels(ctx)
	if err != nil {
		return fmt.Errorf("fetching models from %s: %w", fetchProvider, err)
	}

	if len(models) == 0 {
		fmt.Printf("No models returned from %s\n", fetchProvider)
		return nil
	}

	// Sort models by ID
	sort.Slice(models, func(i, j int) bool {
		return models[i].ID < models[j].ID
	})

	fmt.Printf("Fetched %d models from %s:\n\n", len(models), fetchProvider)
	for _, model := range models {
		fmt.Printf("• %s", model.ID)
		if model.Name != "" && model.Name != model.ID {
			fmt.Printf(" - %s", model.Name)
		}
		fmt.Println()

		if model.Limits != nil {
			if model.Limits.ContextWindow > 0 {
				fmt.Printf("  Context: %d tokens", model.Limits.ContextWindow)
				if model.Limits.OutputTokens > 0 {
					fmt.Printf(", Output: %d tokens", model.Limits.OutputTokens)
				}
				fmt.Println()
			}
		}

		if len(model.Authors) > 0 {
			authors := make([]string, 0, len(model.Authors))
			for _, a := range model.Authors {
				if a.Name != "" {
					authors = append(authors, a.Name)
				} else {
					authors = append(authors, string(a.ID))
				}
			}
			fmt.Printf("  Owner: %s\n", strings.Join(authors, ", "))
		}
		fmt.Println()
	}

	return nil
}
