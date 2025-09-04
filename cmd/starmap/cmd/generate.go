package cmd

import (
	"github.com/agentstation/starmap/cmd/starmap/cmd/generate"
	"github.com/spf13/cobra"
)

// generateCmd represents the parent generate command
var generateCmd = &cobra.Command{
	Use:   "generate",
	Short: "Generate various artifacts (docs, site, testdata)",
	Long: `Generate creates various artifacts for the starmap project.

Available subcommands:
  docs      - Generate markdown documentation for providers and models
  site      - Generate static documentation website using Hugo
  testdata  - Generate or update testdata files from provider APIs`,
	Example: `  starmap generate docs
  starmap generate site --prod
  starmap generate testdata --provider openai`,
}

func init() {
	rootCmd.AddCommand(generateCmd)

	// Add subcommands
	generateCmd.AddCommand(generate.DocsCmd)
	generateCmd.AddCommand(generate.SiteCmd)
}
