package cmd

import (
	"github.com/spf13/cobra"

	"github.com/agentstation/starmap/cmd/starmap/cmd/inspect"
)

// inspectCmd represents the inspect command for examining embedded filesystem.
var inspectCmd = &cobra.Command{
	Use:   "inspect",
	Short: "Inspect embedded filesystem",
	Long: `Inspect the embedded filesystem within the starmap binary.

The inspect command provides Unix-like tools to navigate and examine 
embedded files and directories, including catalog data and source files.

Examples:
  starmap inspect ls                    # List all embedded files
  starmap inspect ls catalog/providers  # List providers directory
  starmap inspect cat catalog/models.yaml  # View file contents
  starmap inspect tree                  # Show directory tree`,
	Run: func(cmd *cobra.Command, _ []string) {
		// Show help when called without subcommand
		_ = cmd.Help()
	},
}

func init() {
	rootCmd.AddCommand(inspectCmd)

	// Add subcommands
	inspectCmd.AddCommand(inspect.LsCmd)
	inspectCmd.AddCommand(inspect.CatCmd)
	inspectCmd.AddCommand(inspect.TreeCmd)
	inspectCmd.AddCommand(inspect.StatCmd)
}
