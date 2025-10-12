package validate

import (
	"github.com/spf13/cobra"

	"github.com/agentstation/starmap/internal/appcontext"
)

// NewCommand creates the validate command using app context.
func NewCommand(appCtx appcontext.Interface) *cobra.Command {
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
		RunE: func(cmd *cobra.Command, args []string) error {
			return cmd.Help()
		},
	}

	// Add subcommands with app context
	cmd.AddCommand(NewModelsCommand(appCtx))
	cmd.AddCommand(NewProvidersCommand(appCtx))
	cmd.AddCommand(NewAuthorsCommand(appCtx))
	cmd.AddCommand(NewCatalogCommand(appCtx))

	return cmd
}
