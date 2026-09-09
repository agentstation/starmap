package runtime

import (
	"slices"
	"testing"

	"github.com/agentstation/starmap/pkg/catalogs"
	"github.com/agentstation/starmap/pkg/sources"
)

func TestAcceptedSourceSetExcludesDistributionAndDuplicates(t *testing.T) {
	var receipts []catalogs.SourceObservationLink
	for _, source := range []sources.ID{sources.EmbeddedCatalogID, sources.ReleaseArtifactID, sources.ProvidersID, sources.LocalCatalogID, sources.LocalCatalogID, "unknown"} {
		receipts = append(receipts, catalogs.SourceObservationLink{
			Source: source, Status: sources.ObservationStatusSucceeded,
			Completeness: sources.ObservationCompletenessComplete,
		})
	}
	receipts = append(receipts, catalogs.SourceObservationLink{
		Source: sources.ModelsDevHTTPID, Status: sources.ObservationStatusDegraded,
		Completeness: sources.ObservationCompletenessPartial,
	})
	actual := acceptedSourceIDs(receipts)
	if !slices.Equal(actual, []sources.ID{sources.LocalCatalogID, sources.ModelsDevHTTPID, sources.ProvidersID}) {
		t.Fatalf("accepted sources=%v", actual)
	}
}
