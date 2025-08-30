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
	updateFlagProvider       string
	updateFlagAutoApprove    bool
	updateFlagDryRun         bool
	updateFlagOutput         string
	updateFlagInput          string
	updateFlagFresh          bool
	updateFlagCleanModelsDev bool
	updateFlagForceFormat    bool
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
  starmap update --output /path/to/custom/providers  # update to custom directory
  starmap update --input ./internal/embedded/catalog  # use files from directory instead of embedded
  starmap update --fresh -p groq  # fresh update - overwrite all models for groq provider
  starmap update --clean-models-dev  # cleanup models.dev repository after update`,
	RunE: runUpdate,
}

func init() {
	rootCmd.AddCommand(updateCmd)

	updateCmd.Flags().StringVarP(&updateFlagProvider, "provider", "p", "", "Provider to update models from (updates all if not specified)")
	updateCmd.Flags().BoolVarP(&updateFlagAutoApprove, "auto-approve", "y", false, "Skip confirmation prompts")
	updateCmd.Flags().BoolVar(&updateFlagDryRun, "dry-run", false, "Show what would change without making modifications")
	updateCmd.Flags().StringVarP(&updateFlagOutput, "output", "o", "", "Output directory for providers (default: internal/embedded/catalog/providers)")
	updateCmd.Flags().StringVarP(&updateFlagInput, "input", "i", "", "Input directory to load catalog from (default: use embedded catalog)")
	updateCmd.Flags().BoolVar(&updateFlagFresh, "fresh", false, "Perform fresh update - delete all existing models and write all API models (destructive)")
	updateCmd.Flags().BoolVar(&updateFlagCleanModelsDev, "clean-models-dev", false, "Remove models.dev repository after update (saves disk space)")
	updateCmd.Flags().BoolVar(&updateFlagForceFormat, "force-format", false, "Force reformat of providers.yaml even if no changes detected")
}

func runUpdate(cmd *cobra.Command, args []string) error {
	// Get command context (includes signal handling if set up)
	ctx := cmd.Context()
	
	// Show warning for fresh update
	if updateFlagFresh {
		fmt.Printf("⚠️  WARNING: Fresh update mode will DELETE all existing model files and replace them with the latest API models.\n")
		if !updateFlagAutoApprove {
			fmt.Printf("\nContinue with fresh update? (y/N): ")
			var response string
			if _, err := fmt.Scanln(&response); err != nil {
				response = "n"
			}
			response = strings.ToLower(strings.TrimSpace(response))
			if response != "y" && response != "yes" {
				fmt.Println("Fresh update cancelled")
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

	// Build sync options
	opts := buildSyncOptions(updateFlagProvider, updateFlagOutput, updateFlagDryRun, updateFlagFresh, updateFlagAutoApprove, updateFlagCleanModelsDev, updateFlagForceFormat)

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
				fmt.Printf("  📡 Fetched %d models from API\n", providerResult.APIModelsCount)
				if providerResult.ExistingModelsCount > 0 {
					fmt.Printf("  📚 Found %d existing models in catalog\n", providerResult.ExistingModelsCount)
				}
				if providerResult.EnhancedCount > 0 {
					fmt.Printf("  🔗 Enhanced %d models with models.dev data\n", providerResult.EnhancedCount)
				}
				fmt.Printf("  📊 Changes: %d added, %d updated, %d removed\n\n",
					providerResult.AddedCount, providerResult.UpdatedCount, providerResult.RemovedCount)
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
			if updateFlagOutput != "" {
				fmt.Printf("📁 Saving models to: %s\n", updateFlagOutput)
			}

			// Call sync again without dry-run
			finalOpts := buildSyncOptions(updateFlagProvider, updateFlagOutput, false, updateFlagFresh, false, updateFlagCleanModelsDev, updateFlagForceFormat)

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
func buildSyncOptions(provider, output string, dryRun, fresh, autoApprove, cleanModelsDev, forceFormat bool) []starmap.SyncOption {
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
	// Pass source-specific options via context
	if fresh {
		opts = append(opts, starmap.WithContext("fresh", true))
	}
	if cleanModelsDev {
		opts = append(opts, starmap.WithContext("cleanModelsDevRepo", true))
	}
	if forceFormat {
		opts = append(opts, starmap.WithContext("forceFormat", true))
	}

	return opts
}
