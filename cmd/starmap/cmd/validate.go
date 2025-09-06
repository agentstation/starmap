package cmd

import (
	"github.com/spf13/cobra"

	validatecmd "github.com/agentstation/starmap/cmd/starmap/cmd/validate"
)

// validateCmd represents the validate command.
var validateCmd = &cobra.Command{
	Use:   "validate [resource]",
	Short: "Validate catalog configuration and structure",
	Long: `Validate the structure and completeness of catalog configuration files.

Without arguments, validates the entire embedded catalog.
Use subcommands to validate specific resources:
  - catalog: Validate entire catalog (default)
  - providers: Validate providers.yaml structure
  - authors: Validate authors.yaml structure
  - models: Validate model definitions

Examples:
  starmap validate              # Validate entire catalog
  starmap validate catalog      # Explicit catalog validation
  starmap validate providers    # Validate only providers
  starmap validate authors      # Validate only authors
  starmap validate models       # Validate model definitions`,
	RunE: validatecmd.CatalogCmd.RunE, // Default to catalog validation
}

func init() {
	rootCmd.AddCommand(validateCmd)

	// Add subcommands
	validateCmd.AddCommand(validatecmd.CatalogCmd)
	validateCmd.AddCommand(validatecmd.ProvidersCmd)
	validateCmd.AddCommand(validatecmd.AuthorsCmd)
	validateCmd.AddCommand(validatecmd.ModelsCmd)
}
