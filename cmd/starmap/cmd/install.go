package cmd

import (
	"github.com/spf13/cobra"

	"github.com/agentstation/starmap/cmd/starmap/cmd/install"
)

// installCmd represents the parent install command.
var installCmd = &cobra.Command{
	Use:   "install",
	Short: "Install starmap components",
	Long: `Install various starmap components like shell completions.

Available subcommands:
  completion  - Install shell completions for bash, zsh, and fish`,
	Example: `  starmap install completion
  starmap install completion --bash
  starmap install completion --zsh --fish`,
}

func init() {
	rootCmd.AddCommand(installCmd)

	// Add subcommands
	installCmd.AddCommand(install.CompletionCmd)
}
