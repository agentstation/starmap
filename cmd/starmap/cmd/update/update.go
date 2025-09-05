package update

import (
	"context"

	"github.com/spf13/cobra"

	"github.com/agentstation/starmap/internal/cmd/cmdutil"
)

// NewCommand creates the update command.
func NewCommand(globalFlags *cmdutil.GlobalFlags) *cobra.Command {
	var updateFlags *cmdutil.UpdateFlags

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

By default, saves to ./internal/embedded/catalog for development.`,
		Example: `  starmap update                            # Update entire catalog
  starmap update --provider openai          # Update specific provider
  starmap update --dry-run                  # Preview changes
  starmap update -y                          # Auto-approve changes
  starmap update --force                    # Force fresh update`,
		RunE: func(cmd *cobra.Command, _ []string) error {
			ctx := cmd.Context()
			return ExecuteUpdate(ctx, updateFlags, globalFlags)
		},
	}

	// Add update-specific flags
	updateFlags = cmdutil.AddUpdateFlags(cmd)

	return cmd
}

// ExecuteUpdate orchestrates the complete update process.
func ExecuteUpdate(ctx context.Context, flags *cmdutil.UpdateFlags, globalFlags *cmdutil.GlobalFlags) error {
	// Validate force update if needed
	if flags.Force {
		proceed, err := ValidateForceUpdate(globalFlags.Quiet, flags.AutoApprove)
		if err != nil {
			return err
		}
		if !proceed {
			return nil
		}
	}

	// Load the appropriate catalog
	sm, err := LoadCatalog(flags.Input, globalFlags.Quiet)
	if err != nil {
		return err
	}

	// Execute the sync operation
	return ExecuteSync(ctx, sm, flags, globalFlags)
}
