package runtime

import (
	"testing"
	"time"

	"github.com/agentstation/starmap/pkg/catalogs"
	"github.com/agentstation/starmap/pkg/sources"
)

func TestAcceptedSourcesExcludeEmptyFailedObservation(t *testing.T) {
	r := openTestRuntime(t, WithStateDirectory(privateRuntimeDirectory(t)), WithCatalogSource("embedded"), WithAcquisitionSources(sources.ModelsDevHTTPID))
	empty, err := catalogs.NewEmpty().Build()
	if err != nil {
		t.Fatal(err)
	}
	observation, err := sources.NewObservation(sources.ModelsDevHTTPID, empty, sources.ObservationMetadata{
		ObservedAt: time.Now().UTC(), Revision: sources.Revision{Kind: sources.RevisionKindContentDigest},
		Completeness: sources.ObservationCompletenessPartial, Status: sources.ObservationStatusDegraded,
		Issues: []sources.ObservationIssue{{Scope: sources.ObservationIssueScopeSource, Code: sources.ObservationIssueCodeFetchFailed, Message: "fixture fetch failed"}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := r.PublishObservations(t.Context(), observation); err != nil {
		t.Fatal(err)
	}
	if actual := r.Status().AcceptedAcquisitionSources; len(actual) != 0 {
		t.Fatalf("failed empty source reported accepted: %v", actual)
	}
	matches := 0
	for _, receipt := range r.layers.buildEvidence.SourceObservations {
		if receipt.ObservationID == observation.ID {
			matches++
			if receipt != observation.Link() {
				t.Fatal("failure receipt changed in diagnostics")
			}
		}
	}
	if matches != 1 {
		t.Fatalf("failure receipt count = %d, want one", matches)
	}
}
