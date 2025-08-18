package cmd

import (
	"fmt"

	"github.com/agentstation/starmap"
	"github.com/agentstation/starmap/internal/validation"
	"github.com/spf13/cobra"
)

// validateCmd represents the validate command
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

func runValidate(cmd *cobra.Command, args []string) error {
	sm, err := starmap.New()
	if err != nil {
		return fmt.Errorf("creating starmap: %w", err)
	}

	catalog, err := sm.Catalog()
	if err != nil {
		return fmt.Errorf("getting catalog: %w", err)
	}

	report, err := validation.ValidateProviderAccess(catalog)
	if err != nil {
		return fmt.Errorf("validating provider access: %w", err)
	}

	validation.PrintProviderReport(report)

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
