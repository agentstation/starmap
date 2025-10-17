package deps

import (
	"github.com/spf13/cobra"
)

// NewCommand creates the deps command.
func NewCommand() *cobra.Command {
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

	// Add subcommands
	cmd.AddCommand(NewCheckCommand())

	return cmd
}
