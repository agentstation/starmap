// Package list provides commands for listing starmap resources.
package list

import (
	"fmt"
	"os"
	"sort"

	"github.com/spf13/cobra"

	"github.com/agentstation/starmap/internal/cmd/catalog"
	"github.com/agentstation/starmap/internal/cmd/constants"
	"github.com/agentstation/starmap/internal/cmd/filter"
	"github.com/agentstation/starmap/internal/cmd/globals"
	"github.com/agentstation/starmap/internal/cmd/output"
	"github.com/agentstation/starmap/internal/cmd/provider"
	"github.com/agentstation/starmap/internal/cmd/table"
	"github.com/agentstation/starmap/pkg/catalogs"
	"github.com/agentstation/starmap/pkg/errors"
)

// ProvidersCmd represents the list providers subcommand.
var ProvidersCmd = &cobra.Command{
	Use:     "providers [provider-id]",
	Short:   "List providers from catalog",
	Aliases: []string{"provider"},
	Args:    cobra.MaximumNArgs(1),
	Example: `  starmap list providers                   # List all providers
  starmap list providers anthropic         # Show specific provider details
  starmap list providers --keys            # Show API key configuration status`,
	RunE: func(cmd *cobra.Command, args []string) error {
		// Single provider detail view
		if len(args) == 1 {
			showKeys, _ := cmd.Flags().GetBool("keys")
			return showProviderDetails(cmd, args[0], showKeys)
		}

		// List view with filters
		resourceFlags := globals.ParseResources(cmd)
		showKeys, _ := cmd.Flags().GetBool("keys")
		return listProviders(cmd, resourceFlags, showKeys)
	},
}

func init() {
	// Add resource-specific flags
	globals.AddResourceFlags(ProvidersCmd)
	ProvidersCmd.Flags().Bool("keys", false,
		"Show if API keys are configured (keys are not displayed)")
}

// listProviders lists all providers with optional filters.
func listProviders(cmd *cobra.Command, flags *globals.ResourceFlags, showKeys bool) error {
	cat, err := catalog.Load()
	if err != nil {
		return err
	}

	// Get all providers
	providers := cat.Providers().List()

	// Apply filters
	providerFilter := &filter.ProviderFilter{
		Search: flags.Search,
	}
	filtered := providerFilter.Apply(providers)

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
		// Convert to pointer slice for table compatibility
		providerPointers := make([]*catalogs.Provider, len(filtered))
		for i := range filtered {
			providerPointers[i] = &filtered[i]
		}
		tableData := table.ProvidersToTableData(providerPointers, showKeys)
		// Convert to output.Data for formatter compatibility
		outputData = output.Data{
			Headers: tableData.Headers,
			Rows:    tableData.Rows,
		}
	default:
		outputData = filtered
	}

	if !globalFlags.Quiet {
		fmt.Fprintf(os.Stderr, "Found %d providers\n", len(filtered))
	}

	return formatter.Format(os.Stdout, outputData)
}

// showProviderDetails shows detailed information about a specific provider.
func showProviderDetails(cmd *cobra.Command, providerID string, showKeys bool) error {
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

	globalFlags, err := globals.Parse(cmd)
	if err != nil {
		return err
	}
	formatter := output.NewFormatter(output.Format(globalFlags.Output))

	// For table output, show detailed view
	if globalFlags.Output == constants.FormatTable || globalFlags.Output == "" {
		printProviderDetails(prov, showKeys)
		return nil
	}

	// For structured output, return the provider
	return formatter.Format(os.Stdout, prov)
}

// Removed providersToTableData - now using shared table.ProvidersToTableData

// printProviderDetails prints detailed provider information using table format.
func printProviderDetails(provider *catalogs.Provider, showKeys bool) {
	formatter := output.NewFormatter(output.FormatTable)

	// Basic Information Table
	basicRows := [][]string{
		{"Provider ID", string(provider.ID)},
		{"Name", provider.Name},
	}

	if provider.Headquarters != nil {
		basicRows = append(basicRows, []string{"Location", *provider.Headquarters})
	}

	if provider.Catalog != nil && provider.Catalog.Docs != nil {
		basicRows = append(basicRows, []string{"Documentation", *provider.Catalog.Docs})
	}

	if provider.ChatCompletions != nil && provider.ChatCompletions.URL != nil {
		basicRows = append(basicRows, []string{"API Endpoint", *provider.ChatCompletions.URL})
	}

	basicRows = append(basicRows, []string{"Models", fmt.Sprintf("%d", len(provider.Models))})

	basicTable := output.Data{
		Headers: []string{"Property", "Value"},
		Rows:    basicRows,
	}

	fmt.Printf("Provider: %s\n\n", provider.ID)
	fmt.Println("Basic Information:")
	_ = formatter.Format(os.Stdout, basicTable)
	fmt.Println()

	// API Configuration Table
	if provider.APIKey != nil {
		var configRows [][]string

		requirement := "Optional"
		if provider.IsAPIKeyRequired() {
			requirement = "Required"
		}

		configRows = append(configRows, []string{"Key Variable", provider.APIKey.Name})
		configRows = append(configRows, []string{"Requirement", requirement})

		if showKeys {
			status := "✗ Not configured"
			if os.Getenv(provider.APIKey.Name) != "" {
				status = "✅ Configured"
			}
			configRows = append(configRows, []string{"Status", status})
		}

		configTable := output.Data{
			Headers: []string{"Setting", "Value"},
			Rows:    configRows,
		}

		fmt.Println("API Configuration:")
		_ = formatter.Format(os.Stdout, configTable)
		fmt.Println()
	}

	// Environment Variables Table
	if len(provider.EnvVars) > 0 {
		var envRows [][]string

		for _, envVar := range provider.EnvVars {
			requirement := "Optional"
			if envVar.Required {
				requirement = "Required"
			}

			description := envVar.Description
			if description == "" {
				description = "-"
			}

			envRows = append(envRows, []string{envVar.Name, requirement, description})
		}

		envTable := output.Data{
			Headers: []string{"Variable", "Requirement", "Description"},
			Rows:    envRows,
		}

		fmt.Println("Environment Variables:")
		_ = formatter.Format(os.Stdout, envTable)
		fmt.Println()
	}
}
