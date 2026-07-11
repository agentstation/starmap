package update

import (
	"fmt"
	"os"

	"github.com/agentstation/starmap/pkg/sync"
)

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

			// Show enrichment only when models.dev was an active source.
			if providerResult.EnhancedCount > 0 {
				fmt.Fprintf(os.Stderr, "  🔗 Enriched: %d models with models.dev data\n", providerResult.EnhancedCount)
			}

			// Show changes summary
			if providerResult.AddedCount > 0 || providerResult.RemovedCount > 0 {
				fmt.Fprintf(os.Stderr, "  📊 Changes: %d added, %d updated, %d removed\n",
					providerResult.AddedCount, providerResult.UpdatedCount, providerResult.RemovedCount)
			} else if providerResult.UpdatedCount > 0 {
				fmt.Fprintf(os.Stderr, "  📊 Changes: %d updated\n", providerResult.UpdatedCount)
			}
			fmt.Fprintf(os.Stderr, "\n")
		}
	}
}
