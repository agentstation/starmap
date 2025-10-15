package validate

import (
	"github.com/spf13/cobra"

	"github.com/agentstation/starmap/cmd/application"
)

// NewCommand creates the validate command using app context.
func NewCommand(app application.Application) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "validate",
		GroupID: "management",
		Short:   "Validate catalog configuration and structure",
		Long: `Validate catalog configuration and structure.

This command validates various aspects of the catalog including:
  - Model definitions
  - Provider configurations
  - Author information
  - Overall catalog consistency`,
		RunE: func(cmd *cobra.Command, _ []string) error {
			return cmd.Help()
		},
	}

	// Add subcommands with app context
	cmd.AddCommand(NewModelsCommand(app))
	cmd.AddCommand(NewProvidersCommand(app))
	cmd.AddCommand(NewAuthorsCommand(app))
	cmd.AddCommand(NewCatalogCommand(app))

	return cmd
}
