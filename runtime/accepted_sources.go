package runtime

import (
	"slices"

	"github.com/agentstation/starmap/pkg/catalogs"
	"github.com/agentstation/starmap/pkg/sources"
)

// acceptedSourceIDs caches the bounded local source set during catalog reconstruction.
// Status reads this set without scanning receipts or rebuilding catalog evidence.
func acceptedSourceIDs(receipts []catalogs.SourceObservationLink) []sources.ID {
	var accepted []sources.ID
	for _, receipt := range receipts {
		switch receipt.Source {
		case sources.ProvidersID, sources.LocalCatalogID, sources.ModelsDevHTTPID, sources.ModelsDevGitID:
			if !slices.Contains(accepted, receipt.Source) {
				accepted = append(accepted, receipt.Source)
			}
		}
	}
	slices.Sort(accepted)
	return accepted
}
