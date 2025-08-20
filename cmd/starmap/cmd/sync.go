package cmd

import (
	"context"
	"fmt"
	"strings"

	"github.com/agentstation/starmap"
	"github.com/agentstation/starmap/internal/catalogs/operations"
	"github.com/agentstation/starmap/internal/catalogs/persistence"
	"github.com/agentstation/starmap/internal/sources/modelsdev"
	"github.com/agentstation/starmap/internal/sources/providers"
	"github.com/agentstation/starmap/pkg/catalogs"
	"github.com/agentstation/starmap/pkg/catalogs/files"
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
				// If scan fails, default to no
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

	// Get the catalog - use file catalog if input is specified, otherwise use default
	var catalog catalogs.Catalog

	if syncInput != "" {
		// Create file-based catalog from input directory
		filesCatalog, err := files.New(syncInput)
		if err != nil {
			return fmt.Errorf("creating catalog from %s: %w", syncInput, err)
		}
		sm, err := starmap.New(starmap.WithInitialCatalog(filesCatalog))
		if err != nil {
			return fmt.Errorf("creating starmap with files catalog: %w", err)
		}
		catalog, err = sm.Catalog()
		if err != nil {
			return fmt.Errorf("getting catalog: %w", err)
		}
		fmt.Printf("📁 Using catalog from: %s\n", syncInput)
	} else {
		// Use default starmap with embedded catalog
		sm, err := starmap.New()
		if err != nil {
			return fmt.Errorf("creating starmap: %w", err)
		}
		catalog, err = sm.Catalog()
		if err != nil {
			return fmt.Errorf("getting catalog: %w", err)
		}
		fmt.Printf("📦 Using embedded catalog\n")
	}

	// Determine which providers to sync
	var providersToSync []catalogs.ProviderID
	if syncProvider != "" {
		// Sync specific provider
		pid := catalogs.ProviderID(syncProvider)
		providersToSync = []catalogs.ProviderID{pid}
	} else {
		// Sync all supported providers
		providersToSync = providers.ListSupportedProviders()
	}

	fmt.Printf("\nStarting sync for %d provider(s)...\n\n", len(providersToSync))

	// Setup models.dev integration
	var modelsDevAPI *modelsdev.ModelsDevAPI
	var modelsDevClient *modelsdev.Client

	// Determine output directory for models.dev
	outputDir := syncOutput
	if outputDir == "" {
		outputDir = "internal/embedded/catalog/providers"
	}

	// Initialize models.dev client
	modelsDevClient = modelsdev.NewClient(outputDir)

	fmt.Printf("🌐 Setting up models.dev integration...\n")

	// Clone/update models.dev repository
	if err := modelsDevClient.EnsureRepository(); err != nil {
		fmt.Printf("  ⚠️  Warning: Could not setup models.dev repository: %v\n", err)
		fmt.Printf("  📝 Continuing without models.dev enhancement...\n")
	} else {
		// Build api.json
		if err := modelsDevClient.BuildAPI(); err != nil {
			fmt.Printf("  ⚠️  Warning: Could not build api.json: %v\n", err)
			fmt.Printf("  📝 Continuing without models.dev enhancement...\n")
		} else {
			// Parse api.json
			fmt.Printf("  📋 Parsing models.dev data...\n")
			api, err := modelsdev.ParseAPI(modelsDevClient.GetAPIPath())
			if err != nil {
				fmt.Printf("  ❌ Could not parse api.json: %v\n", err)
				fmt.Printf("  📝 Continuing without models.dev enhancement...\n")
			} else {
				modelsDevAPI = api
				fmt.Printf("  ✅ models.dev data loaded successfully\n")
			}
		}
	}

	fmt.Printf("\n")

	var totalChanges int
	var allChanges []operations.ProviderChangeset

	// Process each provider
	for _, providerID := range providersToSync {
		fmt.Printf("🔄 Checking %s...\n", providerID)

		// Get provider from catalog
		provider, found := catalog.Providers().Get(providerID)
		if !found {
			fmt.Printf("  ⚠️  Skipping %s: provider not found in catalog\n\n", providerID)
			continue
		}

		// Load API key from environment
		provider.LoadAPIKey()

		// Get client for this provider (handles API key requirements automatically)
		result, err := provider.GetClient(catalogs.WithAllowMissingAPIKey(true))
		if err != nil {
			fmt.Printf("  ⚠️  Skipping %s: %v\n\n", providerID, err)
			continue
		}
		if result.Error != nil {
			fmt.Printf("  ⚠️  Skipping %s: %v\n\n", providerID, result.Error)
			continue
		}
		client := result.Client

		if !result.APIKeyRequired {
			fmt.Printf("  📡 API key not required for %s\n", providerID)
		}

		// Fetch models from API
		ctx := context.Background()
		apiModels, err := client.ListModels(ctx)
		if err != nil {
			fmt.Printf("  ❌ Failed to fetch models from %s: %v\n\n", providerID, err)
			continue
		}

		fmt.Printf("  📡 Fetched %d models from API\n", len(apiModels))

		// Enhance API models with models.dev data BEFORE comparison
		if modelsDevAPI != nil {
			var enhancedModels int
			apiModels, enhancedModels = modelsdev.EnhanceModelsWithModelsDevData(apiModels, providerID, modelsDevAPI)
			fmt.Printf("  🔗 Enhanced %d models with models.dev data\n", enhancedModels)
		}

		// Handle fresh sync vs normal sync
		var changeset operations.ProviderChangeset
		if syncFresh {
			// Fresh sync: treat all API models as new additions
			fmt.Printf("  🆕 Fresh sync mode: treating all %d API models as new\n", len(apiModels))
			changeset = operations.ProviderChangeset{
				ProviderID: providerID,
				Added:      apiModels,
				Updated:    []operations.ModelUpdate{},
				Removed:    []catalogs.Model{},
			}
		} else {
			// Normal sync: compare with existing models
			existingModels, err := persistence.GetProviderModels(catalog, providerID)
			if err != nil {
				fmt.Printf("  ⚠️  Error getting existing models: %v\n", err)
				existingModels = make(map[string]catalogs.Model)
			}

			fmt.Printf("  📚 Found %d existing models in catalog\n", len(existingModels))
			changeset = operations.CompareProviderModels(providerID, existingModels, apiModels)
		}

		allChanges = append(allChanges, changeset)

		// Display summary for this provider
		changeCount := len(changeset.Added) + len(changeset.Updated) + len(changeset.Removed)
		totalChanges += changeCount

		if changeCount == 0 {
			fmt.Printf("  ✅ No changes needed\n\n")
		} else {
			fmt.Printf("  📊 Changes: %d added, %d updated, %d removed\n\n",
				len(changeset.Added), len(changeset.Updated), len(changeset.Removed))
		}
	}

	// Show detailed diff if there are changes
	if totalChanges > 0 {
		fmt.Printf("=== DETAILED CHANGES ===\n\n")
		for _, changeset := range allChanges {
			if changeset.HasChanges() {
				operations.PrintChangeset(changeset)
			}
		}

		// Ask for confirmation unless auto-approve or dry-run
		if syncDryRun {
			fmt.Printf("🔍 Dry run mode - no changes will be made\n")
			return nil
		}

		if !syncAutoApprove {
			fmt.Printf("Apply these changes? (y/N): ")
			var response string
			if _, err := fmt.Scanln(&response); err != nil {
				// If scan fails, default to no
				response = "n"
			}
			response = strings.ToLower(strings.TrimSpace(response))
			if response != "y" && response != "yes" {
				fmt.Println("Sync cancelled")
				return nil
			}
		}

		// Apply changes
		fmt.Printf("\n🚀 Applying changes...\n")
		if syncOutput != "" {
			fmt.Printf("📁 Saving models to: %s\n", syncOutput)
		}
		for _, changeset := range allChanges {
			if changeset.HasChanges() {
				// Clean provider directory if fresh sync
				if syncFresh {
					fmt.Printf("🧹 Cleaning existing models for %s...\n", changeset.ProviderID)
					if err := persistence.CleanProviderDirectory(changeset.ProviderID, syncOutput); err != nil {
						return fmt.Errorf("cleaning directory for %s: %w", changeset.ProviderID, err)
					}
				}

				if err := persistence.ApplyChangesetToOutput(catalog, changeset, syncOutput); err != nil {
					return fmt.Errorf("applying changes for %s: %w", changeset.ProviderID, err)
				}
				if syncOutput != "" {
					fmt.Printf("✅ Applied changes for %s to %s/%s\n", changeset.ProviderID, syncOutput, changeset.ProviderID)
				} else {
					fmt.Printf("✅ Applied changes for %s\n", changeset.ProviderID)
				}
			}
		}

		// Copy provider logos if models.dev is available
		if modelsDevClient != nil {
			fmt.Printf("\n🎨 Copying provider logos...\n")
			logoOutputDir := syncOutput
			if logoOutputDir == "" {
				logoOutputDir = "internal/embedded/catalog/providers"
			}
			if err := modelsdev.CopyProviderLogos(modelsDevClient, logoOutputDir, providersToSync); err != nil {
				fmt.Printf("  ⚠️  Warning: Could not copy logos: %v\n", err)
			} else {
				fmt.Printf("  ✅ Provider logos copied successfully\n")
			}
		}

		fmt.Printf("\n🎉 Sync completed successfully!\n")
	} else {
		fmt.Printf("✅ All providers are up to date - no changes needed\n")
	}

	// Cleanup models.dev repository if requested
	if syncCleanModelsDev && modelsDevClient != nil {
		fmt.Printf("\n🧹 Cleaning up models.dev repository...\n")
		if err := modelsDevClient.Cleanup(); err != nil {
			fmt.Printf("  ⚠️  Warning: Could not cleanup models.dev repository: %v\n", err)
		} else {
			fmt.Printf("  ✅ models.dev repository cleaned up\n")
		}
	}

	return nil
}
