package acquisition

import (
	"testing"
	"time"

	"github.com/agentstation/starmap"
	"github.com/agentstation/starmap/pkg/catalogs"
	"github.com/agentstation/starmap/pkg/sources"
)

// fixtureCatalogEvidence identifies an explicit replacement baseline in acquisition tests.
func fixtureCatalogEvidence(t testing.TB, catalog *catalogs.Catalog) starmap.CandidateEvidence {
	t.Helper()
	observation, err := sources.NewObservation(sources.LocalCatalogID, catalog, sources.ObservationMetadata{
		ObservedAt:   time.Now().UTC(),
		Revision:     sources.Revision{Kind: sources.RevisionKindContentDigest},
		Completeness: sources.ObservationCompletenessComplete,
		Status:       sources.ObservationStatusSucceeded,
	})
	if err != nil {
		t.Fatal(err)
	}
	return starmap.CandidateEvidence{SourceObservations: []catalogs.SourceObservationLink{observation.Link()}}
}

// activateFixtureBaseline installs explicit operator state before acquisition starts.
func activateFixtureBaseline(t testing.TB, client *starmap.Client, catalog *catalogs.Catalog) {
	t.Helper()
	candidate, err := starmap.NewCandidate(catalog, starmap.CandidateEvidence{})
	if err != nil {
		t.Fatal(err)
	}
	generation, err := client.PrepareGeneration(t.Context(), candidate)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := client.Activate(t.Context(), generation); err != nil {
		t.Fatal(err)
	}
}
