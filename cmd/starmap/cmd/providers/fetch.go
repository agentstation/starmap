package providers

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"sort"
	"sync"
	"time"

	"github.com/spf13/cobra"

	"github.com/agentstation/starmap/internal/application"
	"github.com/agentstation/starmap/internal/cli/emoji"
	"github.com/agentstation/starmap/internal/cli/format"
	"github.com/agentstation/starmap/internal/cli/provider"
	"github.com/agentstation/starmap/internal/cli/table"
	"github.com/agentstation/starmap/pkg/catalogs"
	"github.com/agentstation/starmap/pkg/errors"
	"github.com/agentstation/starmap/pkg/sources"
)

const (
	// outputFormatTable is the default table output format.
	outputFormatTable = "table"
)

// NewFetchCommand creates the fetch subcommand for fetching models from provider APIs.
func NewFetchCommand(app application.Application) *cobra.Command {
	var (
		timeoutFlag int
		rawFlag     bool
		statsFlag   bool
	)

	cmd := &cobra.Command{
		Use:   "fetch [provider]",
		Short: "Fetch models from provider APIs",
		Args:  cobra.MaximumNArgs(1),
		Example: `  starmap providers fetch              # Fetch all providers (table)
  starmap providers fetch openai       # Fetch OpenAI models (table)
  starmap providers fetch openai --stats  # Show detailed request statistics
  starmap providers fetch groq --raw   # Get raw API response from Groq
  starmap providers fetch -o json      # Output as JSON instead of table
  starmap providers fetch --raw --stats # Raw response with statistics
  starmap providers fetch --stats      # All providers with statistics`,
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			logger := app.Logger()
			quiet := logger.GetLevel() > 0 // Use logger level to determine quiet mode

			// No provider specified - fetch all
			if len(args) == 0 {
				return fetchAllProviders(ctx, app, timeoutFlag, quiet, rawFlag, statsFlag)
			}

			// Provider specified as positional arg
			return fetchProviderModels(cmd, app, args[0], timeoutFlag, quiet, rawFlag, statsFlag)
		},
	}

	// Add flags
	cmd.Flags().IntVar(&timeoutFlag, "timeout", 30,
		"Timeout in seconds for API calls")
	cmd.Flags().BoolVar(&rawFlag, "raw", false,
		"Return raw JSON response from provider API")
	cmd.Flags().BoolVar(&statsFlag, "stats", false,
		"Show request statistics (latency, payload size, auth method)")

	return cmd
}

// fetchProviderModels fetches models from a specific provider using app context.
func fetchProviderModels(cmd *cobra.Command, app application.Application, providerID string, timeout int, quiet bool, raw bool, stats bool) error {
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

	// Handle raw response mode
	if raw {
		rawData, fetchStats, err := fetcher.FetchRawResponse(ctx, prov, primarySourceID(prov))
		if err != nil {
			return &errors.SyncError{
				Provider: providerID,
				Err:      err,
			}
		}

		// Display fetch statistics if requested
		if stats && fetchStats != nil {
			displayFetchStats(os.Stderr, prov.ID, fetchStats)
		}

		// Get output format
		outputFormat := app.OutputFormat()
		if outputFormat == "" {
			outputFormat = "json" // Default to JSON for raw mode
		}

		// Format raw response
		formatter := format.NewFormatter(format.Format(outputFormat))

		// Parse JSON to allow re-formatting
		var jsonData any
		if err := json.Unmarshal(rawData, &jsonData); err != nil {
			return fmt.Errorf("failed to parse raw response: %w", err)
		}

		return formatter.Format(os.Stdout, jsonData)
	}

	start := time.Now()
	models, fetchErr := fetcher.FetchModels(ctx, prov)
	if fetchErr != nil {
		return &errors.SyncError{Provider: providerID, Err: fetchErr}
	}
	if stats {
		fmt.Fprintf(os.Stderr, "\n%s Acquisition Statistics:\n", emoji.Info)
		fmt.Fprintf(os.Stderr, "  Provider:     %s\n", prov.ID)
		fmt.Fprintf(os.Stderr, "  Sources:      %d\n", len(prov.Catalog.Sources))
		fmt.Fprintf(os.Stderr, "  Models:       %d\n", len(models))
		fmt.Fprintf(os.Stderr, "  Latency:      %dms\n\n", time.Since(start).Milliseconds())
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

	// Determine output format
	outputFormat := app.OutputFormat()
	if outputFormat == "" {
		outputFormat = outputFormatTable // Default to table
	}

	// Format output
	formatter := format.NewFormatter(format.Format(outputFormat))

	// Transform to output format
	var outputData any
	switch outputFormat {
	case outputFormatTable, "wide", "":
		// Build the pointer slice consumed by the table renderer.
		modelPointers := make([]*catalogs.Model, len(models))
		for i := range models {
			modelPointers[i] = &models[i]
		}
		tableData := table.ModelsToTableData(modelPointers, false)
		// Build the formatter's tabular data contract.
		outputData = format.Data{
			Headers: tableData.Headers,
			Rows:    tableData.Rows,
		}
	default:
		outputData = models
	}

	return formatter.Format(os.Stdout, outputData)
}

// fetchAllProviders fetches models from all configured providers concurrently using app context.
func fetchAllProviders(ctx context.Context, app application.Application, timeout int, quiet bool, raw bool, stats bool) error {
	cat, err := app.Catalog()
	if err != nil {
		return err
	}

	providers := cat.Providers().List()
	fetcher := sources.NewProviderFetcher(cat.Providers())

	// Filter to only providers with clients
	// Build the pointer slice consumed by provider filtering.
	providerPointers := make([]*catalogs.Provider, len(providers))
	for i := range providers {
		providerPointers[i] = &providers[i]
	}
	validProviders := provider.FilterWithClients(providerPointers, fetcher.HasClient)
	if len(validProviders) == 0 {
		return fmt.Errorf("no providers with API clients available")
	}

	// Handle raw response mode
	if raw {
		return fetchAllProvidersRaw(ctx, app, validProviders, fetcher, timeout, quiet, stats)
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
			// Transfer caller-owned model values into the aggregate result.
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
				fmt.Fprintf(os.Stderr, "Warning: %s: %s\n", r.provider, errors.SafeSummary(r.err))
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

	// Determine output format
	outputFormat := app.OutputFormat()
	if outputFormat == "" {
		outputFormat = outputFormatTable // Default to table
	}

	// Format output
	formatter := format.NewFormatter(format.Format(outputFormat))

	// Transform to output format
	var outputData any
	switch outputFormat {
	case outputFormatTable, "wide", "":
		tableData := table.ModelsToTableData(allModels, false)
		// Build the formatter's tabular data contract.
		outputData = format.Data{
			Headers: tableData.Headers,
			Rows:    tableData.Rows,
		}
	default:
		outputData = allModels
	}

	return formatter.Format(os.Stdout, outputData)
}

// fetchAllProvidersRaw fetches raw responses from all configured providers concurrently.
func fetchAllProvidersRaw(ctx context.Context, app application.Application, validProviders []*catalogs.Provider, fetcher *sources.ProviderFetcher, timeout int, quiet bool, stats bool) error {
	type rawResult struct {
		provider string
		rawData  json.RawMessage
		stats    *sources.FetchStats
		err      error
	}

	results := make(chan rawResult, len(validProviders))

	// Concurrent fetching with worker pool
	var wg sync.WaitGroup
	semaphore := make(chan struct{}, 5) // Max 5 concurrent

	for _, prov := range validProviders {
		wg.Add(1)
		go func(p *catalogs.Provider) {
			defer wg.Done()

			semaphore <- struct{}{}
			defer func() { <-semaphore }()

			// Create timeout context for each provider
			fetchCtx, cancel := context.WithTimeout(ctx, time.Duration(timeout)*time.Second)
			defer cancel()

			rawData, stats, err := fetcher.FetchRawResponse(fetchCtx, p, primarySourceID(p))
			results <- rawResult{
				provider: string(p.ID),
				rawData:  json.RawMessage(rawData),
				stats:    stats,
				err:      err,
			}
		}(prov)
	}

	// Wait and close
	go func() {
		wg.Wait()
		close(results)
	}()

	// Collect results into map
	rawResponses := make(map[string]json.RawMessage)
	var successCount, errorCount int

	for r := range results {
		if r.err != nil {
			errorCount++
			if !quiet {
				fmt.Fprintf(os.Stderr, "Warning: %s: %s\n", r.provider, errors.SafeSummary(r.err))
			}
			continue
		}
		successCount++
		rawResponses[r.provider] = r.rawData
		if stats && r.stats != nil {
			fmt.Fprintf(os.Stderr, "%s %s: %s in %dms\n",
				emoji.Success,
				r.provider,
				r.stats.HumanSize(),
				r.stats.Latency.Milliseconds())
		}
	}

	if !quiet {
		fmt.Fprintf(os.Stderr, "\nFetched raw responses from %d providers (%d errors)\n",
			successCount, errorCount)
	}

	// Get output format
	outputFormat := app.OutputFormat()
	if outputFormat == "" {
		outputFormat = "json" // Default to JSON for raw mode
	}

	// Format raw responses
	formatter := format.NewFormatter(format.Format(outputFormat))
	return formatter.Format(os.Stdout, rawResponses)
}

// displayFetchStats displays fetch statistics to the provided writer (typically stderr).
//
//nolint:errcheck // Ignoring write errors for display output
func displayFetchStats(w io.Writer, providerID catalogs.ProviderID, stats *sources.FetchStats) {
	fmt.Fprintf(w, "\n%s Request Statistics:\n", emoji.Info)
	fmt.Fprintf(w, "  Provider:     %s\n", providerID)
	fmt.Fprintf(w, "  Status:       %d\n", stats.StatusCode)
	fmt.Fprintf(w, "  Latency:      %dms\n", stats.Latency.Milliseconds())
	fmt.Fprintf(w, "  Payload:      %s (%d bytes)\n", stats.HumanSize(), stats.PayloadSize)
	fmt.Fprintf(w, "  Content-Type: %s\n", stats.ContentType)

	if stats.AuthMethod != "None" {
		if stats.AuthMethod == "Query" {
			fmt.Fprintf(w, "  Auth:         Query parameter '%s'\n", stats.AuthLocation)
		} else {
			fmt.Fprintf(w, "  Auth:         %s in header '%s'\n", stats.AuthScheme, stats.AuthLocation)
		}
	} else {
		fmt.Fprintf(w, "  Auth:         None\n")
	}
	fmt.Fprintln(w)
}

func primarySourceID(provider *catalogs.Provider) string {
	if provider == nil || provider.Catalog == nil || len(provider.Catalog.Sources) == 0 {
		return ""
	}
	return provider.Catalog.Sources[0].ID
}
