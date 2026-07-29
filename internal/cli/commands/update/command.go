package update

import (
	"github.com/spf13/cobra"
)

// NewCommand creates the update command using app context.
func NewCommand(app application) *cobra.Command {
	var flags *Flags

	cmd := &cobra.Command{
		Use:     "update [provider]",
		GroupID: "catalog",
		Short:   "Synchronize catalog with the default or selected sources",
		Args:    cobra.MaximumNArgs(1),
		Long: `Update synchronizes your local starmap catalog by fetching the latest data
from all configured sources:

1. Provider APIs - Live model information from OpenAI, Anthropic, etc.
2. models.dev - Pricing, limits, and metadata enrichment
3. Embedded catalog - Baseline catalog data

The command will:
• Load the current catalog workspace, with embedded data as the offline fallback
• Fetch live data from provider APIs (if keys configured)
• Enrich with models.dev data (descriptions, features, pricing, limits, logos)
• Reconcile all sources using field-level authority
• Save the updated catalog to the same human workspace

By default, the human provider-YAML workspace is ~/.starmap/catalog. Machine
generation state is separate and is never treated as editable configuration.`,
		Example: `  starmap update                            # Update entire catalog
  starmap update openai                     # Update specific provider
  starmap update --dry-run                  # Preview changes
  starmap update -y                         # Auto-approve changes
  starmap update --force                    # Force fresh update
  starmap update --source local             # Reload semantic workspace edits
  starmap update openai --dry-run           # Preview OpenAI updates`,
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			logger := app.Logger()

			// Extract the optional provider identity from the positional argument.
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
