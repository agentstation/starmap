package generate

import (
	"context"
	"fmt"

	"github.com/spf13/cobra"

	"github.com/agentstation/starmap/internal/cmd/notify"
	"github.com/agentstation/starmap/internal/tools/site"
	"github.com/agentstation/starmap/pkg/catalogs"
	"github.com/agentstation/starmap/pkg/constants"
	"github.com/agentstation/starmap/pkg/logging"
)

var (
	siteBaseURL string
	siteOutput  string
	prodBuild   bool
)

// SiteCmd represents the generate site command.
var SiteCmd = &cobra.Command{
	Use:   "site",
	Short: "Generate static documentation website",
	Long:  `Generate a static documentation website using Hugo from the current catalog and markdown files.`,
	Example: `  starmap generate site
  starmap generate site --prod
  starmap generate site --base-url https://mysite.com/`,
	RunE: runGenerateSite,
}

func init() {
	SiteCmd.Flags().StringVar(&siteBaseURL, "base-url", "https://starmap.agentstation.ai/", "Base URL for the site")
	SiteCmd.Flags().StringVar(&siteOutput, "output", "./site/public", "Output directory for generated site")
	SiteCmd.Flags().BoolVar(&prodBuild, "prod", false, "Build for production (minified, optimized)")
}

func runGenerateSite(cmd *cobra.Command, _ []string) error {
	ctx, cancel := context.WithTimeout(context.Background(), constants.CommandTimeout)
	defer cancel()

	logging.Info().Msg("Generating documentation site")

	// Create site instance
	config := &site.Config{
		RootDir:    "./site",
		ContentDir: "./docs",
		BaseURL:    siteBaseURL,
	}

	siteInstance, err := site.New(config)
	if err != nil {
		return fmt.Errorf("creating site: %w", err)
	}

	// Load catalog for metadata
	catalog, err := catalogs.New(catalogs.WithEmbedded())
	if err != nil {
		logging.Warn().Msg("Could not load catalog for metadata enrichment")
		// Continue without catalog - basic generation still works
	}

	// Generate the site
	if err := siteInstance.Generate(ctx, catalog); err != nil {
		return fmt.Errorf("generating site: %w", err)
	}

	outputMsg := "./site/public"
	if siteOutput != outputMsg {
		outputMsg = siteOutput
	}

	logging.Info().
		Str("output", outputMsg).
		Msg("Site generated successfully")

	// Create notifier and show contextual hints
	notifier, err := notify.NewFromCommand(cmd)
	if err != nil {
		return err
	}
	
	hintContext := notify.Contexts.Command("generate", "site", true, "")
	
	if prodBuild {
		// Production build - just hints since completion is obvious
		return notifier.Hints(hintContext)
	} else {
		// Development build - show next step since it's actionable
		return notifier.Info("Run 'starmap serve site' to preview", hintContext)
	}
}
