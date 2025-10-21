// Package providers provides the providers resource command and subcommands.
package providers

import (
	"os"
	"sort"
	"strings"
	"time"

	"github.com/rs/zerolog"
	"github.com/spf13/cobra"

	"github.com/agentstation/starmap/internal/auth"
	"github.com/agentstation/starmap/internal/cmd/application"
	"github.com/agentstation/starmap/internal/cmd/constants"
	"github.com/agentstation/starmap/internal/cmd/format"
	"github.com/agentstation/starmap/internal/cmd/globals"
	"github.com/agentstation/starmap/internal/cmd/table"
	"github.com/agentstation/starmap/pkg/catalogs"
	"github.com/agentstation/starmap/pkg/errors"
	"github.com/agentstation/starmap/pkg/sources"
)

// NewCommand creates the providers resource command.
func NewCommand(app application.Application) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "providers [provider-id]",
		GroupID: "catalog",
		Short:   "Manage AI providers and test credentials",
		Long: `Manage AI providers including authentication status, credential testing, and data fetching.

List providers with authentication details, test credentials by making API calls,
show provider details, and fetch from provider APIs.`,
		Args: cobra.MaximumNArgs(1),
		Example: `  starmap providers                    # List all providers with auth status
  starmap providers --test             # Test all provider credentials
  starmap providers openai             # Show OpenAI provider details
  starmap providers openai --test      # Test OpenAI credentials
  starmap providers fetch              # Fetch from all provider APIs
  starmap providers fetch openai       # Fetch from OpenAI API`,
		RunE: func(cmd *cobra.Command, args []string) error {
			// Check if --test flag is present
			testMode, _ := cmd.Flags().GetBool("test")

			if testMode {
				// Test mode: test provider credentials
				return runTest(cmd, args, app)
			}

			// Normal mode: show table or details
			logger := app.Logger()

			// Single provider detail view
			if len(args) == 1 {
				return showProviderDetails(cmd, app, args[0])
			}

			// List view (default behavior)
			resourceFlags := globals.ParseResources(cmd)
			return listProviders(cmd, app, logger, resourceFlags)
		},
	}

	// Add resource-specific flags
	globals.AddResourceFlags(cmd)

	// Add test-specific flags
	cmd.Flags().Bool("test", false, "Test provider credentials by making API calls")
	cmd.Flags().Duration("timeout", 10*time.Second, "Timeout for API calls when testing")

	// Add subcommands
	cmd.AddCommand(NewFetchCommand(app))

	return cmd
}

// listProviders lists all providers.
func listProviders(cmd *cobra.Command, app application.Application, logger *zerolog.Logger, flags *globals.ResourceFlags) error {
	// Get catalog from app
	cat, err := app.Catalog()
	if err != nil {
		return err
	}

	// Get all providers
	allProviders := cat.Providers().List()

	// Apply search filter if specified
	var filtered []catalogs.Provider
	if flags.Search != "" {
		searchLower := strings.ToLower(flags.Search)
		for _, p := range allProviders {
			if strings.Contains(strings.ToLower(string(p.ID)), searchLower) ||
				strings.Contains(strings.ToLower(p.Name), searchLower) {
				filtered = append(filtered, p)
			}
		}
	} else {
		filtered = allProviders
	}

	// Sort providers
	sort.Slice(filtered, func(i, j int) bool {
		return filtered[i].ID < filtered[j].ID
	})

	// Apply limit
	if flags.Limit > 0 && len(filtered) > flags.Limit {
		filtered = filtered[:flags.Limit]
	}

	// Create auth checker and get supported providers
	checker := auth.NewChecker()
	fetcher := sources.NewProviderFetcher(cat.Providers())
	supportedProviders := fetcher.List()
	supportedMap := make(map[string]bool)
	for _, pid := range supportedProviders {
		supportedMap[string(pid)] = true
	}

	// Get global flags and format output
	globalFlags, err := globals.Parse(cmd)
	if err != nil {
		return err
	}
	formatter := format.NewFormatter(format.Format(globalFlags.Output))

	// Transform to output format
	var outputData any
	switch globalFlags.Output {
	case constants.FormatTable, constants.FormatWide, "":
		providerPointers := make([]*catalogs.Provider, len(filtered))
		for i := range filtered {
			providerPointers[i] = &filtered[i]
		}
		tableData := table.ProvidersToTableData(providerPointers, checker, supportedMap)
		outputData = format.Data{
			Headers:         tableData.Headers,
			Rows:            tableData.Rows,
			ColumnAlignment: tableData.ColumnAlignment,
		}
	default:
		outputData = filtered
	}

	if !globalFlags.Quiet {
		logger.Info().Msgf("Found %d providers", len(filtered))
	}

	return formatter.Format(os.Stdout, outputData)
}

// showProviderDetails shows detailed information about a specific provider.
func showProviderDetails(cmd *cobra.Command, app application.Application, providerID string) error {
	// Get catalog from app
	cat, err := app.Catalog()
	if err != nil {
		return err
	}

	// Find specific provider (supports aliases)
	provider, exists := cat.Providers().Resolve(catalogs.ProviderID(providerID))
	if !exists {
		cmd.SilenceUsage = true
		return &errors.NotFoundError{
			Resource: "provider",
			ID:       providerID,
		}
	}

	globalFlags, err := globals.Parse(cmd)
	if err != nil {
		return err
	}
	formatter := format.NewFormatter(format.Format(globalFlags.Output))

	// For table output, show detailed view
	if globalFlags.Output == constants.FormatTable || globalFlags.Output == "" {
		printProviderDetails(provider)
		return nil
	}

	// For structured output, return the provider
	return formatter.Format(os.Stdout, provider)
}
