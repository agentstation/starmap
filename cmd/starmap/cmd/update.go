package cmd

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"

	"github.com/agentstation/starmap"
	"github.com/agentstation/starmap/internal/cmd/cmdutil"
	"github.com/agentstation/starmap/internal/cmd/output"
	"github.com/agentstation/starmap/pkg/catalogs"
	"github.com/agentstation/starmap/pkg/errors"
)

// newUpdateCommand creates the update command.
func newUpdateCommand() *cobra.Command {
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
			return performUpdate(ctx, updateFlags)
		},
	}

	// Add update-specific flags
	updateFlags = cmdutil.AddUpdateFlags(cmd)

	return cmd
}

// performUpdate executes the catalog update.
//
//nolint:gocyclo // Complex update logic with multiple options
func performUpdate(ctx context.Context, flags *cmdutil.UpdateFlags) error {
	// Show warning for force update
	if flags.Force {
		if !globalFlags.Quiet {
			fmt.Fprintf(os.Stderr, "⚠️  WARNING: Force mode will DELETE all existing model files and replace them with fresh API models.\n")
		}
		if !flags.AutoApprove {
			fmt.Printf("\nContinue with force update? (y/N): ")
			var response string
			if _, err := fmt.Scanln(&response); err != nil {
				response = "n"
			}
			response = strings.ToLower(strings.TrimSpace(response))
			if response != "y" && response != "yes" {
				fmt.Println("Force update cancelled")
				return nil
			}
		}
		fmt.Println()
	}

	// Create starmap instance
	var sm starmap.Starmap
	var err error

	if flags.Input != "" {
		// Use file-based catalog from input directory
		filesCatalog, err := catalogs.New(catalogs.WithFiles(flags.Input))
		if err != nil {
			return errors.WrapResource("create", "catalog", flags.Input, err)
		}
		sm, err = starmap.New(starmap.WithInitialCatalog(filesCatalog))
		if err != nil {
			return errors.WrapResource("create", "starmap", "files catalog", err)
		}
		if !globalFlags.Quiet {
			fmt.Fprintf(os.Stderr, "📁 Using catalog from: %s\n", flags.Input)
		}
	} else {
		// Use default starmap with embedded catalog
		sm, err = starmap.New()
		if err != nil {
			return errors.WrapResource("create", "starmap", "", err)
		}
		if !globalFlags.Quiet {
			fmt.Fprintf(os.Stderr, "📦 Using embedded catalog\n")
		}
	}

	// Build sync options - use default output path if not specified
	outputPath := flags.Output
	if outputPath == "" {
		outputPath = "./internal/embedded/catalog"
	}
	opts := buildSyncOptions(flags.Provider, outputPath, flags.DryRun, flags.Force, flags.AutoApprove, flags.Cleanup, flags.Reformat)

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

	// Human-readable output
	if result.HasChanges() {
		if !globalFlags.Quiet {
			fmt.Fprintf(os.Stderr, "=== UPDATE RESULTS ===\n\n")
		}

		if !globalFlags.Quiet {
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

		// Ask for confirmation unless auto-approve or dry-run
		if result.DryRun {
			if !globalFlags.Quiet {
				fmt.Fprintf(os.Stderr, "🔍 Dry run mode - no changes will be made\n")
			}
			return nil
		}

		if !flags.AutoApprove {
			fmt.Printf("Apply these changes? (y/N): ")
			var response string
			if _, err := fmt.Scanln(&response); err != nil {
				response = "n"
			}
			response = strings.ToLower(strings.TrimSpace(response))
			if response != "y" && response != "yes" {
				fmt.Println("Update cancelled")
				return nil
			}

			// Re-run sync without dry-run
			if !globalFlags.Quiet {
				fmt.Fprintf(os.Stderr, "\n🚀 Applying changes...\n")
				fmt.Fprintf(os.Stderr, "📁 Saving models to: %s\n", outputPath)
			}

			// Call sync again without dry-run
			finalOpts := buildSyncOptions(flags.Provider, outputPath, false, flags.Force, false, flags.Cleanup, flags.Reformat)

			_, err := sm.Sync(ctx, finalOpts...)
			if err != nil {
				return &errors.ProcessError{
					Operation: "apply changes",
					Command:   "update",
					Err:       err,
				}
			}
		}

		if !globalFlags.Quiet {
			fmt.Fprintf(os.Stderr, "\n🎉 Update completed successfully!\n")
			fmt.Fprintf(os.Stderr, "📊 Total: %s\n", result.Summary())
		}
	} else {
		if !globalFlags.Quiet {
			fmt.Fprintf(os.Stderr, "✅ All providers are up to date - no changes needed\n")
		}
	}

	return nil
}

// buildSyncOptions creates a slice of sync options based on the provided flags.
func buildSyncOptions(provider, output string, dryRun, force, autoApprove, cleanup, reformat bool) []starmap.SyncOption {
	var opts []starmap.SyncOption

	if provider != "" {
		opts = append(opts, starmap.WithProvider(catalogs.ProviderID(provider)))
	}
	if dryRun {
		opts = append(opts, starmap.WithDryRun(true))
	}
	if autoApprove {
		opts = append(opts, starmap.WithAutoApprove(true))
	}
	if output != "" {
		opts = append(opts, starmap.WithOutputPath(output))
	}
	// Use typed options for source-specific behavior
	if force {
		opts = append(opts, starmap.WithFresh(true))
	}
	if cleanup {
		opts = append(opts, starmap.WithCleanModelsDevRepo(true))
	}
	if reformat {
		opts = append(opts, starmap.WithReformat(true))
	}

	return opts
}

func init() {
	rootCmd.AddCommand(newUpdateCommand())
}
