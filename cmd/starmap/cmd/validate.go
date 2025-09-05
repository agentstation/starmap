package cmd

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/agentstation/starmap"
	"github.com/agentstation/starmap/pkg/catalogs"
	"github.com/agentstation/starmap/pkg/errors"
	"github.com/agentstation/starmap/pkg/sources"
)

// validateCmd represents the validate command.
var validateCmd = &cobra.Command{
	Use:   "validate",
	Short: "Validate provider API key configuration",
	Long: `Validate checks which providers have properly configured API keys
and shows which providers are ready to use.

This command helps you understand:
  - Which providers are configured and ready to use
  - Which providers are missing required API keys
  - Which providers have optional API key configuration
  - Which providers don't have client implementations yet`,
	RunE: runValidate,
}

func init() {
	rootCmd.AddCommand(validateCmd)
}

func runValidate(_ *cobra.Command, _ []string) error {
	sm, err := starmap.New()
	if err != nil {
		return errors.WrapResource("create", "starmap", "", err)
	}

	catalog, err := sm.Catalog()
	if err != nil {
		return errors.WrapResource("get", "catalog", "", err)
	}

	// Get list of supported providers using the public API
	fetcher := sources.NewProviderFetcher()
	supportedProviders := fetcher.List()

	report, err := catalogs.ValidateAllProviders(catalog, supportedProviders)
	if err != nil {
		return &errors.ProcessError{
			Operation: "validate provider access",
			Command:   "validate",
			Err:       err,
		}
	}

	report.Print()

	// Return error if there are missing required keys
	if len(report.Missing) > 0 {
		fmt.Println("\n⚠️  Some providers are missing required API keys")
		return nil // Don't return error, just inform
	}

	if len(report.Configured) > 0 {
		fmt.Printf("\n✨ %d provider(s) ready to use!\n", len(report.Configured))
	}

	return nil
}
