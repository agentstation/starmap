package cmd

import (
	"github.com/spf13/cobra"

	"github.com/agentstation/starmap/cmd/starmap/cmd/generate"
)

// generateCmd represents the parent generate command.
var generateCmd = &cobra.Command{
	Use:   "generate",
	Short: "Generate various artifacts (completion)",
	Long: `Generate creates various artifacts for the starmap project.

Available subcommands:
  completion - Generate shell completion scripts (bash, zsh, fish)`,
	Example: `  starmap generate completion bash`,
}

func init() {
	rootCmd.AddCommand(generateCmd)

	// Add subcommands
	generateCmd.AddCommand(generate.CompletionCmd)
}
