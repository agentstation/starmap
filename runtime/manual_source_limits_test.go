package runtime

import (
	"fmt"
	"github.com/agentstation/starmap/pkg/catalogs"
	"github.com/agentstation/starmap/pkg/catalogs/storage"
	"github.com/agentstation/starmap/pkg/sources"
	"testing"
	"time"
)

func TestManualSourceProviderLimitSurvivesPublicationAndRestart(t *testing.T) {
	store := storage.NewMemory()
	connected, options := manualTestRuntime(t, store)
	baseline, err := store.Current(t.Context())
	if err != nil || len(baseline.Manifest.SourceObservations) != 1 {
		t.Fatalf("baseline receipt: generation=%+v error=%v", baseline.Manifest, err)
	}
	builder := catalogs.NewEmpty()
	for index := range 101 {
		id := catalogs.ProviderID(fmt.Sprintf("metadata-%03d", index))
		if index == 0 {
			id = "baseline-provider"
		}
		if err := builder.SetProvider(catalogs.Provider{ID: id, Name: "Metadata provider"}); err != nil {
			t.Fatal(err)
		}
	}
	candidate, err := catalogs.NewObservationCatalog(builder)
	if err != nil {
		t.Fatal(err)
	}
	observation, err := sources.NewObservation(sources.ModelsDevHTTPID, candidate, sources.ObservationMetadata{ObservedAt: time.Date(2026, 9, 7, 1, 0, 0, 0, time.UTC), Revision: sources.Revision{Kind: sources.RevisionKindContentDigest}, Completeness: sources.ObservationCompletenessComplete, Status: sources.ObservationStatusSucceeded, Records: sources.ObservationRecordCounts{Accepted: 101}})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := connected.PublishObservations(t.Context(), observation); err != nil {
		t.Fatalf("retain bounded source: %v", err)
	}
	assertAccepted := func() {
		t.Helper()
		state := connected.State()
		if len(state.Catalog.Providers().List()) != 1 {
			t.Fatal("metadata input expanded canonical provider membership")
		}
		generation, err := store.Current(t.Context())
		if err != nil {
			t.Fatal(err)
		}
		if _, err := catalogs.DecodeCatalogPayload(generation.Payload); err != nil {
			t.Fatalf("accepted generation failed canonical validation: %v", err)
		}
		assertExactSourceReceipts(t, generation.Manifest.SourceObservations,
			baseline.Manifest.SourceObservations[0], observation.Link())
	}
	assertAccepted()
	before := connected.State()
	if err := connected.Close(); err != nil {
		t.Fatal(err)
	}
	connected = openTestRuntime(t, options...)
	assertAccepted()
	if connected.State().GenerationID != before.GenerationID || connected.State().PayloadChecksum != before.PayloadChecksum {
		t.Fatal("restart changed the accepted generation")
	}
}
