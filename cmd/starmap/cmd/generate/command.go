// Package generate provides commands for generating artifacts like shell completions.
package generate

import (
	"github.com/spf13/cobra"
)

// NewCommand creates the generate command.
func NewCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "generate",
		Short: "Generate various artifacts (completion)",
		Long:  `Generate shell completion scripts and other artifacts.`,
		RunE: func(cmd *cobra.Command, _ []string) error {
			return cmd.Help()
		},
	}

	// Add subcommands
	cmd.AddCommand(NewCompletionCommand())

	return cmd
}
