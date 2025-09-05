// Package generate provides commands for generating documentation and site content.
package generate

import (
	"context"
	"fmt"

	"github.com/spf13/cobra"

	"github.com/agentstation/starmap/internal/tools/docs"
	"github.com/agentstation/starmap/pkg/catalogs"
)

var outputDir string

// DocsCmd represents the generate docs command.
var DocsCmd = &cobra.Command{
	Use:   "docs",
	Short: "Generate markdown documentation for providers and models",
	Long: `Generate creates comprehensive markdown documentation for all providers and models
in the starmap catalog. The documentation includes:

• Main index with provider overview
• Individual provider pages with model listings  
• Detailed model specification pages
• Cross-referenced navigation links

The documentation is organized hierarchically and optimized for GitHub viewing.`,
	Example: `  starmap generate docs
  starmap generate docs --output ./documentation
  starmap generate docs -o ./my-docs`,
	RunE: runGenerateDocs,
}

func init() {
	DocsCmd.Flags().StringVarP(&outputDir, "output", "o", "./docs", "Output directory for generated documentation")
}

func runGenerateDocs(_ *cobra.Command, _ []string) error {
	fmt.Printf("📝 Generating documentation in %s...\n", outputDir)

	// Initialize the catalog with embedded data
	catalog, err := catalogs.New(catalogs.WithEmbedded())
	if err != nil {
		return fmt.Errorf("initializing catalog: %w", err)
	}

	// Use the new generator package
	generator := docs.New(
		docs.WithOutputDir(outputDir),
		docs.WithVerbose(true),
	)

	// Generate all documentation
	if err := generator.Generate(context.Background(), catalog); err != nil {
		return fmt.Errorf("generating documentation: %w", err)
	}

	fmt.Println("✅ Documentation generation complete!")
	return nil
}
