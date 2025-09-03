package cmd

import (
	"fmt"
	"strings"

	"github.com/agentstation/starmap"
	"github.com/agentstation/starmap/pkg/catalogs"
	"github.com/agentstation/starmap/pkg/errors"
	"github.com/spf13/cobra"
)

var (
	updateFlagProvider    string
	updateFlagAutoApprove bool
	updateFlagDryRun      bool
	updateFlagOutput      string
	updateFlagInput       string
	updateFlagForce       bool
	updateFlagCleanup     bool
	updateFlagReformat    bool
)

// updateCmd represents the update command
var updateCmd = &cobra.Command{
	Use:   "update",
	Short: "Update your local starmap catalog of authors, models, and providers.",
	Long: `Update loads the starmap embedded catalog and fetches the latest catalog data 
using your local provider API Keys (and auth sessions). This will include any special 
access you have to models, including beta or preview models. 

As a final step, starmap will pull from additional sources to enrich the data further 
and save the catalog to ~/.starmap/ by default. 

This will overwrite and append to an existing saved catalog.

The command will:
1. Load the embedded catalog
2. Check for configured provider API keys
3. Fetch model data from providers
4. Update the embedded catalog with the latest model data
5. Fetch data from additional sources
6. Enrich the catalog with the latest data
7. Save the catalog to ~/.starmap/ by default`,
	Example: `  starmap update --provider openai
  starmap update -p anthropic --auto-approve
  starmap update --dry-run
  starmap update -y  # update all providers with auto-approve
  starmap update --output /path/to/custom/providers  # save to custom directory
  starmap update --input ./internal/embedded/catalog  # load from directory instead of embedded
  starmap update --force -p groq  # force fresh update - delete and refetch all groq models
  starmap update --cleanup  # remove temporary models.dev repository after update`,
	RunE: runUpdate,
}

func init() {
	rootCmd.AddCommand(updateCmd)

	// Core functionality
	updateCmd.Flags().StringVarP(&updateFlagProvider, "provider", "p", "", "Update models from specific provider (default: all providers)")

	// Input/Output options
	updateCmd.Flags().StringVarP(&updateFlagInput, "input", "i", "", "Load catalog from directory instead of embedded version")
	updateCmd.Flags().StringVarP(&updateFlagOutput, "output", "o", "", "Save updated catalog to directory (default: internal/embedded/catalog/providers)")

	// Behavior modifiers
	updateCmd.Flags().BoolVarP(&updateFlagDryRun, "dry-run", "n", false, "Preview changes without applying them")
	updateCmd.Flags().BoolVarP(&updateFlagAutoApprove, "auto-approve", "y", false, "Apply changes without confirmation prompts")

	// Special operations (potentially destructive)
	updateCmd.Flags().BoolVarP(&updateFlagForce, "force", "f", false, "Delete existing models and fetch fresh from APIs (destructive)")
	updateCmd.Flags().BoolVarP(&updateFlagCleanup, "cleanup", "c", false, "Remove temporary models.dev repository after update")
	updateCmd.Flags().BoolVar(&updateFlagReformat, "reformat", false, "Reformat providers.yaml file even without changes")
}

func runUpdate(cmd *cobra.Command, args []string) error {
	// Get command context (includes signal handling if set up)
	ctx := cmd.Context()

	// Show warning for force update
	if updateFlagForce {
		fmt.Printf("⚠️  WARNING: Force mode will DELETE all existing model files and replace them with fresh API models.\n")
		if !updateFlagAutoApprove {
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

	if updateFlagInput != "" {
		// Use file-based catalog from input directory
		filesCatalog, err := catalogs.New(catalogs.WithFiles(updateFlagInput))
		if err != nil {
			return errors.WrapResource("create", "catalog", updateFlagInput, err)
		}
		sm, err = starmap.New(starmap.WithInitialCatalog(filesCatalog))
		if err != nil {
			return errors.WrapResource("create", "starmap", "files catalog", err)
		}
		fmt.Printf("📁 Using catalog from: %s\n", updateFlagInput)
	} else {
		// Use default starmap with embedded catalog
		sm, err = starmap.New()
		if err != nil {
			return errors.WrapResource("create", "starmap", "", err)
		}
		fmt.Printf("📦 Using embedded catalog\n")
	}

	// Build sync options - use default output path if not specified
	outputPath := updateFlagOutput
	if outputPath == "" {
		outputPath = "./internal/embedded/catalog"
	}
	opts := buildSyncOptions(updateFlagProvider, outputPath, updateFlagDryRun, updateFlagForce, updateFlagAutoApprove, updateFlagCleanup, updateFlagReformat)

	fmt.Printf("\nStarting update...\n\n")

	// Perform the update
	result, err := sm.Sync(ctx, opts...)
	if err != nil {
		return &errors.ProcessError{
			Operation: "update catalog",
			Command:   "update",
			Err:       err,
		}
	}

	// Display results
	if result.HasChanges() {
		fmt.Printf("=== UPDATE RESULTS ===\n\n")

		// Show summary for each provider
		for providerID, providerResult := range result.ProviderResults {
			if providerResult.HasChanges() {
				fmt.Printf("🔄 %s:\n", providerID)

				// Show API fetch status
				if providerResult.APIModelsCount > 0 {
					fmt.Printf("  📡 Provider API: %d models found\n", providerResult.APIModelsCount)
				} else {
					// When no models from API but we have updates, it's from enrichment
					if providerResult.UpdatedCount > 0 {
						fmt.Printf("  ⏭️  Provider API: Skipped (using cached models)\n")
					} else {
						fmt.Printf("  ⏭️  Provider API: No models fetched\n")
					}
				}

				// Show enrichment if models were updated but not added
				if providerResult.UpdatedCount > 0 && providerResult.AddedCount == 0 {
					fmt.Printf("  🔗 Enriched: %d models with pricing/limits from models.dev\n", providerResult.UpdatedCount)
				}

				// Show changes summary
				if providerResult.AddedCount > 0 || providerResult.RemovedCount > 0 {
					fmt.Printf("  📊 Changes: %d added, %d updated, %d removed\n",
						providerResult.AddedCount, providerResult.UpdatedCount, providerResult.RemovedCount)
				} else if providerResult.UpdatedCount > 0 {
					fmt.Printf("  📊 Changes: %d models enriched\n", providerResult.UpdatedCount)
				}
				fmt.Printf("\n")
			}
		}

		// Ask for confirmation unless auto-approve or dry-run
		if result.DryRun {
			fmt.Printf("🔍 Dry run mode - no changes will be made\n")
			return nil
		}

		if !updateFlagAutoApprove {
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
			fmt.Printf("\n🚀 Applying changes...\n")
			fmt.Printf("📁 Saving models to: %s\n", outputPath)

			// Call sync again without dry-run
			finalOpts := buildSyncOptions(updateFlagProvider, outputPath, false, updateFlagForce, false, updateFlagCleanup, updateFlagReformat)

			_, err := sm.Sync(ctx, finalOpts...)
			if err != nil {
				return &errors.ProcessError{
					Operation: "apply changes",
					Command:   "update",
					Err:       err,
				}
			}
		}

		fmt.Printf("\n🎉 Update completed successfully!\n")
		fmt.Printf("📊 Total: %s\n", result.Summary())
	} else {
		fmt.Printf("✅ All providers are up to date - no changes needed\n")
	}

	return nil
}

// buildSyncOptions creates a slice of sync options based on the provided flags
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
