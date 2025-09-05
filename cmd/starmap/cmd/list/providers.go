// Package list provides commands for listing starmap resources.
package list

import (
	"fmt"
	"os"
	"sort"

	"github.com/spf13/cobra"

	"github.com/agentstation/starmap/internal/cmd/catalog"
	"github.com/agentstation/starmap/internal/cmd/cmdutil"
	"github.com/agentstation/starmap/internal/cmd/constants"
	"github.com/agentstation/starmap/internal/cmd/filter"
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
		resourceFlags := getResourceFlags(cmd)
		showKeys, _ := cmd.Flags().GetBool("keys")
		return listProviders(resourceFlags, showKeys)
	},
}

func init() {
	// Add resource-specific flags
	cmdutil.AddResourceFlags(ProvidersCmd)
	ProvidersCmd.Flags().Bool("keys", false,
		"Show if API keys are configured (keys are not displayed)")
}

// listProviders lists all providers with optional filters.
func listProviders(flags *cmdutil.ResourceFlags, showKeys bool) error {
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
	// Convert to value slice for filter
	providerValues := make([]catalogs.Provider, len(providers))
	for i, p := range providers {
		providerValues[i] = *p
	}
	filtered := providerFilter.Apply(providerValues)

	// Sort providers
	sort.Slice(filtered, func(i, j int) bool {
		return filtered[i].ID < filtered[j].ID
	})

	// Apply limit
	if flags.Limit > 0 && len(filtered) > flags.Limit {
		filtered = filtered[:flags.Limit]
	}

	// Format output
	globalFlags := getGlobalFlags()
	formatter := output.NewFormatter(output.Format(globalFlags.Output))

	// Transform to output format
	var outputData any
	switch globalFlags.Output {
	case constants.FormatTable, constants.FormatWide, "":
		tableData := table.ProvidersToTableData(filtered, showKeys)
		// Convert to output.TableData for formatter compatibility
		outputData = output.TableData{
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

	globalFlags := getGlobalFlags()
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

// printProviderDetails prints detailed provider information in a human-readable format.
func printProviderDetails(provider *catalogs.Provider, showKeys bool) {
	fmt.Printf("Provider: %s\n", provider.ID)
	fmt.Printf("Name: %s\n", provider.Name)

	if provider.Headquarters != nil {
		fmt.Printf("Location: %s\n", *provider.Headquarters)
	}

	if provider.APIKey != nil {
		fmt.Printf("\nAPI Configuration:\n")
		fmt.Printf("  Key Variable: %s", provider.APIKey.Name)
		if provider.IsAPIKeyRequired() {
			fmt.Printf(" (required)")
		} else {
			fmt.Printf(" (optional)")
		}
		fmt.Println()

		if showKeys {
			if os.Getenv(provider.APIKey.Name) != "" {
				fmt.Printf("  Status: ✓ Configured\n")
			} else {
				fmt.Printf("  Status: ✗ Not configured\n")
			}
		}
	}

	if len(provider.EnvVars) > 0 {
		fmt.Printf("\nEnvironment Variables:\n")
		for _, envVar := range provider.EnvVars {
			status := "optional"
			if envVar.Required {
				status = "required"
			}
			fmt.Printf("  %s (%s)", envVar.Name, status)
			if envVar.Description != "" {
				fmt.Printf(" - %s", envVar.Description)
			}
			fmt.Println()
		}
	}

	if provider.Catalog != nil && provider.Catalog.DocsURL != nil {
		fmt.Printf("\nDocumentation: %s\n", *provider.Catalog.DocsURL)
	}

	if provider.ChatCompletions != nil && provider.ChatCompletions.URL != nil {
		fmt.Printf("API Endpoint: %s\n", *provider.ChatCompletions.URL)
	}

	fmt.Printf("\nModels: %d\n", len(provider.Models))
}
