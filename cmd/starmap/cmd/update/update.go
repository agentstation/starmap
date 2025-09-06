package update

import (
	"context"
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/agentstation/starmap"
	"github.com/agentstation/starmap/internal/cmd/cmdutil"
	"github.com/agentstation/starmap/internal/cmd/output"
	"github.com/agentstation/starmap/pkg/errors"
)

// Flags holds flags for update command.
type Flags struct {
	Provider    string
	Source      string
	DryRun      bool
	Force       bool
	AutoApprove bool
	Output      string
	Input       string
	Cleanup     bool
	Reformat    bool
}

// addUpdateFlags adds update-specific flags to the update command.
func addUpdateFlags(cmd *cobra.Command) *Flags {
	flags := &Flags{}

	cmd.Flags().StringVarP(&flags.Provider, "provider", "p", "",
		"Update specific provider only")
	cmd.Flags().StringVar(&flags.Source, "source", "",
		"Update from specific source (provider-api, models.dev)")
	cmd.Flags().BoolVar(&flags.DryRun, "dry-run", false,
		"Preview changes without applying them")
	cmd.Flags().BoolVarP(&flags.Force, "force", "f", false,
		"Force fresh update (delete and recreate)")
	cmd.Flags().BoolVarP(&flags.AutoApprove, "yes", "y", false,
		"Auto-approve changes without confirmation")
	cmd.Flags().StringVar(&flags.Output, "output", "",
		"Save updated catalog to directory")
	cmd.Flags().StringVar(&flags.Input, "input", "",
		"Load catalog from directory instead of embedded")
	cmd.Flags().BoolVar(&flags.Cleanup, "cleanup", false,
		"Remove temporary models.dev repository after update")
	cmd.Flags().BoolVar(&flags.Reformat, "reformat", false,
		"Reformat catalog files even without changes")

	return flags
}

// NewCommand creates the update command.
func NewCommand(globalFlags *cmdutil.GlobalFlags) *cobra.Command {
	var updateFlags *Flags

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
	updateFlags = addUpdateFlags(cmd)

	return cmd
}

// ExecuteUpdate orchestrates the complete update process.
func ExecuteUpdate(ctx context.Context, flags *Flags, globalFlags *cmdutil.GlobalFlags) error {
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

	// Execute the update operation
	return update(ctx, sm, flags, globalFlags)
}

// update executes the update operation and handles the results.
func update(ctx context.Context, sm starmap.Starmap, flags *Flags, globalFlags *cmdutil.GlobalFlags) error {
	// Build update options - use default output path if not specified
	outputPath := flags.Output
	if outputPath == "" {
		outputPath = "./internal/embedded/catalog"
	}
	opts := BuildUpdateOptions(flags.Provider, outputPath, flags.DryRun, flags.Force, flags.AutoApprove, flags.Cleanup, flags.Reformat)

	if !globalFlags.Quiet {
		fmt.Fprintf(os.Stderr, "\n🔄 Starting update...\n\n")
	}

	// Perform the update
	result, err := sm.Sync(ctx, opts...)
	if err != nil {
		return &errors.ProcessError{
			Operation: "update catalog",
			Command:   "update",
			Err:       err,
		}
	}

	// Display results based on output format
	if globalFlags.Output == "json" || globalFlags.Output == "yaml" {
		formatter := output.NewFormatter(output.Format(globalFlags.Output))
		return formatter.Format(os.Stdout, result)
	}

	// Handle results
	return handleResults(ctx, sm, result, flags, outputPath, globalFlags)
}

// handleResults processes the update results and handles user interaction.
func handleResults(ctx context.Context, sm starmap.Starmap, result *starmap.Result, flags *Flags, outputPath string, globalFlags *cmdutil.GlobalFlags) error {
	if !result.HasChanges() {
		if !globalFlags.Quiet {
			fmt.Fprintf(os.Stderr, "✅ All providers are up to date - no changes needed\n")
		}
		return nil
	}

	// Show results summary
	if !globalFlags.Quiet {
		displayResultsSummary(result)
	}

	// Handle dry run
	if result.DryRun {
		if !globalFlags.Quiet {
			fmt.Fprintf(os.Stderr, "🔍 Dry run mode - no changes will be made\n")
		}
		return nil
	}

	// Handle auto-approve vs manual confirmation
	if flags.AutoApprove {
		return finalizeChanges(globalFlags.Quiet, result)
	}

	// Ask for confirmation
	confirmed, err := ConfirmChanges()
	if err != nil {
		return err
	}
	if !confirmed {
		return nil
	}

	// Re-run update without dry-run
	if !globalFlags.Quiet {
		fmt.Fprintf(os.Stderr, "\n🚀 Applying changes...\n")
		fmt.Fprintf(os.Stderr, "📁 Saving models to: %s\n", outputPath)
	}

	// Call update again without dry-run
	finalOpts := BuildUpdateOptions(flags.Provider, outputPath, false, flags.Force, false, flags.Cleanup, flags.Reformat)

	_, err = sm.Sync(ctx, finalOpts...)
	if err != nil {
		return &errors.ProcessError{
			Operation: "apply changes",
			Command:   "update",
			Err:       err,
		}
	}

	return finalizeChanges(globalFlags.Quiet, result)
}

// displayResultsSummary shows a detailed summary of the update results.
func displayResultsSummary(result *starmap.Result) {
	fmt.Fprintf(os.Stderr, "=== UPDATE RESULTS ===\n\n")

	// Show summary for each provider
	for providerID, providerResult := range result.ProviderResults {
		if providerResult.HasChanges() {
			fmt.Fprintf(os.Stderr, "🔄 %s:\n", providerID)

			// Show API fetch status
			if providerResult.APIModelsCount > 0 {
				fmt.Fprintf(os.Stderr, "  📡 Provider API: %d models found\n", providerResult.APIModelsCount)
			} else {
				// When no models from API but we have updates, it's from enrichment
				if providerResult.UpdatedCount > 0 {
					fmt.Fprintf(os.Stderr, "  ⏭️  Provider API: Skipped (using cached models)\n")
				} else {
					fmt.Fprintf(os.Stderr, "  ⏭️  Provider API: No models fetched\n")
				}
			}

			// Show enrichment if models were updated but not added
			if providerResult.UpdatedCount > 0 && providerResult.AddedCount == 0 {
				fmt.Fprintf(os.Stderr, "  🔗 Enriched: %d models with pricing/limits from models.dev\n", providerResult.UpdatedCount)
			}

			// Show changes summary
			if providerResult.AddedCount > 0 || providerResult.RemovedCount > 0 {
				fmt.Fprintf(os.Stderr, "  📊 Changes: %d added, %d updated, %d removed\n",
					providerResult.AddedCount, providerResult.UpdatedCount, providerResult.RemovedCount)
			} else if providerResult.UpdatedCount > 0 {
				fmt.Fprintf(os.Stderr, "  📊 Changes: %d models enriched\n", providerResult.UpdatedCount)
			}
			fmt.Fprintf(os.Stderr, "\n")
		}
	}
}

// finalizeChanges displays the completion message.
func finalizeChanges(isQuiet bool, result *starmap.Result) error {
	if !isQuiet {
		fmt.Fprintf(os.Stderr, "\n🎉 Update completed successfully!\n")
		fmt.Fprintf(os.Stderr, "📊 Total: %s\n", result.Summary())
	}
	return nil
}
