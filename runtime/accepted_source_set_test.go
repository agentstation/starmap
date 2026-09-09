package runtime

import (
	"slices"
	"testing"

	"github.com/agentstation/starmap/pkg/catalogs"
	"github.com/agentstation/starmap/pkg/sources"
)

func TestAcceptedSourceSetExcludesDistributionAndDuplicates(t *testing.T) {
	empty, err := catalogs.NewEmpty().Build()
	if err != nil {
		t.Fatal(err)
	}
	var actual []sources.ID
	for _, source := range []sources.ID{sources.EmbeddedCatalogID, sources.ReleaseArtifactID, sources.ProvidersID, sources.LocalCatalogID, sources.LocalCatalogID, "unknown"} {
		actual = appendAcceptedSource(actual, sources.Observation{SourceID: source, Catalog: empty, Status: sources.ObservationStatusSucceeded, Completeness: sources.ObservationCompletenessComplete})
	}
	partial := acquisitionSourceObservation(t, false)
	partial.SourceID = sources.ModelsDevHTTPID
	partial.Status = sources.ObservationStatusDegraded
	partial.Completeness = sources.ObservationCompletenessPartial
	actual = appendAcceptedSource(actual, partial)
	if !slices.Equal(actual, []sources.ID{sources.LocalCatalogID, sources.ModelsDevHTTPID, sources.ProvidersID}) {
		t.Fatalf("accepted sources=%v", actual)
	}
}
