package cmd

import (
	"context"
	"fmt"
	"sort"
	"strings"

	"github.com/agentstation/starmap"
	"github.com/agentstation/starmap/pkg/catalogs"
	"github.com/agentstation/starmap/pkg/errors"
	"github.com/agentstation/starmap/pkg/sources"
	"github.com/spf13/cobra"
)

var fetchFlagProvider string

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

	fetchCmd.Flags().StringVarP(&fetchFlagProvider, "provider", "p", "", "Provider to fetch models from (required)")
	if err := fetchCmd.MarkFlagRequired("provider"); err != nil {
		panic(fmt.Sprintf("Failed to mark provider flag as required: %v", err))
	}
}

func runFetch(cmd *cobra.Command, args []string) error {
	if fetchFlagProvider == "" {
		return &errors.ValidationError{
			Field:   "provider",
			Message: "provider flag is required",
		}
	}

	// Convert string to ProviderID
	pid := catalogs.ProviderID(fetchFlagProvider)

	// Get catalog
	sm, err := starmap.New()
	if err != nil {
		return errors.WrapResource("create", "starmap", "", err)
	}

	catalog, err := sm.Catalog()
	if err != nil {
		return errors.WrapResource("get", "catalog", "", err)
	}

	// Get provider from catalog
	provider, found := catalog.Providers().Get(pid)
	if !found {
		return &errors.NotFoundError{
			Resource: "provider",
			ID:       fetchFlagProvider,
		}
	}

	// Create provider fetcher using public API
	fetcher := sources.NewProviderFetcher()

	// Fetch models from API
	ctx := context.Background()
	models, err := fetcher.FetchModels(ctx, provider)
	if err != nil {
		return &errors.SyncError{
			Provider: fetchFlagProvider,
			Err:      err,
		}
	}

	if len(models) == 0 {
		fmt.Printf("No models returned from %s\n", fetchFlagProvider)
		return nil
	}

	// Sort models by ID
	sort.Slice(models, func(i, j int) bool {
		return models[i].ID < models[j].ID
	})

	fmt.Printf("Fetched %d models from %s:\n\n", len(models), fetchFlagProvider)
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
