package list

import (
	"fmt"
	"os"
	"sort"
	"strings"

	"github.com/rs/zerolog"
	"github.com/spf13/cobra"

	"github.com/agentstation/starmap/cmd/application"
	"github.com/agentstation/starmap/internal/cmd/constants"
	"github.com/agentstation/starmap/internal/cmd/globals"
	"github.com/agentstation/starmap/internal/cmd/output"
	"github.com/agentstation/starmap/internal/cmd/table"
	"github.com/agentstation/starmap/pkg/catalogs"
	"github.com/agentstation/starmap/pkg/errors"
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
				return showProviderDetails(cmd, app, logger, args[0])
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

	// Get global flags and format output
	globalFlags, err := globals.Parse(cmd)
	if err != nil {
		return err
	}
	formatter := output.NewFormatter(output.Format(globalFlags.Output))

	// Transform to output format
	var outputData any
	switch globalFlags.Output {
	case constants.FormatTable, constants.FormatWide, "":
		providerPointers := make([]*catalogs.Provider, len(filtered))
		for i := range filtered {
			providerPointers[i] = &filtered[i]
		}
		tableData := table.ProvidersToTableData(providerPointers, false)
		outputData = output.Data{
			Headers: tableData.Headers,
			Rows:    tableData.Rows,
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
func showProviderDetails(cmd *cobra.Command, app application.Application, logger *zerolog.Logger, providerID string) error {
	// Get catalog from app
	cat, err := app.Catalog()
	if err != nil {
		return err
	}

	// Find specific provider
	provider, exists := cat.Providers().Get(catalogs.ProviderID(providerID))
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
	formatter := output.NewFormatter(output.Format(globalFlags.Output))

	// For table output, show detailed view
	if globalFlags.Output == constants.FormatTable || globalFlags.Output == "" {
		printProviderDetails(provider)
		return nil
	}

	// For structured output, return the provider
	return formatter.Format(os.Stdout, provider)
}

// printProviderDetails prints detailed provider information.
func printProviderDetails(provider *catalogs.Provider) {
	formatter := output.NewFormatter(output.FormatTable)

	fmt.Printf("Provider: %s\n\n", provider.ID)

	// Basic info
	basicRows := [][]string{
		{"Provider ID", string(provider.ID)},
		{"Name", provider.Name},
	}

	if provider.Headquarters != nil && *provider.Headquarters != "" {
		basicRows = append(basicRows, []string{"Headquarters", *provider.Headquarters})
	}

	if provider.StatusPageURL != nil && *provider.StatusPageURL != "" {
		basicRows = append(basicRows, []string{"Status Page", *provider.StatusPageURL})
	}

	basicTable := output.Data{
		Headers: []string{"Property", "Value"},
		Rows:    basicRows,
	}

	fmt.Println("Basic Information:")
	_ = formatter.Format(os.Stdout, basicTable)
	fmt.Println()

	// Model count
	fmt.Printf("Models: %d\n", len(provider.Models))
}
