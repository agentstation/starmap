package install

import (
	"github.com/spf13/cobra"

	"github.com/agentstation/starmap/cmd/starmap/context"
)

// NewCommand creates the install command using app context.
func NewCommand(appCtx context.Context) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "install",
		Short: "Install starmap components",
		Long:  `Install shell completions and other starmap components.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return cmd.Help()
		},
	}

	// Add subcommands
	cmd.AddCommand(NewCompletionCommand(appCtx))

	return cmd
}
