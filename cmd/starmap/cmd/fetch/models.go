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

	"github.com/agentstation/starmap/internal/cmd/catalog"
	"github.com/agentstation/starmap/internal/cmd/cmdutil"
	"github.com/agentstation/starmap/internal/cmd/output"
	"github.com/agentstation/starmap/internal/cmd/provider"
	"github.com/agentstation/starmap/internal/cmd/table"
	"github.com/agentstation/starmap/pkg/catalogs"
	"github.com/agentstation/starmap/pkg/errors"
	"github.com/agentstation/starmap/pkg/sources"
)

func init() {
	// Add fetch-specific flags
	ModelsCmd.Flags().StringVarP(nil, "provider", "p", "",
		"Provider to fetch from")
	ModelsCmd.Flags().BoolVar(nil, "all", false,
		"Fetch from all configured providers")
	ModelsCmd.Flags().IntVar(nil, "timeout", 30,
		"Timeout in seconds for API calls")
}

// ModelsCmd represents the fetch models subcommand.
var ModelsCmd = &cobra.Command{
	Use:   "models",
	Short: "Fetch models from provider APIs",
	Example: `  starmap fetch models --provider openai
  starmap fetch models --all
  starmap fetch models -p anthropic --timeout 60`,
	RunE: func(cmd *cobra.Command, _ []string) error {
		ctx := cmd.Context()

		// Extract flags
		flags := getFetchFlags(cmd)

		if flags.All {
			return fetchAllProviders(ctx, flags.Timeout)
		}

		if flags.Provider == "" {
			return fmt.Errorf("--provider or --all required")
		}

		return fetchProviderModels(cmd, flags.Provider, flags.Timeout)
	},
}

// Flags holds flags for fetch command.
type Flags struct {
	Provider string
	All      bool
	Timeout  int
}

// getFetchFlags extracts fetch flags from a command.
func getFetchFlags(cmd *cobra.Command) *Flags {
	provider, _ := cmd.Flags().GetString("provider")
	all, _ := cmd.Flags().GetBool("all")
	timeout, _ := cmd.Flags().GetInt("timeout")

	return &Flags{
		Provider: provider,
		All:      all,
		Timeout:  timeout,
	}
}

// getGlobalFlags returns the global flags.
func getGlobalFlags() *cmdutil.GlobalFlags {
	// Return defaults for now - this will be passed from the calling commands
	return &cmdutil.GlobalFlags{
		Output: "",
		Quiet:  false,
	}
}

// fetchProviderModels fetches models from a specific provider.
func fetchProviderModels(cmd *cobra.Command, providerID string, timeout int) error {
	// Get context from command
	ctx := cmd.Context()
	// Create context with timeout
	ctx, cancel := context.WithTimeout(ctx, time.Duration(timeout)*time.Second)
	defer cancel()

	cat, err := catalog.Load()
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
	fetcher := sources.NewProviderFetcher()
	models, err := fetcher.FetchModels(ctx, prov)
	if err != nil {
		return &errors.SyncError{
			Provider: providerID,
			Err:      err,
		}
	}

	if len(models) == 0 {
		globalFlags := getGlobalFlags()
		if !globalFlags.Quiet {
			fmt.Fprintf(os.Stderr, "No models returned from %s\n", providerID)
		}
		return nil
	}

	// Sort models by ID
	sort.Slice(models, func(i, j int) bool {
		return models[i].ID < models[j].ID
	})

	globalFlags := getGlobalFlags()
	if !globalFlags.Quiet {
		fmt.Fprintf(os.Stderr, "Fetched %d models from %s\n", len(models), providerID)
	}

	// Format output
	formatter := output.NewFormatter(output.Format(globalFlags.Output))

	// Transform to output format
	var outputData any
	switch globalFlags.Output {
	case "table", "wide", "":
		tableData := table.ModelsToTableData(models, false)
		// Convert to output.TableData for formatter compatibility
		outputData = output.TableData{
			Headers: tableData.Headers,
			Rows:    tableData.Rows,
		}
	default:
		outputData = models
	}

	return formatter.Format(os.Stdout, outputData)
}

// fetchAllProviders fetches models from all configured providers concurrently.
func fetchAllProviders(ctx context.Context, timeout int) error {
	cat, err := catalog.Load()
	if err != nil {
		return err
	}

	providers := cat.Providers().List()
	fetcher := sources.NewProviderFetcher()

	// Filter to only providers with clients
	validProviders := provider.FilterWithClients(providers, fetcher.HasClient)
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

	globalFlags := getGlobalFlags()
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
	case "table", "wide", "":
		tableData := table.ModelsToTableData(allModels, false)
		// Convert to output.TableData for formatter compatibility
		outputData = output.TableData{
			Headers: tableData.Headers,
			Rows:    tableData.Rows,
		}
	default:
		outputData = allModels
	}

	return formatter.Format(os.Stdout, outputData)
}
