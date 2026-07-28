package pipeline

import (
	"testing"
	"time"

	"github.com/agentstation/starmap/pkg/catalogs"
	"github.com/agentstation/starmap/pkg/provenance"
	"github.com/agentstation/starmap/pkg/sources"
)

func TestObservationVolumeRegressionBecomesDegradedWithoutInventingDeletion(t *testing.T) {
	baselineBuilder := catalogs.NewEmpty()
	if err := baselineBuilder.SetProvider(catalogs.Provider{
		ID:   "provider",
		Name: "Provider",
		Models: map[string]*catalogs.Model{
			"present": {ID: "present", Name: "Present"},
			"omitted": {ID: "omitted", Name: "Omitted"},
			"manual":  {ID: "manual", Name: "Manual"},
		},
	}); err != nil {
		t.Fatalf("SetProvider baseline: %v", err)
	}
	observedAt := time.Date(2026, time.July, 28, 12, 0, 0, 0, time.UTC)
	baselineBuilder.SetProvenance(provenance.Map{
		"model:" + provenance.ModelResourceID("provider", "present") + ":Name": {{
			Source: sources.ProvidersID, Timestamp: observedAt.Add(-2 * time.Hour),
		}},
		"model:" + provenance.ModelResourceID("provider", "omitted") + ":Name": {{
			Source: sources.ProvidersID, Timestamp: observedAt.Add(-2 * time.Hour),
		}},
		"model:" + provenance.ModelResourceID("provider", "manual") + ":Name": {{
			Source: sources.LocalCatalogID, Timestamp: observedAt.Add(-time.Hour),
		}},
	})
	baseline, err := baselineBuilder.Build()
	if err != nil {
		t.Fatalf("Build baseline: %v", err)
	}

	currentBuilder := catalogs.NewEmpty()
	if err := currentBuilder.SetProvider(catalogs.Provider{
		ID:   "provider",
		Name: "Provider",
		Models: map[string]*catalogs.Model{
			"present": {ID: "present", Name: "Present"},
		},
	}); err != nil {
		t.Fatalf("SetProvider current: %v", err)
	}
	current, err := currentBuilder.Build()
	if err != nil {
		t.Fatalf("Build current: %v", err)
	}
	observation, err := sources.NewObservation(sources.ProvidersID, current, sources.ObservationMetadata{
		ObservedAt:   observedAt,
		Revision:     sources.Revision{Kind: sources.RevisionKindContentDigest},
		Completeness: sources.ObservationCompletenessComplete,
		Status:       sources.ObservationStatusSucceeded,
		Records:      sources.ObservationRecordCounts{Accepted: 1},
	})
	if err != nil {
		t.Fatalf("NewObservation: %v", err)
	}

	guarded, err := guardObservationHealth(baseline, []sources.Observation{observation})
	if err != nil {
		t.Fatalf("guardObservationHealth: %v", err)
	}
	if len(guarded) != 1 {
		t.Fatalf("guarded observations = %d, want 1", len(guarded))
	}
	got := guarded[0]
	if got.Status != sources.ObservationStatusDegraded ||
		got.Completeness != sources.ObservationCompletenessPartial {
		t.Fatalf("health = %q/%q, want degraded/partial", got.Status, got.Completeness)
	}
	if len(got.Issues) != 1 ||
		got.Issues[0].Code != sources.ObservationIssueCodeVolumeCollapse ||
		got.Issues[0].Subject != "provider" {
		t.Fatalf("volume issues = %#v", got.Issues)
	}
	if got.ID == observation.ID {
		t.Fatal("health classification did not produce a distinct observation identity")
	}
	if err := requireHealthyObservations(
		[]sources.Source{&lifecycleTestSource{id: sources.ProvidersID}},
		guarded,
	); err == nil {
		t.Fatal("require-all accepted a volume-regressed observation")
	}
}
