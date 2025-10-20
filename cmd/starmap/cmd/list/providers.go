package list

import (
	"os"
	"sort"
	"strings"

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

// NewProvidersCommand creates the list providers subcommand using app context.
func NewProvidersCommand(app application.Application) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "providers [provider-id]",
		Short:   "List providers from catalog",
		Aliases: []string{"provider"},
		Args:    cobra.MaximumNArgs(1),
		Example: `  starmap list providers                    # List all providers
  starmap list providers openai             # Show specific provider details
  starmap list providers --search anthropic # Search providers by name`,
		RunE: func(cmd *cobra.Command, args []string) error {
			logger := app.Logger()

			// Single provider detail view
			if len(args) == 1 {
				return showProviderDetails(cmd, app, args[0])
			}

			// List view
			resourceFlags := globals.ParseResources(cmd)
			return listProviders(cmd, app, logger, resourceFlags)
		},
	}

	// Add resource-specific flags
	globals.AddResourceFlags(cmd)

	return cmd
}

// listProviders lists all providers using app context.
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
	formatter := format.NewFormatter(format.Format(globalFlags.Format))

	// Transform to output format
	var outputData any
	switch globalFlags.Format {
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
	formatter := format.NewFormatter(format.Format(globalFlags.Format))

	// For table output, show detailed view
	if globalFlags.Format == constants.FormatTable || globalFlags.Format == "" {
		printProviderDetails(provider)
		return nil
	}

	// For structured output, return the provider
	return formatter.Format(os.Stdout, provider)
}
