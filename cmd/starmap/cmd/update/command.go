package update

import (
	"github.com/spf13/cobra"

	"github.com/agentstation/starmap/internal/cmd/application"
)

// NewCommand creates the update command using app context.
func NewCommand(app application.Application) *cobra.Command {
	var flags *Flags

	cmd := &cobra.Command{
		Use:     "update [provider]",
		GroupID: "core",
		Short:   "Synchronize catalog with all sources",
		Args:    cobra.MaximumNArgs(1),
		Long: `Update synchronizes your local starmap catalog by fetching the latest data
from all configured sources:

1. Provider APIs - Live model information from OpenAI, Anthropic, etc.
2. models.dev - Pricing, limits, and metadata enrichment
3. Embedded catalog - Baseline catalog data

The command will:
• Load the current catalog (embedded or from --input-dir)
• Fetch live data from provider APIs (if keys configured)
• Enrich with models.dev data (pricing, limits, logos)
• Reconcile all sources using field-level authority
• Save the updated catalog to disk

By default, saves to ~/.starmap for the local user catalog.`,
		Example: `  starmap update                            # Update entire catalog
  starmap update openai                     # Update specific provider
  starmap update --dry                      # Preview changes
  starmap update -y                         # Auto-approve changes
  starmap update --force                    # Force fresh update
  starmap update openai --dry               # Preview OpenAI updates`,
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			logger := app.Logger()

			// Extract provider from positional argument if present
			// This takes precedence over the --provider flag
			if len(args) == 1 {
				flags.Provider = args[0]
			}

			return ExecuteUpdate(ctx, app, flags, logger)
		},
	}

	// Add update-specific flags
	flags = addUpdateFlags(cmd)

	return cmd
}
