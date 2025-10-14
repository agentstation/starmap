package fetch

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/agentstation/starmap/cmd/starmap/context"
)

// NewCommand creates the fetch command using app context.
func NewCommand(appCtx context.Context) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "fetch [resource]",
		GroupID: "core",
		Short:   "Retrieve resources from provider APIs",
		Long: `Fetch retrieves live data from provider APIs.

This requires the appropriate API key to be configured either through
environment variables or the configuration file.

Supported providers include: openai, anthropic, google-ai-studio, google-vertex, groq`,
		Example: `  starmap fetch models --provider openai
  starmap fetch models --all`,
		RunE: func(cmd *cobra.Command, args []string) error {
			// Default to models if no subcommand
			if len(args) == 0 {
				return cmd.Help()
			}
			return fmt.Errorf("unknown resource: %s", args[0])
		},
	}

	// Add subcommands with app context
	cmd.AddCommand(NewModelsCommand(appCtx))

	return cmd
}
