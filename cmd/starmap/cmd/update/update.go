package update

import (
	stdctx "context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/rs/zerolog"
	"github.com/spf13/cobra"

	"github.com/agentstation/starmap"
	"github.com/agentstation/starmap/cmd/starmap/context"
	"github.com/agentstation/starmap/internal/cmd/output"
	"github.com/agentstation/starmap/pkg/constants"
	"github.com/agentstation/starmap/pkg/errors"
	"github.com/agentstation/starmap/pkg/sync"
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
	SourcesDir  string
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
	cmd.Flags().BoolVar(&flags.DryRun, "dry", false,
		"Preview changes without applying them (alias for --dry-run)")
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
	cmd.Flags().StringVar(&flags.SourcesDir, "sources-dir", "",
		"Directory for external source data (default: ~/.starmap/sources)")

	return flags
}

// ExecuteUpdate orchestrates the complete update process using app context.
func ExecuteUpdate(ctx stdctx.Context, appCtx context.Context, flags *Flags, logger *zerolog.Logger) error {
	// Determine quiet mode from logger level
	quiet := logger.GetLevel() > zerolog.InfoLevel

	// Validate force update if needed
	if flags.Force {
		proceed, err := ValidateForceUpdate(quiet, flags.AutoApprove)
		if err != nil {
			return err
		}
		if !proceed {
			return nil
		}
	}

	// Load the appropriate catalog using app context
	sm, err := LoadCatalog(appCtx, flags.Input, quiet)
	if err != nil {
		return err
	}

	// Execute the update operation
	return updateCatalog(ctx, sm, flags, logger, quiet)
}

// updateCatalog executes the update operation using app context.
func updateCatalog(ctx stdctx.Context, sm starmap.Starmap, flags *Flags, logger *zerolog.Logger, quiet bool) error {
	// Build update options - use default output path if not specified
	outputPath := flags.Output
	if outputPath == "" {
		outputPath = expandPath(constants.DefaultCatalogPath)
	}
	// Support environment variable fallback for sources directory
	sourcesDir := flags.SourcesDir
	if sourcesDir == "" {
		sourcesDir = os.Getenv("STARMAP_SOURCES_DIR")
	}

	opts := BuildUpdateOptions(flags.Provider, outputPath, flags.DryRun, flags.Force, flags.AutoApprove, flags.Cleanup, flags.Reformat, sourcesDir)

	if !quiet {
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

	// Display results based on output format (checking if JSON logging is enabled)
	if logger.GetLevel() == zerolog.TraceLevel {
		// Assume structured output for trace level
		formatter := output.NewFormatter(output.FormatJSON)
		return formatter.Format(os.Stdout, result)
	}

	// Handle results
	return handleResults(ctx, sm, result, flags, outputPath, sourcesDir, quiet)
}

// handleResults processes the update results using app context.
func handleResults(ctx stdctx.Context, sm starmap.Starmap, result *sync.Result, flags *Flags, outputPath string, sourcesDir string, quiet bool) error {
	if !result.HasChanges() {
		if !quiet {
			fmt.Fprintf(os.Stderr, "✅ All providers are up to date - no changes needed\n")
		}
		return nil
	}

	// Show results summary
	if !quiet {
		displayResultsSummary(result)
	}

	// Handle dry run
	if result.DryRun {
		if !quiet {
			fmt.Fprintf(os.Stderr, "🔍 Dry run mode - no changes will be made\n")
		}
		return nil
	}

	// Handle auto-approve vs manual confirmation
	if flags.AutoApprove {
		return finalizeChanges(quiet, result)
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
	if !quiet {
		fmt.Fprintf(os.Stderr, "\n🚀 Applying changes...\n")
	}

	// Rebuild options without dry-run
	opts := BuildUpdateOptions(flags.Provider, outputPath, false, flags.Force, flags.AutoApprove, flags.Cleanup, flags.Reformat, sourcesDir)

	// Apply changes
	finalResult, err := sm.Sync(ctx, opts...)
	if err != nil {
		return &errors.ProcessError{
			Operation: "apply changes",
			Command:   "update",
			Err:       err,
		}
	}

	return finalizeChanges(quiet, finalResult)
}

// displayResultsSummary shows a detailed summary of the update results.
func displayResultsSummary(result *sync.Result) {
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
func finalizeChanges(isQuiet bool, result *sync.Result) error {
	if !isQuiet {
		fmt.Fprintf(os.Stderr, "\n🎉 Update completed successfully!\n")
		fmt.Fprintf(os.Stderr, "📊 Total: %s\n", result.Summary())
	}
	return nil
}

// expandPath expands a path that may contain ~ to the user's home directory.
func expandPath(path string) string {
	if !strings.HasPrefix(path, "~") {
		return path
	}

	homeDir, err := os.UserHomeDir()
	if err != nil {
		// Fall back to the original path if we can't get home dir
		return path
	}

	if path == "~" {
		return homeDir
	}

	if strings.HasPrefix(path, "~/") {
		return filepath.Join(homeDir, path[2:])
	}

	return path
}
