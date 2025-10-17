package update

import (
	"github.com/spf13/cobra"

	"github.com/agentstation/starmap/internal/cmd/application"
)

// NewCommand creates the update command using app context.
func NewCommand(app application.Application) *cobra.Command {
	var flags *Flags

	cmd := &cobra.Command{
		Use:     "update",
		GroupID: "core",
		Short:   "Synchronize catalog with all sources",
		Long: `Update synchronizes your local starmap catalog by fetching the latest data
from all configured sources:

1. Provider APIs - Live model information from OpenAI, Anthropic, etc.
2. models.dev - Pricing, limits, and metadata enrichment
3. Embedded catalog - Baseline catalog data

The command will:
• Load the current catalog (embedded or from --input)
• Fetch live data from provider APIs (if keys configured)
• Enrich with models.dev data (pricing, limits, logos)
• Reconcile all sources using field-level authority
• Save the updated catalog to disk

By default, saves to ~/.starmap for the local user catalog.`,
		Example: `  starmap update                            # Update entire catalog
  starmap update --provider openai          # Update specific provider
  starmap update --dry                      # Preview changes
  starmap update -y                          # Auto-approve changes
  starmap update --force                    # Force fresh update`,
		RunE: func(cmd *cobra.Command, _ []string) error {
			ctx := cmd.Context()
			logger := app.Logger()
			return ExecuteUpdate(ctx, app, flags, logger)
		},
	}

	// Add update-specific flags
	flags = addUpdateFlags(cmd)

	return cmd
}
