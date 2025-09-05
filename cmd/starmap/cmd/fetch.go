package cmd

import (
	"context"
	"fmt"
	"os"
	"sort"
	"sync"
	"time"

	"github.com/spf13/cobra"

	"github.com/agentstation/starmap"
	"github.com/agentstation/starmap/internal/cmd/cmdutil"
	"github.com/agentstation/starmap/internal/cmd/output"
	"github.com/agentstation/starmap/pkg/catalogs"
	"github.com/agentstation/starmap/pkg/errors"
	"github.com/agentstation/starmap/pkg/sources"
)

const (
	// Output formats.
	tableFormat = "table"
	wideFormat  = "wide"
)

// newFetchCommand creates the fetch command with subcommands.
func newFetchCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "fetch [resource]",
		GroupID: "core",
		Short:   "Retrieve resources from provider APIs",
		Long: `Fetch retrieves live data from provider APIs.

This requires the appropriate API key to be configured either through
environment variables or the configuration file.

Supported providers include: openai, anthropic, google-ai-studio, google-vertex, groq`,
		Example: `  starmap fetch models --provider openai
  starmap fetch models --all`,
		RunE: func(cmd *cobra.Command, args []string) error {
			// Default to models if no subcommand
			if len(args) == 0 {
				return cmd.Help()
			}
			return fmt.Errorf("unknown resource: %s", args[0])
		},
	}

	// Add subcommands for each fetchable resource
	cmd.AddCommand(newFetchModelsCommand())

	return cmd
}

// newFetchModelsCommand creates the fetch models subcommand.
func newFetchModelsCommand() *cobra.Command {
	var fetchFlags *cmdutil.FetchFlags

	cmd := &cobra.Command{
		Use:   "models",
		Short: "Fetch models from provider APIs",
		Example: `  starmap fetch models --provider openai
  starmap fetch models --all
  starmap fetch models -p anthropic --timeout 60`,
		RunE: func(cmd *cobra.Command, _ []string) error {
			ctx := cmd.Context()

			if fetchFlags.All {
				return fetchAllProviders(ctx, fetchFlags.Timeout)
			}

			if fetchFlags.Provider == "" {
				return fmt.Errorf("--provider or --all required")
			}

			return fetchProviderModels(ctx, fetchFlags.Provider, fetchFlags.Timeout)
		},
	}

	// Add fetch-specific flags
	fetchFlags = cmdutil.AddFetchFlags(cmd)

	return cmd
}

// fetchProviderModels fetches models from a specific provider.
func fetchProviderModels(ctx context.Context, providerID string, timeout int) error {
	// Create context with timeout
	ctx, cancel := context.WithTimeout(ctx, time.Duration(timeout)*time.Second)
	defer cancel()

	sm, err := starmap.New()
	if err != nil {
		return errors.WrapResource("create", "starmap", "", err)
	}

	catalog, err := sm.Catalog()
	if err != nil {
		return errors.WrapResource("get", "catalog", "", err)
	}

	provider, found := catalog.Providers().Get(catalogs.ProviderID(providerID))
	if !found {
		return &errors.NotFoundError{
			Resource: "provider",
			ID:       providerID,
		}
	}

	// Use provider fetcher
	fetcher := sources.NewProviderFetcher()
	models, err := fetcher.FetchModels(ctx, provider)
	if err != nil {
		return &errors.SyncError{
			Provider: providerID,
			Err:      err,
		}
	}

	if len(models) == 0 {
		if !globalFlags.Quiet {
			fmt.Fprintf(os.Stderr, "No models returned from %s\n", providerID)
		}
		return nil
	}

	// Sort models by ID
	sort.Slice(models, func(i, j int) bool {
		return models[i].ID < models[j].ID
	})

	if !globalFlags.Quiet {
		fmt.Fprintf(os.Stderr, "Fetched %d models from %s\n", len(models), providerID)
	}

	// Format output
	formatter := output.NewFormatter(output.Format(globalFlags.Output))

	// Transform to output format
	var outputData any
	switch globalFlags.Output {
	case tableFormat, wideFormat, "":
		outputData = modelsToTableData(models, false)
	default:
		outputData = models
	}

	return formatter.Format(os.Stdout, outputData)
}

// fetchAllProviders fetches models from all configured providers concurrently.
func fetchAllProviders(ctx context.Context, timeout int) error {
	sm, err := starmap.New()
	if err != nil {
		return errors.WrapResource("create", "starmap", "", err)
	}

	catalog, err := sm.Catalog()
	if err != nil {
		return errors.WrapResource("get", "catalog", "", err)
	}

	providers := catalog.Providers().List()
	fetcher := sources.NewProviderFetcher()

	// Filter to only providers with clients
	var validProviders []*catalogs.Provider
	for _, provider := range providers {
		if fetcher.HasClient(provider.ID) {
			validProviders = append(validProviders, provider)
		}
	}

	if len(validProviders) == 0 {
		return fmt.Errorf("no providers with API clients available")
	}

	type result struct {
		provider string
		models   []catalogs.Model
		err      error
	}

	results := make(chan result, len(validProviders))

	// Concurrent fetching with worker pool
	var wg sync.WaitGroup
	semaphore := make(chan struct{}, 5) // Max 5 concurrent

	for _, provider := range validProviders {
		wg.Add(1)
		go func(p *catalogs.Provider) {
			defer wg.Done()

			semaphore <- struct{}{}
			defer func() { <-semaphore }()

			// Create timeout context for each provider
			fetchCtx, cancel := context.WithTimeout(ctx, time.Duration(timeout)*time.Second)
			defer cancel()

			models, err := fetcher.FetchModels(fetchCtx, p)
			results <- result{
				provider: string(p.ID),
				models:   models,
				err:      err,
			}
		}(provider)
	}

	// Wait and close
	go func() {
		wg.Wait()
		close(results)
	}()

	// Collect results
	var allModels []catalogs.Model
	var successCount, errorCount int

	for r := range results {
		if r.err != nil {
			errorCount++
			if !globalFlags.Quiet {
				fmt.Fprintf(os.Stderr, "Warning: %s: %v\n", r.provider, r.err)
			}
			continue
		}
		successCount++
		allModels = append(allModels, r.models...)
		if !globalFlags.Quiet {
			fmt.Fprintf(os.Stderr, "✓ %s: %d models\n", r.provider, len(r.models))
		}
	}

	if !globalFlags.Quiet {
		fmt.Fprintf(os.Stderr, "\nFetched %d total models from %d providers (%d errors)\n",
			len(allModels), successCount, errorCount)
	}

	// Sort all models by ID
	sort.Slice(allModels, func(i, j int) bool {
		return allModels[i].ID < allModels[j].ID
	})

	// Format output
	formatter := output.NewFormatter(output.Format(globalFlags.Output))

	// Transform to output format
	var outputData any
	switch globalFlags.Output {
	case tableFormat, wideFormat, "":
		outputData = modelsToTableData(allModels, false)
	default:
		outputData = allModels
	}

	return formatter.Format(os.Stdout, outputData)
}

func init() {
	rootCmd.AddCommand(newFetchCommand())
}
