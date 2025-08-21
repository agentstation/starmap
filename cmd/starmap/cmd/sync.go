package cmd

import (
	"fmt"
	"strings"

	"github.com/agentstation/starmap"
	"github.com/agentstation/starmap/pkg/catalogs"
	"github.com/agentstation/starmap/pkg/catalogs/files"
	"github.com/agentstation/starmap/pkg/sources"
	"github.com/spf13/cobra"
)

var (
	syncProvider       string
	syncAutoApprove    bool
	syncDryRun         bool
	syncOutput         string
	syncInput          string
	syncFresh          bool
	syncCleanModelsDev bool
)

// syncCmd represents the sync command
var syncCmd = &cobra.Command{
	Use:   "sync",
	Short: "Sync models from provider APIs to the embedded catalog",
	Long: `Sync fetches the current list of available models from provider APIs
and updates the embedded catalog. This requires appropriate API keys to be 
configured either through environment variables or the configuration file.

The command will:
1. Load the embedded catalog
2. Check for available API keys
3. Fetch models from providers with configured API keys
4. Clone/update models.dev repository for enhanced model data
5. Generate api.json with comprehensive model specifications
6. Merge API data with models.dev data and existing catalog
7. Show a diff of changes
8. Ask for confirmation (unless --auto-approve is used)
9. Update the embedded catalog files if approved
10. Copy provider logos from models.dev

Models are saved to internal/embedded/catalog/providers/<provider_id>/<model_id>.yaml`,
	Example: `  starmap sync --provider openai
  starmap sync -p anthropic --auto-approve
  starmap sync --dry-run
  starmap sync -y  # sync all providers with auto-approve
  starmap sync --output /path/to/custom/providers  # sync to custom directory
  starmap sync --input ./internal/embedded/catalog  # use files from directory instead of embedded
  starmap sync --fresh -p groq  # fresh sync - overwrite all models for groq provider
  starmap sync --clean-models-dev  # cleanup models.dev repository after sync`,
	RunE: runSync,
}

func init() {
	rootCmd.AddCommand(syncCmd)

	syncCmd.Flags().StringVarP(&syncProvider, "provider", "p", "", "Provider to sync models from (syncs all if not specified)")
	syncCmd.Flags().BoolVarP(&syncAutoApprove, "auto-approve", "y", false, "Skip confirmation prompts")
	syncCmd.Flags().BoolVar(&syncDryRun, "dry-run", false, "Show what would change without making modifications")
	syncCmd.Flags().StringVarP(&syncOutput, "output", "o", "", "Output directory for providers (default: internal/embedded/catalog/providers)")
	syncCmd.Flags().StringVarP(&syncInput, "input", "i", "", "Input directory to load catalog from (default: use embedded catalog)")
	syncCmd.Flags().BoolVar(&syncFresh, "fresh", false, "Perform fresh sync - delete all existing models and write all API models (destructive)")
	syncCmd.Flags().BoolVar(&syncCleanModelsDev, "clean-models-dev", false, "Remove models.dev repository after sync (saves disk space)")
}

func runSync(cmd *cobra.Command, args []string) error {
	// Show warning for fresh sync
	if syncFresh {
		fmt.Printf("⚠️  WARNING: Fresh sync mode will DELETE all existing model files and replace them with API models.\n")
		fmt.Printf("⚠️  This is a DESTRUCTIVE operation that cannot be undone.\n")
		if !syncAutoApprove {
			fmt.Printf("\nContinue with fresh sync? (y/N): ")
			var response string
			if _, err := fmt.Scanln(&response); err != nil {
				response = "n"
			}
			response = strings.ToLower(strings.TrimSpace(response))
			if response != "y" && response != "yes" {
				fmt.Println("Fresh sync cancelled")
				return nil
			}
		}
		fmt.Println()
	}

	// Create starmap instance
	var sm starmap.Starmap
	var err error

	if syncInput != "" {
		// Use file-based catalog from input directory
		filesCatalog, err := files.New(syncInput)
		if err != nil {
			return fmt.Errorf("creating catalog from %s: %w", syncInput, err)
		}
		sm, err = starmap.New(starmap.WithInitialCatalog(filesCatalog))
		if err != nil {
			return fmt.Errorf("creating starmap with files catalog: %w", err)
		}
		fmt.Printf("📁 Using catalog from: %s\n", syncInput)
	} else {
		// Use default starmap with embedded catalog
		sm, err = starmap.New()
		if err != nil {
			return fmt.Errorf("creating starmap: %w", err)
		}
		fmt.Printf("📦 Using embedded catalog\n")
	}

	// Build sync options
	opts := buildSyncOptions(syncProvider, syncOutput, syncDryRun, syncFresh, syncAutoApprove, syncCleanModelsDev)

	fmt.Printf("\nStarting sync...\n\n")

	// Perform the sync
	result, err := sm.Sync(opts...)
	if err != nil {
		return fmt.Errorf("sync failed: %w", err)
	}

	// Display results
	if result.HasChanges() {
		fmt.Printf("=== SYNC RESULTS ===\n\n")

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

		if !syncAutoApprove {
			fmt.Printf("Apply these changes? (y/N): ")
			var response string
			if _, err := fmt.Scanln(&response); err != nil {
				response = "n"
			}
			response = strings.ToLower(strings.TrimSpace(response))
			if response != "y" && response != "yes" {
				fmt.Println("Sync cancelled")
				return nil
			}

			// Re-run sync without dry-run
			fmt.Printf("\n🚀 Applying changes...\n")
			if syncOutput != "" {
				fmt.Printf("📁 Saving models to: %s\n", syncOutput)
			}

			// Call sync again without dry-run
			finalOpts := buildSyncOptions(syncProvider, syncOutput, false, syncFresh, false, syncCleanModelsDev)

			_, err := sm.Sync(finalOpts...)
			if err != nil {
				return fmt.Errorf("applying changes failed: %w", err)
			}
		}

		fmt.Printf("\n🎉 Sync completed successfully!\n")
		fmt.Printf("📊 Total: %s\n", result.Summary())
	} else {
		fmt.Printf("✅ All providers are up to date - no changes needed\n")
	}

	return nil
}

// buildSyncOptions creates a slice of sync options based on the provided flags
func buildSyncOptions(provider, output string, dryRun, fresh, autoApprove, cleanModelsDev bool) []sources.SyncOption {
	var opts []sources.SyncOption

	if provider != "" {
		opts = append(opts, sources.SyncWithProvider(catalogs.ProviderID(provider)))
	}
	if dryRun {
		opts = append(opts, sources.SyncWithDryRun(true))
	}
	if fresh {
		opts = append(opts, sources.SyncWithFreshSync(true))
	}
	if autoApprove {
		opts = append(opts, sources.SyncWithAutoApprove(true))
	}
	if output != "" {
		opts = append(opts, sources.SyncWithOutputDir(output))
	}
	if cleanModelsDev {
		opts = append(opts, sources.SyncWithCleanModelsDevRepo(true))
	}

	return opts
}
