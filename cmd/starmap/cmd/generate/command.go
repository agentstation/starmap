package generate

import (
	"github.com/spf13/cobra"

	"github.com/agentstation/starmap/cmd/application"
)

// NewCommand creates the generate command using app context.
func NewCommand(app application.Application) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "generate",
		Short: "Generate various artifacts (completion)",
		Long:  `Generate shell completion scripts and other artifacts.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return cmd.Help()
		},
	}

	// Add subcommands
	cmd.AddCommand(NewCompletionCommand(app))

	return cmd
}
