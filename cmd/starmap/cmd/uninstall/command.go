package uninstall

import (
	"github.com/spf13/cobra"
)

// NewCommand creates the uninstall command.
func NewCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "uninstall",
		Short: "Uninstall starmap components",
		Long:  `Uninstall shell completions and other starmap components.`,
		RunE: func(cmd *cobra.Command, _ []string) error {
			return cmd.Help()
		},
	}

	// Add subcommands
	cmd.AddCommand(NewCompletionCommand())

	return cmd
}
