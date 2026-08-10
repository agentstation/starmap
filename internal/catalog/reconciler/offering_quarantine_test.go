package reconciler

import (
	"context"
	stderrors "errors"
	"testing"
	"time"

	"github.com/agentstation/starmap/pkg/catalogmeta"
	"github.com/agentstation/starmap/pkg/catalogs"
	"github.com/agentstation/starmap/pkg/errors"
	"github.com/agentstation/starmap/pkg/sources"
)

func TestReconciliationQuarantinesOnlyUnresolvedProviderOfferings(t *testing.T) {
	t.Parallel()

	baseline := quarantineBaseline(t)
	providerObservation := quarantineProviderObservation(t, map[catalogs.ProviderID][]string{
		"alpha": {"existing", "unresolved-z"},
		"zeta":  {"existing", "unresolved-a"},
	})

	reconcile, err := New(WithBaseline(baseline), WithProvenance(true))
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	providerEvidence := verifiedReviewObservation(t, sources.ProvidersID, providerObservation)
	result, err := reconcile.Sources(context.Background(), sources.ProvidersID, []sources.Observation{
		providerEvidence,
		verifiedReviewObservation(t, sources.EmbeddedCatalogID, baseline),
	})
	if err != nil {
		t.Fatalf("Sources: %v", err)
	}

	published, err := result.Catalog.Build()
	if err != nil {
		t.Fatalf("Build reconciled catalog: %v", err)
	}
	for _, providerID := range []catalogs.ProviderID{"alpha", "zeta"} {
		offering, err := published.Offering(providerID, "existing")
		if err != nil {
			t.Fatalf("Offering(%s/existing): %v", providerID, err)
		}
		if offering.DefinitionID != "author/existing" {
			t.Fatalf("Offering(%s/existing).DefinitionID = %q", providerID, offering.DefinitionID)
		}
	}
	for _, key := range []catalogs.OfferingKey{
		{ProviderID: "alpha", ProviderModelID: "unresolved-z"},
		{ProviderID: "zeta", ProviderModelID: "unresolved-a"},
	} {
		if _, err := published.Offering(key.ProviderID, key.ProviderModelID); err == nil {
			t.Fatalf("unresolved offering %s/%s was published", key.ProviderID, key.ProviderModelID)
		}
		if fields := published.Provenance().FindModel(key.ProviderID, string(key.ProviderModelID)); len(fields) != 0 {
			t.Fatalf("unresolved offering %s/%s retained provenance: %#v", key.ProviderID, key.ProviderModelID, fields)
		}
	}

	want := []catalogmeta.ReviewCandidate{
		{
			Code:                catalogmeta.ReviewCandidateUnresolvedModelReference,
			ProviderID:          "alpha",
			ProviderModelID:     "unresolved-z",
			SourceID:            providerEvidence.SourceID,
			SourceObservationID: providerEvidence.ID,
			SourceRevision:      providerEvidence.Revision,
			EvidenceChecksum:    providerEvidence.EvidenceChecksum,
			Reason:              unresolvedModelReferenceMessage,
		},
		{
			Code:                catalogmeta.ReviewCandidateUnresolvedModelReference,
			ProviderID:          "zeta",
			ProviderModelID:     "unresolved-a",
			SourceID:            providerEvidence.SourceID,
			SourceObservationID: providerEvidence.ID,
			SourceRevision:      providerEvidence.Revision,
			EvidenceChecksum:    providerEvidence.EvidenceChecksum,
			Reason:              unresolvedModelReferenceMessage,
		},
	}
	if len(result.ReviewCandidates) != len(want) {
		t.Fatalf("review candidates = %#v, want %#v", result.ReviewCandidates, want)
	}
	for index := range want {
		if result.ReviewCandidates[index] != want[index] {
			t.Fatalf("review candidate %d = %#v, want %#v", index, result.ReviewCandidates[index], want[index])
		}
	}
	if result.Metadata.Stats.ResourcesSkipped != len(want) {
		t.Fatalf("resources skipped = %d, want %d", result.Metadata.Stats.ResourcesSkipped, len(want))
	}
	if len(published.AuthoredModels()) != 1 {
		t.Fatalf("authored models = %d, want no invented provider authorship", len(published.AuthoredModels()))
	}
}

func TestReconciliationQuarantinesReferenceToMissingAuthoredModel(t *testing.T) {
	t.Parallel()

	baseline := quarantineBaseline(t)
	observationBuilder := catalogs.NewEmpty()
	if err := observationBuilder.SetProvider(catalogs.Provider{
		ID: "alpha", Name: "Alpha",
		Models: map[string]*catalogs.Model{
			"missing": {
				ID: "missing", Name: "Missing", ModelRef: "author/missing",
			},
		},
	}); err != nil {
		t.Fatalf("SetProvider: %v", err)
	}
	observation, err := catalogs.NewObservationCatalog(observationBuilder)
	if err != nil {
		t.Fatalf("NewObservationCatalog: %v", err)
	}

	reconcile, err := New(WithBaseline(baseline))
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	result, err := reconcile.Sources(context.Background(), sources.ProvidersID, []sources.Observation{
		verifiedReviewObservation(t, sources.ProvidersID, observation),
		verifiedReviewObservation(t, sources.EmbeddedCatalogID, baseline),
	})
	if err != nil {
		t.Fatalf("Sources: %v", err)
	}
	if len(result.ReviewCandidates) != 1 ||
		result.ReviewCandidates[0].ProviderModelID != "missing" {
		t.Fatalf("review candidates = %#v", result.ReviewCandidates)
	}
	if _, err := result.Catalog.Build(); err != nil {
		t.Fatalf("Build reconciled catalog: %v", err)
	}
}

func TestReconciliationQuarantinesUnresolvedOfferingOutsideFilteredModelSet(t *testing.T) {
	t.Parallel()

	baselineBuilder := catalogs.NewEmpty()
	author := catalogs.Author{ID: "author", Name: "Author"}
	if err := baselineBuilder.SetAuthor(author); err != nil {
		t.Fatalf("SetAuthor: %v", err)
	}
	if err := baselineBuilder.SetAuthorModel(author.ID, catalogs.Model{
		ID: "new-model", Name: "New Model", Authors: []catalogs.Author{author},
	}); err != nil {
		t.Fatalf("SetAuthorModel: %v", err)
	}
	if err := baselineBuilder.SetProvider(catalogs.Provider{ID: "alpha", Name: "Alpha"}); err != nil {
		t.Fatalf("SetProvider baseline: %v", err)
	}
	baseline, err := baselineBuilder.Build()
	if err != nil {
		t.Fatalf("Build baseline: %v", err)
	}

	primaryBuilder := catalogs.NewEmpty()
	if err := primaryBuilder.SetProvider(catalogs.Provider{ID: "alpha", Name: "Alpha"}); err != nil {
		t.Fatalf("SetProvider primary: %v", err)
	}
	primary, err := catalogs.NewObservationCatalog(primaryBuilder)
	if err != nil {
		t.Fatalf("Build primary observation: %v", err)
	}
	enrichmentBuilder := catalogs.NewEmpty()
	if err := enrichmentBuilder.SetProvider(catalogs.Provider{
		ID: "alpha", Name: "Alpha",
		Models: map[string]*catalogs.Model{
			"new-model": {ID: "new-model", Name: "New Model"},
		},
	}); err != nil {
		t.Fatalf("SetProvider enrichment: %v", err)
	}
	enrichment, err := catalogs.NewObservationCatalog(enrichmentBuilder)
	if err != nil {
		t.Fatalf("Build enrichment observation: %v", err)
	}

	reconcile, err := New(WithBaseline(baseline))
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	result, err := reconcile.Sources(context.Background(), sources.ProvidersID, []sources.Observation{
		verifiedReviewObservation(t, sources.ProvidersID, primary),
		verifiedReviewObservation(t, sources.ModelsDevHTTPID, enrichment),
		verifiedReviewObservation(t, sources.EmbeddedCatalogID, baseline),
	})
	if err != nil {
		t.Fatalf("Sources: %v", err)
	}
	if len(result.ReviewCandidates) != 1 ||
		result.ReviewCandidates[0].ProviderID != "alpha" ||
		result.ReviewCandidates[0].ProviderModelID != "new-model" {
		t.Fatalf("review candidates = %#v", result.ReviewCandidates)
	}
	if _, err := result.Catalog.ProviderModel("alpha", "new-model"); err == nil {
		t.Fatal("unresolved offering outside the filtered model set was published")
	}
}

func TestReconciliationCarriesLastKnownGoodModelReference(t *testing.T) {
	t.Parallel()

	baseline := &catalogs.Model{ID: "existing", ModelRef: "author/existing"}
	observation := &catalogs.Model{ID: "existing", Name: "Current provider facts"}
	resolved, issues, err := resolvableProviderModels(
		"provider",
		[]*catalogs.Model{observation},
		map[catalogs.ModelDefinitionID]struct{}{"author/existing": {}},
		map[string]*catalogs.Model{"existing": baseline},
	)
	if err != nil {
		t.Fatalf("resolvableProviderModels: %v", err)
	}
	if len(issues) != 0 {
		t.Fatalf("issues = %#v, want none", issues)
	}
	if len(resolved) != 1 || resolved[0].ModelRef != baseline.ModelRef {
		t.Fatalf("resolved models = %#v, want carried model reference %q", resolved, baseline.ModelRef)
	}
	if resolved[0] == observation {
		t.Fatal("last-known-good identity mutated the observed model")
	}
	if observation.ModelRef != "" {
		t.Fatalf("observation model reference = %q, want unchanged", observation.ModelRef)
	}
}

func TestReconciliationCandidateRecordsPriorReviewedModelLink(t *testing.T) {
	_, candidates, err := resolvableProviderModels(
		"provider",
		[]*catalogs.Model{{ID: "renamed-model", Name: "Renamed Model"}},
		map[catalogs.ModelDefinitionID]struct{}{},
		map[string]*catalogs.Model{
			"renamed-model": {ID: "renamed-model", ModelRef: "author/prior-model"},
		},
	)
	if err != nil {
		t.Fatalf("resolvableProviderModels: %v", err)
	}
	if len(candidates) != 1 || candidates[0].PriorReviewedModelLink != "author/prior-model" {
		t.Fatalf("review candidates = %#v", candidates)
	}
}

func TestReconciliationRejectsMalformedExplicitModelReference(t *testing.T) {
	t.Parallel()

	_, _, err := resolvableProviderModels(
		"provider",
		[]*catalogs.Model{{ID: "model", ModelRef: "malformed"}},
		map[catalogs.ModelDefinitionID]struct{}{},
		nil,
	)
	var validationErr *errors.ValidationError
	if !stderrors.As(err, &validationErr) {
		t.Fatalf("error = %T %v, want ValidationError", err, err)
	}
}

func quarantineBaseline(t testing.TB) *catalogs.Catalog {
	t.Helper()
	builder := catalogs.NewEmpty()
	author := catalogs.Author{ID: "author", Name: "Author"}
	if err := builder.SetAuthor(author); err != nil {
		t.Fatalf("SetAuthor: %v", err)
	}
	if err := builder.SetAuthorModel(author.ID, catalogs.Model{
		ID: "existing", Name: "Existing", Authors: []catalogs.Author{author},
	}); err != nil {
		t.Fatalf("SetAuthorModel: %v", err)
	}
	for _, providerID := range []catalogs.ProviderID{"alpha", "zeta"} {
		if err := builder.SetProvider(catalogs.Provider{
			ID: providerID, Name: string(providerID),
			Models: map[string]*catalogs.Model{
				"existing": {
					ID: "existing", Name: "Existing", ModelRef: "author/existing",
				},
			},
		}); err != nil {
			t.Fatalf("SetProvider(%s): %v", providerID, err)
		}
	}
	baseline, err := builder.Build()
	if err != nil {
		t.Fatalf("Build baseline: %v", err)
	}
	return baseline
}

func quarantineProviderObservation(
	t testing.TB,
	providerModels map[catalogs.ProviderID][]string,
) *catalogs.Catalog {
	t.Helper()
	builder := catalogs.NewEmpty()
	for providerID, modelIDs := range providerModels {
		models := make(map[string]*catalogs.Model, len(modelIDs))
		for _, modelID := range modelIDs {
			models[modelID] = &catalogs.Model{ID: modelID, Name: modelID}
		}
		if err := builder.SetProvider(catalogs.Provider{
			ID: providerID, Name: string(providerID), Models: models,
		}); err != nil {
			t.Fatalf("SetProvider(%s): %v", providerID, err)
		}
	}
	observation, err := catalogs.NewObservationCatalog(builder)
	if err != nil {
		t.Fatalf("NewObservationCatalog: %v", err)
	}
	return observation
}

func verifiedReviewObservation(
	t testing.TB,
	sourceID sources.ID,
	catalog *catalogs.Catalog,
) sources.Observation {
	t.Helper()
	observation, err := sources.NewObservation(sourceID, catalog, sources.ObservationMetadata{
		ObservedAt:   time.Date(2026, time.August, 10, 12, 0, 0, 0, time.UTC),
		Revision:     sources.Revision{Kind: sources.RevisionKindContentDigest},
		Completeness: sources.ObservationCompletenessComplete,
		Status:       sources.ObservationStatusSucceeded,
	})
	if err != nil {
		t.Fatalf("NewObservation(%s): %v", sourceID, err)
	}
	return observation
}
