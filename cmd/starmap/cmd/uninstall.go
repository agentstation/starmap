package cmd

import (
	"github.com/spf13/cobra"

	"github.com/agentstation/starmap/cmd/starmap/cmd/uninstall"
)

// uninstallCmd represents the parent uninstall command.
var uninstallCmd = &cobra.Command{
	Use:   "uninstall",
	Short: "Uninstall starmap components",
	Long: `Uninstall various starmap components like shell completions.

Available subcommands:
  completion  - Remove shell completions for bash, zsh, and fish`,
	Example: `  starmap uninstall completion
  starmap uninstall completion --bash
  starmap uninstall completion --zsh --fish`,
}

func init() {
	rootCmd.AddCommand(uninstallCmd)

	// Add subcommands
	uninstallCmd.AddCommand(uninstall.CompletionCmd)
}
