// Package fetch provides commands for fetching starmap resources from provider APIs.
package fetch

import (
	"context"
	"fmt"
	"os"
	"sort"
	"sync"
	"time"

	"github.com/spf13/cobra"

	"github.com/agentstation/starmap/cmd/application"
	"github.com/agentstation/starmap/internal/cmd/output"
	"github.com/agentstation/starmap/internal/cmd/provider"
	"github.com/agentstation/starmap/internal/cmd/table"
	"github.com/agentstation/starmap/pkg/catalogs"
	"github.com/agentstation/starmap/pkg/errors"
	"github.com/agentstation/starmap/pkg/sources"
	"github.com/agentstation/starmap/internal/cmd/emoji"
)

const (
	// outputFormatTable is the default table output format.
	outputFormatTable = "table"
)

// NewModelsCommand creates the fetch models subcommand using app context.
func NewModelsCommand(app application.Application) *cobra.Command {
	var (
		providerFlag string
		allFlag      bool
		timeoutFlag  int
	)

	cmd := &cobra.Command{
		Use:   "models",
		Short: "Fetch models from provider APIs",
		Example: `  starmap fetch models --provider openai
  starmap fetch models --all
  starmap fetch models -p anthropic --timeout 60`,
		RunE: func(cmd *cobra.Command, _ []string) error {
			ctx := cmd.Context()
			logger := app.Logger()
			quiet := logger.GetLevel() > 0 // Use logger level to determine quiet mode

			if allFlag {
				return fetchAllProviders(ctx, app, timeoutFlag, quiet)
			}

			if providerFlag == "" {
				return fmt.Errorf("--provider or --all required")
			}

			return fetchProviderModels(cmd, app, providerFlag, timeoutFlag, quiet)
		},
	}

	// Add flags
	cmd.Flags().StringVarP(&providerFlag, "provider", "p", "",
		"Provider to fetch from")
	cmd.Flags().BoolVar(&allFlag, "all", false,
		"Fetch from all configured providers")
	cmd.Flags().IntVar(&timeoutFlag, "timeout", 30,
		"Timeout in seconds for API calls")

	return cmd
}

// fetchProviderModels fetches models from a specific provider using app context.
func fetchProviderModels(cmd *cobra.Command, app application.Application, providerID string, timeout int, quiet bool) error {
	// Get context from command
	ctx := cmd.Context()
	// Create context with timeout
	ctx, cancel := context.WithTimeout(ctx, time.Duration(timeout)*time.Second)
	defer cancel()

	cat, err := app.Catalog()
	if err != nil {
		return err
	}

	prov, err := provider.Get(cat, providerID)
	if err != nil {
		// Suppress usage display for not found errors
		if errors.IsNotFound(err) {
			cmd.SilenceUsage = true
		}
		return err
	}

	// Use provider fetcher
	fetcher := sources.NewProviderFetcher(cat.Providers())
	models, err := fetcher.FetchModels(ctx, prov)
	if err != nil {
		return &errors.SyncError{
			Provider: providerID,
			Err:      err,
		}
	}

	if len(models) == 0 {
		if !quiet {
			fmt.Fprintf(os.Stderr, "No models returned from %s\n", providerID)
		}
		return nil
	}

	// Sort models by ID
	sort.Slice(models, func(i, j int) bool {
		return models[i].ID < models[j].ID
	})

	if !quiet {
		fmt.Fprintf(os.Stderr, "Fetched %d models from %s\n", len(models), providerID)
	}

	// Determine output format from logger
	logger := app.Logger()
	outputFormat := outputFormatTable
	// Could be extended to read from config if needed

	// Format output
	formatter := output.NewFormatter(output.Format(outputFormat))

	// Transform to output format
	var outputData any
	switch outputFormat {
	case outputFormatTable, "wide", "":
		// Convert to pointer slice for table compatibility
		modelPointers := make([]*catalogs.Model, len(models))
		for i := range models {
			modelPointers[i] = &models[i]
		}
		tableData := table.ModelsToTableData(modelPointers, false)
		// Convert to output.Data for formatter compatibility
		outputData = output.Data{
			Headers: tableData.Headers,
			Rows:    tableData.Rows,
		}
	default:
		outputData = models
	}

	_ = logger // Silence unused warning
	return formatter.Format(os.Stdout, outputData)
}

// fetchAllProviders fetches models from all configured providers concurrently using app context.
func fetchAllProviders(ctx context.Context, app application.Application, timeout int, quiet bool) error {
	cat, err := app.Catalog()
	if err != nil {
		return err
	}

	providers := cat.Providers().List()
	fetcher := sources.NewProviderFetcher(cat.Providers())

	// Filter to only providers with clients
	// Convert to pointer slice for compatibility
	providerPointers := make([]*catalogs.Provider, len(providers))
	for i := range providers {
		providerPointers[i] = &providers[i]
	}
	validProviders := provider.FilterWithClients(providerPointers, fetcher.HasClient)
	if len(validProviders) == 0 {
		return fmt.Errorf("no providers with API clients available")
	}

	type result struct {
		provider string
		models   []*catalogs.Model
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
			// Convert to pointer slice for result struct compatibility
			modelPointers := make([]*catalogs.Model, len(models))
			for i := range models {
				modelPointers[i] = &models[i]
			}
			results <- result{
				provider: string(p.ID),
				models:   modelPointers,
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
	var allModels []*catalogs.Model
	var successCount, errorCount int

	for r := range results {
		if r.err != nil {
			errorCount++
			if !quiet {
				fmt.Fprintf(os.Stderr, "Warning: %s: %v\n", r.provider, r.err)
			}
			continue
		}
		successCount++
		allModels = append(allModels, r.models...)
		if !quiet {
			fmt.Fprintf(os.Stderr, "%s %s: %d models\n", emoji.Success, r.provider, len(r.models))
		}
	}

	if !quiet {
		fmt.Fprintf(os.Stderr, "\nFetched %d total models from %d providers (%d errors)\n",
			len(allModels), successCount, errorCount)
	}

	// Sort all models by ID
	sort.Slice(allModels, func(i, j int) bool {
		return allModels[i].ID < allModels[j].ID
	})

	// Determine output format from logger
	logger := app.Logger()
	outputFormat := outputFormatTable
	// Could be extended to read from config if needed

	// Format output
	formatter := output.NewFormatter(output.Format(outputFormat))

	// Transform to output format
	var outputData any
	switch outputFormat {
	case outputFormatTable, "wide", "":
		tableData := table.ModelsToTableData(allModels, false)
		// Convert to output.Data for formatter compatibility
		outputData = output.Data{
			Headers: tableData.Headers,
			Rows:    tableData.Rows,
		}
	default:
		outputData = allModels
	}

	_ = logger // Silence unused warning
	return formatter.Format(os.Stdout, outputData)
}
