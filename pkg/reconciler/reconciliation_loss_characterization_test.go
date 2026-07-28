package reconciler

import (
	"context"
	"testing"
	"time"

	"github.com/agentstation/starmap/pkg/catalogs"
	"github.com/agentstation/starmap/pkg/catalogstore"
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

// TestF007PersistedProvenanceIsProviderModelScoped inverts the former
// characterization: every offering keeps independent durable evidence even
// when another provider uses the same opaque model ID.
func TestF007PersistedProvenanceIsProviderModelScoped(t *testing.T) {
	source := catalogs.NewEmpty()
	fixtures := []struct {
		providerID   catalogs.ProviderID
		name         string
		status       catalogs.ModelStatus
		contextLimit int64
		inputPrice   float64
	}{
		{providerID: "provider-a", name: "Provider A Shared", status: catalogs.ModelStatusActive, contextLimit: 8192, inputPrice: 1},
		{providerID: "provider-b", name: "Provider B Shared", status: catalogs.ModelStatusDeprecated, contextLimit: 16384, inputPrice: 2},
	}
	for _, fixture := range fixtures {
		model := catalogs.Model{
			ID:     "shared",
			Name:   fixture.name,
			Status: fixture.status,
			Limits: &catalogs.ModelLimits{
				ContextWindow: fixture.contextLimit,
			},
			Pricing: &catalogs.ModelPricing{
				Currency: catalogs.ModelPricingCurrencyUSD,
				Tokens: &catalogs.ModelTokenPricing{
					Input: &catalogs.ModelTokenCost{Per1M: fixture.inputPrice},
				},
			},
		}
		if err := source.SetProvider(catalogs.Provider{
			ID:   fixture.providerID,
			Name: string(fixture.providerID),
			Models: map[string]*catalogs.Model{
				model.ID: &model,
			},
		}); err != nil {
			t.Fatalf("SetProvider(%s): %v", fixture.providerID, err)
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
	if entries := persisted["model:shared:Name"]; len(entries) != 0 {
		t.Fatalf("bare model evidence survived: %#v", entries)
	}
	for _, fixture := range fixtures {
		fields := result.Catalog.Provenance().FindModel(fixture.providerID, "shared")
		for _, field := range []string{"Name", "Status", "limits.context_window", "pricing"} {
			entries := fields[field]
			if len(entries) != 1 {
				t.Fatalf("%s/shared %s evidence = %#v, want one independent entry", fixture.providerID, field, entries)
			}
		}
		if got := fields["Name"][0].Value; got != fixture.name {
			t.Fatalf("%s/shared name evidence = %#v, want %q", fixture.providerID, got, fixture.name)
		}
		if got := fields["Status"][0].Value; got != fixture.status {
			t.Fatalf("%s/shared status evidence = %#v, want %q", fixture.providerID, got, fixture.status)
		}
		if got := fields["limits.context_window"][0].Value; got != fixture.contextLimit {
			t.Fatalf("%s/shared limit evidence = %#v, want %d", fixture.providerID, got, fixture.contextLimit)
		}
	}

	report := provenance.GenerateReport(persisted)
	for _, fixture := range fixtures {
		resourceID := provenance.ModelResourceID(string(fixture.providerID), "shared")
		resource, ok := report.Resources["model:"+resourceID]
		if !ok || len(resource.Fields["Name"].History) != 1 {
			t.Fatalf("report resource %q = %#v", resourceID, resource)
		}
	}

	payload, err := catalogstore.EncodeCatalogPayload(result.Catalog)
	if err != nil {
		t.Fatalf("EncodeCatalogPayload: %v", err)
	}
	decoded, err := catalogstore.DecodeCatalogPayload(payload)
	if err != nil {
		t.Fatalf("DecodeCatalogPayload: %v", err)
	}
	for _, fixture := range fixtures {
		if fields := decoded.Provenance().FindModel(fixture.providerID, "shared"); len(fields) < 4 {
			t.Fatalf("decoded %s/shared evidence = %#v, want independent name/status/limit/pricing", fixture.providerID, fields)
		}
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
