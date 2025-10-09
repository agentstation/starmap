package list

import (
	"fmt"

	"github.com/rs/zerolog"
	"github.com/spf13/cobra"

	"github.com/agentstation/starmap/pkg/catalogs"
)

// AppContext defines the interface that list commands need from the app.
// This allows for better testability and decoupling from the full app.
type AppContext interface {
	Catalog() (catalogs.Catalog, error)
	Logger() *zerolog.Logger
}

// NewCommand creates the list command with app dependencies.
func NewCommand(app AppContext) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "list [resource]",
		GroupID: "core",
		Short:   "List resources from local catalog",
		Long: `List displays resources from the local starmap catalog.

Available subcommands:
  models      - AI models and their capabilities
  providers   - Model providers and API endpoints
  authors     - Model creators and organizations`,
		Example: `  starmap list models                      # List all models
  starmap list models claude-3-5-sonnet    # Show specific model details
  starmap list providers                   # List all providers
  starmap list authors                     # List all authors`,
		RunE: func(cmd *cobra.Command, args []string) error {
			// Default to help if no subcommand
			if len(args) == 0 {
				return cmd.Help()
			}
			return fmt.Errorf("unknown resource: %s", args[0])
		},
	}

	// Add subcommands using the app context
	cmd.AddCommand(NewModelsCommand(app))
	cmd.AddCommand(NewProvidersCommand(app))
	cmd.AddCommand(NewAuthorsCommand(app))

	return cmd
}
