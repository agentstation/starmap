package install

import (
	"github.com/spf13/cobra"

	"github.com/agentstation/starmap/internal/appcontext"
)

// NewCommand creates the install command using app context.
func NewCommand(appCtx appcontext.Interface) *cobra.Command {
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
