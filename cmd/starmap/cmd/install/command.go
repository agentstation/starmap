package install

import (
	"github.com/spf13/cobra"
)

// NewCommand creates the install command.
func NewCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "install",
		Short: "Install starmap components",
		Long:  `Install shell completions and other starmap components.`,
		RunE: func(cmd *cobra.Command, _ []string) error {
			return cmd.Help()
		},
	}

	// Add subcommands
	cmd.AddCommand(NewCompletionCommand())

	return cmd
}
