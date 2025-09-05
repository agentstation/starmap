package cmd

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/agentstation/starmap/cmd/starmap/cmd/list"
)

// listCmd represents the parent list command.
var listCmd = &cobra.Command{
	Use:     "list [resource]",
	GroupID: "core",
	Short:   "List resources from local catalog",
	Long: `List displays resources from the local starmap catalog.

Available subcommands:
  models      - AI models and their capabilities
  providers   - Model providers and API endpoints  
  authors     - Model creators and organizations`,
	Example: `  starmap list models                      # List all models
  starmap list models claude-3-5-sonnet    # Show specific model details
  starmap list providers                   # List all providers
  starmap list authors                     # List all authors`,
	RunE: func(cmd *cobra.Command, args []string) error {
		// Default to models if no subcommand
		if len(args) == 0 {
			return cmd.Help()
		}
		return fmt.Errorf("unknown resource: %s", args[0])
	},
}

func init() {
	rootCmd.AddCommand(listCmd)

	// Add subcommands
	listCmd.AddCommand(list.ModelsCmd)
	listCmd.AddCommand(list.ProvidersCmd)
	listCmd.AddCommand(list.AuthorsCmd)
}
