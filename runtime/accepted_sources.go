package runtime

import (
	"slices"

	"github.com/agentstation/starmap/pkg/sources"
)

// appendAcceptedSource caches valid input separately from empty failure receipts.
// Complete empty inventories still count because they can withdraw prior records.
func appendAcceptedSource(accepted []sources.ID, observation sources.Observation) []sources.ID {
	switch observation.SourceID {
	case sources.ProvidersID, sources.LocalCatalogID, sources.ModelsDevHTTPID, sources.ModelsDevGitID:
	default:
		return accepted
	}
	if observation.Catalog == nil {
		return accepted
	}
	complete := observation.Status == sources.ObservationStatusSucceeded && observation.Completeness == sources.ObservationCompletenessComplete
	if !complete && observation.Catalog.Providers().Len() == 0 && observation.Catalog.Authors().Len() == 0 && len(observation.Catalog.AuthoredModels()) == 0 {
		return accepted
	}
	if !slices.Contains(accepted, observation.SourceID) {
		accepted = append(accepted, observation.SourceID)
		slices.Sort(accepted)
	}
	return accepted
}
