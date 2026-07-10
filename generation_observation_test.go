package starmap

import (
	"testing"
	"time"

	"github.com/agentstation/starmap/pkg/catalogs"
	"github.com/agentstation/starmap/pkg/sources"
)

func TestGenerationLinksExactSourceObservationMetadata(t *testing.T) {
	observedAt := time.Date(2026, time.July, 9, 12, 0, 0, 0, time.UTC)
	sourceCatalog, err := catalogs.NewEmpty().Build()
	if err != nil {
		t.Fatalf("Build source catalog: %v", err)
	}
	observation, err := sources.NewObservation(sources.ModelsDevHTTPID, sourceCatalog, sources.ObservationMetadata{
		ObservedAt:   observedAt,
		Revision:     sources.Revision{Kind: sources.RevisionKindETag, Value: `"models-dev-v1"`},
		Completeness: sources.ObservationCompletenessComplete,
		Status:       sources.ObservationStatusSucceeded,
	})
	if err != nil {
		t.Fatalf("NewObservation: %v", err)
	}

	publishedBuilder := catalogs.NewEmpty()
	if err := publishedBuilder.SetProvider(catalogs.Provider{ID: "published", Name: "Published"}); err != nil {
		t.Fatalf("SetProvider: %v", err)
	}
	published, err := publishedBuilder.Build()
	if err != nil {
		t.Fatalf("Build published catalog: %v", err)
	}
	client := generationTestClient(observedAt)
	generation, err := client.newGeneration(published, []sources.Observation{observation})
	if err != nil {
		t.Fatalf("newGeneration: %v", err)
	}

	if len(generation.Manifest.SourceObservations) != 1 {
		t.Fatalf("source observations = %#v", generation.Manifest.SourceObservations)
	}
	link := generation.Manifest.SourceObservations[0]
	if link.Source != observation.SourceID || link.ObservationID != observation.ID || link.EvidenceChecksum != observation.EvidenceChecksum {
		t.Fatalf("source observation link = %#v, want %#v", link, observation)
	}
	if !link.ObservedAt.Equal(observation.ObservedAt) || link.Revision != observation.Revision ||
		link.Completeness != observation.Completeness || link.Status != observation.Status {
		t.Fatalf("source observation metadata = %#v, want %#v", link, observation)
	}
	if link.EvidenceChecksum == generation.Manifest.Payload.Checksum {
		t.Fatal("source evidence checksum incorrectly aliases the reconciled generation payload checksum")
	}
}

func TestGenerationDerivesDegradedCompletenessFromObservations(t *testing.T) {
	observedAt := time.Date(2026, time.July, 9, 12, 0, 0, 0, time.UTC)
	catalog, err := catalogs.NewEmpty().Build()
	if err != nil {
		t.Fatalf("Build: %v", err)
	}
	observation, err := sources.NewObservation(sources.ProvidersID, catalog, sources.ObservationMetadata{
		ObservedAt:   observedAt,
		Revision:     sources.Revision{Kind: sources.RevisionKindContentDigest},
		Completeness: sources.ObservationCompletenessPartial,
		Status:       sources.ObservationStatusDegraded,
		Issues: []sources.ObservationIssue{{
			Scope: sources.ObservationIssueScopeSource, Code: sources.ObservationIssueCodeFetchFailed, Message: "partial source",
		}},
	})
	if err != nil {
		t.Fatalf("NewObservation: %v", err)
	}
	generation, err := generationTestClient(observedAt).newGeneration(catalog, []sources.Observation{observation})
	if err != nil {
		t.Fatalf("newGeneration: %v", err)
	}
	if generation.Manifest.Completeness != catalogs.GenerationCompletenessPartial || !generation.Manifest.Degraded {
		t.Fatalf("generation state = (%q, %t)", generation.Manifest.Completeness, generation.Manifest.Degraded)
	}
	if len(generation.Manifest.DegradationReasons) != 1 {
		t.Fatalf("degradation reasons = %#v", generation.Manifest.DegradationReasons)
	}
}

func generationTestClient(now time.Time) *Client {
	ids := []string{"generation-id", "sync-run-id"}
	return &Client{
		now: func() time.Time { return now },
		newID: func() (string, error) {
			id := ids[0]
			ids = ids[1:]
			return id, nil
		},
	}
}
