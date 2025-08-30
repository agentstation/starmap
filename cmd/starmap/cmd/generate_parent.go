package cmd

import (
	"github.com/agentstation/starmap/cmd/starmap/cmd/generate"
	"github.com/spf13/cobra"
)

// generateParentCmd represents the parent generate command
var generateParentCmd = &cobra.Command{
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
	rootCmd.AddCommand(generateParentCmd)
	
	// Add subcommands
	generateParentCmd.AddCommand(generate.DocsCmd)
	generateParentCmd.AddCommand(generate.SiteCmd)
}