package update

import (
	"context"
	"fmt"
	"os"

	"github.com/agentstation/starmap"
	"github.com/agentstation/starmap/internal/cmd/cmdutil"
	"github.com/agentstation/starmap/internal/cmd/output"
	"github.com/agentstation/starmap/pkg/errors"
)

// ExecuteSync performs the sync operation and handles the results.
func ExecuteSync(ctx context.Context, sm starmap.Starmap, flags *cmdutil.UpdateFlags, globalFlags *cmdutil.GlobalFlags) error {
	// Build sync options - use default output path if not specified
	outputPath := flags.Output
	if outputPath == "" {
		outputPath = "./internal/embedded/catalog"
	}
	opts := BuildSyncOptions(flags.Provider, outputPath, flags.DryRun, flags.Force, flags.AutoApprove, flags.Cleanup, flags.Reformat)

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

// handleResults processes the sync results and handles user interaction.
func handleResults(ctx context.Context, sm starmap.Starmap, result *starmap.SyncResult, flags *cmdutil.UpdateFlags, outputPath string, globalFlags *cmdutil.GlobalFlags) error {
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

	// Re-run sync without dry-run
	if !globalFlags.Quiet {
		fmt.Fprintf(os.Stderr, "\n🚀 Applying changes...\n")
		fmt.Fprintf(os.Stderr, "📁 Saving models to: %s\n", outputPath)
	}

	// Call sync again without dry-run
	finalOpts := BuildSyncOptions(flags.Provider, outputPath, false, flags.Force, false, flags.Cleanup, flags.Reformat)

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

// displayResultsSummary shows a detailed summary of the sync results.
func displayResultsSummary(result *starmap.SyncResult) {
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
func finalizeChanges(isQuiet bool, result *starmap.SyncResult) error {
	if !isQuiet {
		fmt.Fprintf(os.Stderr, "\n🎉 Update completed successfully!\n")
		fmt.Fprintf(os.Stderr, "📊 Total: %s\n", result.Summary())
	}
	return nil
}
