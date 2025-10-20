package fetch

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/agentstation/starmap/internal/cmd/application"
)

// NewCommand creates the fetch command using app context.
func NewCommand(app application.Application) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "fetch [resource]",
		GroupID: "catalog",
		Short:   "Fetch live data from provider APIs",
		Long: `Fetch retrieves live data from provider APIs.

This requires the appropriate API key to be configured either through
environment variables or the configuration file.

Supported providers include: openai, anthropic, google-ai-studio, google-vertex, groq`,
		Example: `  starmap fetch models              # Fetch all providers
  starmap fetch models openai       # Fetch OpenAI models
  starmap fetch models openai --raw # Get raw OpenAI API response`,
		RunE: func(cmd *cobra.Command, args []string) error {
			// Default to models if no subcommand
			if len(args) == 0 {
				return cmd.Help()
			}
			return fmt.Errorf("unknown resource: %s", args[0])
		},
	}

	// Add subcommands with app context
	cmd.AddCommand(NewModelsCommand(app))

	return cmd
}
