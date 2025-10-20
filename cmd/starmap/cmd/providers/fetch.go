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

	"github.com/agentstation/starmap/internal/cmd/application"
	"github.com/agentstation/starmap/internal/cmd/emoji"
	"github.com/agentstation/starmap/internal/cmd/format"
	"github.com/agentstation/starmap/internal/cmd/provider"
	"github.com/agentstation/starmap/internal/cmd/table"
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
		rawData, fetchStats, err := fetcher.FetchRawResponse(ctx, prov, prov.Catalog.Endpoint.URL)
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

	// Normal mode - fetch models with or without stats
	var models []catalogs.Model

	if stats {
		// Use FetchRawResponse to get stats, then parse models
		rawData, fetchStats, fetchErr := fetcher.FetchRawResponse(ctx, prov, prov.Catalog.Endpoint.URL)
		if fetchErr != nil {
			return &errors.SyncError{
				Provider: providerID,
				Err:      fetchErr,
			}
		}

		// Display stats
		if fetchStats != nil {
			displayFetchStats(os.Stderr, prov.ID, fetchStats)
		}

		// Parse models from raw response
		var parseErr error
		models, parseErr = parseModelsFromRaw(prov, rawData)
		if parseErr != nil {
			return &errors.SyncError{
				Provider: providerID,
				Err:      parseErr,
			}
		}
	} else {
		// Normal fetch without stats
		var fetchErr error
		models, fetchErr = fetcher.FetchModels(ctx, prov)
		if fetchErr != nil {
			return &errors.SyncError{
				Provider: providerID,
				Err:      fetchErr,
			}
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
		// Convert to pointer slice for table compatibility
		modelPointers := make([]*catalogs.Model, len(models))
		for i := range models {
			modelPointers[i] = &models[i]
		}
		tableData := table.ModelsToTableData(modelPointers, false)
		// Convert to format.Data for formatter compatibility
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
	// Convert to pointer slice for compatibility
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
		// Convert to format.Data for formatter compatibility
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

			rawData, stats, err := fetcher.FetchRawResponse(fetchCtx, p, p.Catalog.Endpoint.URL)
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
				fmt.Fprintf(os.Stderr, "Warning: %s: %v\n", r.provider, r.err)
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
	fmt.Fprintf(w, "  URL:          %s\n", stats.URL)
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

// parseModelsFromRaw parses raw JSON response bytes into models using the provider's client.
// This allows us to get both stats and models from a single FetchRawResponse call.
func parseModelsFromRaw(prov *catalogs.Provider, rawData []byte) ([]catalogs.Model, error) {
	// For now, we only support parsing OpenAI-compatible responses
	// This covers: OpenAI, Groq, DeepSeek, Cerebras, Moonshot, etc.
	if prov.Catalog.Endpoint.Type != catalogs.EndpointTypeOpenAI {
		return nil, fmt.Errorf("parsing raw responses only supported for OpenAI-compatible endpoints, got %s", prov.Catalog.Endpoint.Type)
	}

	// Parse raw JSON into generic structure to avoid import cycles
	var response struct {
		Data []json.RawMessage `json:"data"`
	}
	if err := json.Unmarshal(rawData, &response); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	// For each model in the response, parse it generically and extract fields
	models := make([]catalogs.Model, 0, len(response.Data))
	for _, modelData := range response.Data {
		var apiModel struct {
			ID      string `json:"id"`
			Object  string `json:"object"`
			Created int64  `json:"created"`
			OwnedBy string `json:"owned_by"`
		}
		if err := json.Unmarshal(modelData, &apiModel); err != nil {
			continue // Skip invalid models
		}

		// Build basic model
		model := catalogs.Model{
			ID: apiModel.ID,
		}

		// Apply author mapping if configured
		if prov.Catalog.Endpoint.AuthorMapping != nil {
			authorID := catalogs.AuthorID(apiModel.OwnedBy)
			if normalized, ok := prov.Catalog.Endpoint.AuthorMapping.Normalized[apiModel.OwnedBy]; ok {
				authorID = normalized
			}
			model.Authors = []catalogs.Author{{Name: string(authorID)}}
		}

		models = append(models, model)
	}

	return models, nil
}
