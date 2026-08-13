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

// TestF005CompletePrimaryOmissionPreservesBaselineModel proves that absence
// alone is not explicit lifecycle evidence.
func TestF005CompletePrimaryOmissionPreservesBaselineModel(t *testing.T) {
	baseline := characterizationCatalog(t, "provider", "observed", "omitted")
	baseline = withCharacterizationProvenance(t, baseline, "provider", "omitted", sources.ProvidersID)
	primary := characterizationCatalog(t, "provider", "observed")
	observation := characterizationObservation(t, primary, false)

	result := characterizationReconcile(t, baseline, observation)
	provider, err := result.Catalog.Provider("provider")
	if err != nil {
		t.Fatalf("Provider: %v", err)
	}
	if _, exists := provider.Models["omitted"]; !exists {
		t.Fatal("complete source omission deleted last-known-good baseline model")
	}
	entries := result.Catalog.Provenance().FindModelField("provider", "omitted", "Name")
	if len(entries) != 1 || entries[0].Source != sources.ProvidersID {
		t.Fatalf("retained model provenance = %#v, want exact provider evidence", entries)
	}
}

// TestF005DegradedObservationPreservesBaselineModel proves partial record
// rejection cannot delete its valid last-known-good sibling.
func TestF005DegradedObservationPreservesBaselineModel(t *testing.T) {
	baseline := characterizationCatalog(t, "provider", "observed", "rejected-sibling")
	partial := characterizationCatalog(t, "provider", "observed")
	observation := characterizationObservation(t, partial, true)

	result := characterizationReconcile(t, baseline, observation)
	provider, err := result.Catalog.Provider("provider")
	if err != nil {
		t.Fatalf("Provider: %v", err)
	}
	if _, exists := provider.Models["rejected-sibling"]; !exists {
		t.Fatal("degraded source omission deleted last-known-good baseline model")
	}
	observedEvidence := result.Catalog.Provenance().FindModelField("provider", "observed", "Name")
	if len(observedEvidence) != 1 ||
		!strings.Contains(observedEvidence[0].Reason, "status=degraded") ||
		!strings.Contains(observedEvidence[0].Reason, "rejected=1") ||
		!strings.Contains(observedEvidence[0].Reason, "issues=invalid_record:1") {
		t.Fatalf("present degraded field lacks health evidence: %#v", observedEvidence)
	}
}

func TestF005StaleFallbackCannotRegressKnownBaselineFacts(t *testing.T) {
	baseline := characterizationCatalog(t, "provider", "model")
	baselineBuilder, err := catalogs.NewBuilderFrom(baseline)
	if err != nil {
		t.Fatalf("NewBuilderFrom: %v", err)
	}
	baselineProvider, err := baselineBuilder.Provider("provider")
	if err != nil {
		t.Fatalf("baseline Provider: %v", err)
	}
	baselineProvider.Name = "Current Provider"
	baselineProvider.Models["model"].Name = "Current Model"
	if err := baselineBuilder.SetProvider(baselineProvider); err != nil {
		t.Fatalf("SetProvider baseline: %v", err)
	}
	baseline, err = catalogs.NewObservationCatalog(baselineBuilder)
	if err != nil {
		t.Fatalf("Build baseline observation: %v", err)
	}

	staleBuilder := catalogs.NewEmpty()
	if err := staleBuilder.SetProvider(catalogs.Provider{
		ID:   "provider",
		Name: "Stale Provider",
		Models: map[string]*catalogs.Model{
			"model": {ID: "model", Name: "Stale Model"},
		},
	}); err != nil {
		t.Fatalf("SetProvider stale: %v", err)
	}
	stale, err := catalogs.NewObservationCatalog(staleBuilder)
	if err != nil {
		t.Fatalf("Build stale observation: %v", err)
	}
	observation, err := sources.NewObservation(sources.ProvidersID, stale, sources.ObservationMetadata{
		ObservedAt:   time.Date(2026, time.July, 27, 12, 0, 0, 0, time.UTC),
		Revision:     sources.Revision{Kind: sources.RevisionKindContentDigest},
		Completeness: sources.ObservationCompletenessComplete,
		Status:       sources.ObservationStatusDegraded,
		Records:      sources.ObservationRecordCounts{Accepted: 1},
		Issues: []sources.ObservationIssue{{
			Scope:   sources.ObservationIssueScopeStaleFallback,
			Code:    sources.ObservationIssueCodeStaleFallback,
			Message: "cached fallback",
		}},
	})
	if err != nil {
		t.Fatalf("NewObservation: %v", err)
	}
	result := characterizationReconcile(t, baseline, observation)
	provider, err := result.Catalog.Provider("provider")
	if err != nil {
		t.Fatalf("Provider: %v", err)
	}
	if provider.Name != "Current Provider" || provider.Models["model"].Name != "Current Model" {
		t.Fatalf("stale fallback regressed last-known-good facts: %#v", provider)
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
			ID:       "shared",
			ModelRef: "test-author/shared",
			Name:     fixture.name,
			Status:   fixture.status,
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
	author := catalogs.Author{ID: "test-author", Name: "Test Author"}
	if err := source.SetAuthor(author); err != nil {
		t.Fatalf("SetAuthor: %v", err)
	}
	if err := source.SetAuthorModel(author.ID, catalogs.Model{
		ID:      "shared",
		Name:    "Shared",
		Authors: []catalogs.Author{author},
	}); err != nil {
		t.Fatalf("SetAuthorModel: %v", err)
	}
	sourceCatalog, err := source.Build()
	if err != nil {
		t.Fatalf("Build source catalog: %v", err)
	}
	observation := characterizationObservation(t, sourceCatalog, false)

	reconcile, err := New(WithBaseline(sourceCatalog))
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

	payload, err := catalogs.EncodeCatalogPayload(result.Catalog)
	if err != nil {
		t.Fatalf("EncodeCatalogPayload: %v", err)
	}
	decoded, err := catalogs.DecodeCatalogPayload(payload)
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
	author := catalogs.Author{ID: "test-author", Name: "Test Author"}
	if err := builder.SetAuthor(author); err != nil {
		t.Fatalf("SetAuthor: %v", err)
	}
	models := make(map[string]*catalogs.Model, len(modelIDs))
	for _, modelID := range modelIDs {
		model := catalogs.Model{
			ID:       modelID,
			ModelRef: catalogs.ModelDefinitionID(string(author.ID) + "/" + modelID),
			Name:     modelID,
			Limits: &catalogs.ModelLimits{
				ContextWindow: 8192,
			},
		}
		models[modelID] = &model
		if err := builder.SetAuthorModel(author.ID, catalogs.Model{
			ID: modelID, Name: modelID, Authors: []catalogs.Author{author},
		}); err != nil {
			t.Fatalf("SetAuthorModel(%s): %v", modelID, err)
		}
	}
	if err := builder.SetProvider(catalogs.Provider{
		ID:     providerID,
		Name:   string(providerID),
		Models: models,
	}); err != nil {
		t.Fatalf("SetProvider: %v", err)
	}
	catalog, err := catalogs.NewObservationCatalog(builder)
	if err != nil {
		t.Fatalf("Build observation: %v", err)
	}
	return catalog
}

func withCharacterizationProvenance(
	t testing.TB,
	catalog *catalogs.Catalog,
	providerID catalogs.ProviderID,
	modelID string,
	source sources.ID,
) *catalogs.Catalog {
	t.Helper()
	builder, err := catalogs.NewBuilderFrom(catalog)
	if err != nil {
		t.Fatalf("NewBuilderFrom: %v", err)
	}
	builder.SetProvenance(provenance.Map{
		"model:" + provenance.ModelResourceID(string(providerID), modelID) + ":Name": {{
			Source:    source,
			Field:     "Name",
			Value:     modelID,
			Timestamp: time.Date(2026, time.July, 26, 12, 0, 0, 0, time.UTC),
		}},
	})
	updated, err := catalogs.NewObservationCatalog(builder)
	if err != nil {
		t.Fatalf("Build provenance observation catalog: %v", err)
	}
	return updated
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
