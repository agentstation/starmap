package reconciler

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/agentstation/starmap/pkg/catalogs"
	"github.com/agentstation/starmap/pkg/provenance"
	"github.com/agentstation/starmap/pkg/sources"
)

// TestF005CharacterizationPrimaryOmissionPrunesBaselineModel pins wholesale
// provider replacement after a complete primary observation. P4.7 must invert
// this expectation: absence alone is not explicit lifecycle evidence.
func TestF005CharacterizationPrimaryOmissionPrunesBaselineModel(t *testing.T) {
	baseline := characterizationCatalog(t, "provider", "observed", "omitted")
	primary := characterizationCatalog(t, "provider", "observed")
	observation := characterizationObservation(t, primary, false)

	result := characterizationReconcile(t, baseline, observation)
	provider, err := result.Catalog.Provider("provider")
	if err != nil {
		t.Fatalf("Provider: %v", err)
	}
	if _, exists := provider.Models["omitted"]; exists {
		t.Fatal("F-005 characterization changed: omitted baseline model survived provider replacement")
	}
}

// TestF005CharacterizationDegradedObservationStillPrunesBaselineModel pins the
// current failure to consume observation status, completeness, and issues.
// P4.7 must preserve last-known-good data when an observation is partial or
// degraded.
func TestF005CharacterizationDegradedObservationStillPrunesBaselineModel(t *testing.T) {
	baseline := characterizationCatalog(t, "provider", "observed", "rejected-sibling")
	partial := characterizationCatalog(t, "provider", "observed")
	observation := characterizationObservation(t, partial, true)

	result := characterizationReconcile(t, baseline, observation)
	provider, err := result.Catalog.Provider("provider")
	if err != nil {
		t.Fatalf("Provider: %v", err)
	}
	if _, exists := provider.Models["rejected-sibling"]; exists {
		t.Fatal("F-005 characterization changed: degraded omission preserved last-known-good model")
	}
}

// TestF007CharacterizationPersistedProvenanceCollidesAcrossProviders pins the
// bare-model-ID tracker key. Although Result.Provenance is scoped, the
// provenance merged into the catalog combines both provider offerings and the
// report selects whichever entry has the later reconciliation timestamp.
// P4.4 must make provider/model identity part of the durable tracker key.
func TestF007CharacterizationPersistedProvenanceCollidesAcrossProviders(t *testing.T) {
	source := catalogs.NewEmpty()
	for _, providerID := range []catalogs.ProviderID{"provider-a", "provider-b"} {
		model := catalogs.Model{
			ID:   "shared",
			Name: strings.ToUpper(string(providerID)),
			Limits: &catalogs.ModelLimits{
				ContextWindow: 8192,
			},
		}
		if err := source.SetProvider(catalogs.Provider{
			ID:   providerID,
			Name: string(providerID),
			Models: map[string]*catalogs.Model{
				model.ID: &model,
			},
		}); err != nil {
			t.Fatalf("SetProvider(%s): %v", providerID, err)
		}
	}
	sourceCatalog, err := source.Build()
	if err != nil {
		t.Fatalf("Build source: %v", err)
	}
	observation := characterizationObservation(t, sourceCatalog, false)

	reconcile, err := New()
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	result, err := reconcile.Sources(context.Background(), sources.ProvidersID, []sources.Observation{observation})
	if err != nil {
		t.Fatalf("Sources: %v", err)
	}

	persisted := result.Catalog.Provenance().Map()
	const collidedKey = "model:shared:Name"
	entries := persisted[collidedKey]
	if len(entries) != 2 {
		t.Fatalf("F-007 characterization changed: %q entries = %d, want 2 in one unscoped key; map=%#v", collidedKey, len(entries), persisted)
	}
	for key := range persisted {
		if strings.Contains(key, "provider-a") || strings.Contains(key, "provider-b") {
			t.Fatalf("F-007 characterization changed: persisted provenance unexpectedly provider-scoped at %q", key)
		}
	}

	report := provenance.GenerateReport(persisted)
	field, ok := report.Resources["model:shared"].Fields["Name"]
	if !ok {
		t.Fatalf("collided report field missing: %#v", report.Resources)
	}
	if len(field.History) != 2 {
		t.Fatalf("collided report history = %d, want 2", len(field.History))
	}
	if field.Current.Timestamp != field.History[0].Timestamp {
		t.Fatalf("report current timestamp = %s, newest history = %s", field.Current.Timestamp, field.History[0].Timestamp)
	}
	if field.History[0].Timestamp.Before(field.History[1].Timestamp) {
		t.Fatalf("report history is not timestamp-selected: %#v", field.History)
	}
}

func characterizationCatalog(t testing.TB, providerID catalogs.ProviderID, modelIDs ...string) *catalogs.Catalog {
	t.Helper()
	builder := catalogs.NewEmpty()
	models := make(map[string]*catalogs.Model, len(modelIDs))
	for _, modelID := range modelIDs {
		model := catalogs.Model{
			ID:   modelID,
			Name: modelID,
			Limits: &catalogs.ModelLimits{
				ContextWindow: 8192,
			},
		}
		models[modelID] = &model
	}
	if err := builder.SetProvider(catalogs.Provider{
		ID:     providerID,
		Name:   string(providerID),
		Models: models,
	}); err != nil {
		t.Fatalf("SetProvider: %v", err)
	}
	catalog, err := builder.Build()
	if err != nil {
		t.Fatalf("Build: %v", err)
	}
	return catalog
}

func characterizationObservation(t testing.TB, catalog *catalogs.Catalog, degraded bool) sources.Observation {
	t.Helper()
	metadata := sources.ObservationMetadata{
		ObservedAt:   time.Date(2026, time.July, 27, 12, 0, 0, 0, time.UTC),
		Revision:     sources.Revision{Kind: sources.RevisionKindContentDigest},
		Completeness: sources.ObservationCompletenessComplete,
		Status:       sources.ObservationStatusSucceeded,
		Records:      sources.ObservationRecordCounts{Accepted: 1},
	}
	if degraded {
		metadata.Completeness = sources.ObservationCompletenessPartial
		metadata.Status = sources.ObservationStatusDegraded
		metadata.Records.Rejected = 1
		metadata.Issues = []sources.ObservationIssue{{
			Scope:   sources.ObservationIssueScopeRecord,
			Code:    sources.ObservationIssueCodeInvalidRecord,
			Subject: "rejected-sibling",
			Message: "characterization fixture",
		}}
	}
	observation, err := sources.NewObservation(sources.ProvidersID, catalog, metadata)
	if err != nil {
		t.Fatalf("NewObservation: %v", err)
	}
	return observation
}

func characterizationReconcile(t testing.TB, baseline *catalogs.Catalog, observation sources.Observation) *Result {
	t.Helper()
	reconcile, err := New(WithBaseline(baseline))
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	result, err := reconcile.Sources(context.Background(), sources.ProvidersID, []sources.Observation{observation})
	if err != nil {
		t.Fatalf("Sources: %v", err)
	}
	return result
}
