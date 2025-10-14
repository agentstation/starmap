package uninstall

import (
	"github.com/spf13/cobra"

	"github.com/agentstation/starmap/cmd/starmap/application"
)

// NewCommand creates the uninstall command using app context.
func NewCommand(app application.Application) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "uninstall",
		Short: "Uninstall starmap components",
		Long:  `Uninstall shell completions and other starmap components.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return cmd.Help()
		},
	}

	// Add subcommands
	cmd.AddCommand(NewCompletionCommand(app))

	return cmd
}
