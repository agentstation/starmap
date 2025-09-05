package cmd

import (
	"github.com/spf13/cobra"

	"github.com/agentstation/starmap/cmd/starmap/cmd/generate"
)

// generateCmd represents the parent generate command.
var generateCmd = &cobra.Command{
	Use:   "generate",
	Short: "Generate various artifacts (docs, site, completion)",
	Long: `Generate creates various artifacts for the starmap project.

Available subcommands:
  completion - Generate shell completion scripts (bash, zsh, fish)
  docs       - Generate markdown documentation for providers and models
  site       - Generate static documentation website using Hugo`,
	Example: `  starmap generate completion bash
  starmap generate docs
  starmap generate site --prod`,
}

func init() {
	rootCmd.AddCommand(generateCmd)

	// Add subcommands
	generateCmd.AddCommand(generate.CompletionCmd)
	generateCmd.AddCommand(generate.DocsCmd)
	generateCmd.AddCommand(generate.SiteCmd)
}
