// Package cmd provides the main command structure for the starmap CLI.
package cmd

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/agentstation/starmap/cmd/starmap/cmd/fetch"
)

// fetchCmd represents the parent fetch command.
var fetchCmd = &cobra.Command{
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

func init() {
	rootCmd.AddCommand(fetchCmd)

	// Add subcommands
	fetchCmd.AddCommand(fetch.NewModelsCmd(globalFlags))
}
