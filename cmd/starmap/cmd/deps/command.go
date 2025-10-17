package deps

import (
	"github.com/spf13/cobra"

	"github.com/agentstation/starmap/cmd/application"
)

// NewCommand creates the deps command using app context.
func NewCommand(app application.Application) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "deps",
		GroupID: "management",
		Short:   "Manage external dependencies",
		Long: `Check and manage external tool dependencies required by data sources.

Data sources may require external tools like 'bun' for building data locally.
This command helps you check which dependencies are installed and provides
installation instructions for missing ones.`,
		RunE: func(cmd *cobra.Command, _ []string) error {
			return cmd.Help()
		},
	}

	// Add subcommands with app context
	cmd.AddCommand(NewCheckCommand(app))

	return cmd
}
