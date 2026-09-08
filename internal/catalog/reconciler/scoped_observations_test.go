package reconciler

import (
	"context"
	stderrors "errors"
	"slices"
	"testing"
	"time"

	"github.com/agentstation/starmap/pkg/catalogs"
	"github.com/agentstation/starmap/pkg/errors"
	"github.com/agentstation/starmap/pkg/sources"
)

func TestScopedReconciliationKeepsHealthWithItsRecord(t *testing.T) {
	at := time.Date(2026, 9, 7, 0, 0, 0, 0, time.UTC)
	one := scopedReconciliationObservation(t, "one", "model-one", 1000, at)
	two := scopedReconciliationObservation(t, "two", "model-two", 2000, at.Add(time.Minute))
	stale, err := sources.NewObservation(two.SourceID, two.Catalog, sources.ObservationMetadata{
		ProviderBinding: two.ProviderBinding, ObservedAt: two.ObservedAt, Revision: two.Revision,
		Completeness: sources.ObservationCompletenessPartial, Status: sources.ObservationStatusDegraded,
		Records: two.Records, Issues: []sources.ObservationIssue{{Scope: sources.ObservationIssueScopeStaleFallback, Code: sources.ObservationIssueCodeStaleFallback, Message: "retained fixture after source failure"}},
	})
	if err != nil {
		t.Fatal(err)
	}
	builder := catalogs.NewEmpty()
	for _, observation := range []sources.Observation{one, two} {
		if err := builder.MergeWith(observation.Catalog, catalogs.WithStrategy(catalogs.MergeEnrichEmpty)); err != nil {
			t.Fatal(err)
		}
	}
	provider, _ := builder.Providers().Get("provider-a")
	for _, model := range provider.Models {
		model.Limits.ContextWindow = 500
	}
	if err := builder.SetProvider(*provider); err != nil {
		t.Fatal(err)
	}
	baseline, err := builder.Build()
	if err != nil {
		t.Fatal(err)
	}
	reconciler, err := New(WithBaseline(baseline))
	if err != nil {
		t.Fatal(err)
	}
	result, err := reconciler.Sources(t.Context(), sources.ProvidersID, []sources.Observation{one, stale})
	if err != nil {
		t.Fatal(err)
	}
	provider, _ = result.Catalog.Providers().Get("provider-a")
	if provider.Models["model-one"].Limits.ContextWindow != 1000 {
		t.Fatal("a healthy scope inherited a peer's stale-fallback status")
	}
	if provider.Models["model-two"].Limits.ContextWindow != 500 {
		t.Fatal("stale fallback replaced accepted baseline facts")
	}
}

func TestScopedReconciliationChecksCancellationBeforeReceipts(t *testing.T) {
	reconciler, err := New()
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	invalid := sources.Observation{ProviderBinding: &sources.ProviderAcquisitionBinding{}}
	if _, err := reconciler.Sources(ctx, sources.ProvidersID, []sources.Observation{invalid}); !stderrors.Is(err, context.Canceled) {
		t.Fatalf("cancellation was not checked first: %v", err)
	}
}

func TestScopedReconciliationPrefersFreshPeerToNewerStaleFallback(t *testing.T) {
	at := time.Date(2026, 9, 7, 0, 0, 0, 0, time.UTC)
	fresh := scopedReconciliationObservation(t, "fresh", "shared", 1000, at)
	cached := scopedReconciliationObservation(t, "cached", "shared", 2000, at.Add(time.Minute))
	stale, err := sources.NewObservation(cached.SourceID, cached.Catalog, sources.ObservationMetadata{
		ProviderBinding: cached.ProviderBinding, ObservedAt: cached.ObservedAt, Revision: cached.Revision,
		Completeness: sources.ObservationCompletenessPartial, Status: sources.ObservationStatusDegraded,
		Records: cached.Records, Issues: []sources.ObservationIssue{{Scope: sources.ObservationIssueScopeStaleFallback, Code: sources.ObservationIssueCodeStaleFallback, Message: "retained fixture after source failure"}},
	})
	if err != nil {
		t.Fatal(err)
	}
	reconciler, err := New(WithBaseline(scopedReconciliationBaseline(t, fresh)))
	if err != nil {
		t.Fatal(err)
	}
	result, err := reconciler.Sources(t.Context(), sources.ProvidersID, []sources.Observation{fresh, stale})
	if err != nil {
		t.Fatal(err)
	}
	provider, _ := result.Catalog.Providers().Get("provider-a")
	if provider.Models["shared"].Limits.ContextWindow != 1000 {
		t.Fatal("a newer stale fallback displaced a fresh peer observation")
	}
	assertModelEvidenceSource(t, result.Catalog, "limits.context_window", sources.ProvidersID, fresh.ID)
}

func scopedReconciliationObservation(t *testing.T, bindingID, modelID string, window int64, at time.Time) sources.Observation {
	t.Helper()
	catalog := sourceIdentityCatalog(t, "", catalogs.Model{ID: modelID, Name: modelID, Limits: &catalogs.ModelLimits{ContextWindow: window}})
	binding := sources.ProviderAcquisitionBinding{SchemaVersion: 1, ID: bindingID, Revision: "1", ProviderID: "provider-a",
		AccountID: bindingID, Region: "global", APISurface: "models.list", CredentialRole: sources.ProviderBindingCatalogAcquisition,
		CredentialProfileID: "unauthenticated"}
	observation, err := sources.NewObservation(sources.ProvidersID, catalog, sources.ObservationMetadata{
		ProviderBinding: &binding, ObservedAt: at, Revision: sources.Revision{Kind: sources.RevisionKindContentDigest},
		Completeness: sources.ObservationCompletenessComplete, Status: sources.ObservationStatusSucceeded,
		Records: sources.ObservationRecordCounts{Accepted: 1},
	})
	if err != nil {
		t.Fatal(err)
	}
	return observation
}

func TestScopedReconciliationRefusesAmbiguousScopesAndFacts(t *testing.T) {
	at := time.Date(2026, 9, 7, 0, 0, 0, 0, time.UTC)
	one := scopedReconciliationObservation(t, "one", "shared", 1000, at)
	different := scopedReconciliationObservation(t, "two", "shared", 2000, at)
	unscoped := sourceIdentityObservation(t, sources.ProvidersID, one.Catalog, at)
	for name, observations := range map[string][]sources.Observation{
		"same-time-conflict":     {one, different},
		"reversed-time-conflict": {different, one},
		"duplicate-binding":      {one, one},
		"mixed-authority":        {one, unscoped},
	} {
		t.Run(name, func(t *testing.T) {
			reconciler, err := New(WithBaseline(scopedReconciliationBaseline(t, one)))
			if err != nil {
				t.Fatal(err)
			}
			result, err := reconciler.Sources(t.Context(), sources.ProvidersID, observations)
			var conflict *errors.ConflictError
			if result != nil || !stderrors.As(err, &conflict) {
				t.Fatalf("ambiguous scope did not stop reconciliation: %v", err)
			}
		})
	}
	corrupted := one
	binding := *one.ProviderBinding
	binding.AccountID = "changed-account"
	corrupted.ProviderBinding = &binding
	reconciler, err := New()
	if err != nil {
		t.Fatal(err)
	}
	if result, err := reconciler.Sources(t.Context(), sources.ProvidersID, []sources.Observation{corrupted}); result != nil || err == nil {
		t.Fatal("reconciliation accepted an altered binding without a matching receipt")
	}
}

func TestScopedReconciliationAcceptsIdenticalSharedFactsDeterministically(t *testing.T) {
	at := time.Date(2026, 9, 7, 0, 0, 0, 0, time.UTC)
	one := scopedReconciliationObservation(t, "one", "shared", 1000, at)
	two := scopedReconciliationObservation(t, "two", "shared", 1000, at)
	var selected string
	for _, observations := range [][]sources.Observation{{one, two}, {two, one}} {
		originalIDs := []string{observations[0].ID, observations[1].ID}
		reconciler, err := New(WithBaseline(scopedReconciliationBaseline(t, one)))
		if err != nil {
			t.Fatal(err)
		}
		result, err := reconciler.Sources(t.Context(), sources.ProvidersID, observations)
		if err != nil {
			t.Fatal(err)
		}
		entries := result.Catalog.Provenance().FindModelField("provider-a", "shared", "limits.context_window")
		if len(entries) != 1 || entries[0].ObservationID != one.ID && entries[0].ObservationID != two.ID {
			t.Fatal("identical scoped facts lost their source receipt")
		}
		if selected == "" {
			selected = entries[0].ObservationID
		} else if entries[0].ObservationID != selected {
			t.Fatal("input order changed the selected receipt")
		}
		if observations[0].ID != originalIDs[0] || observations[1].ID != originalIDs[1] {
			t.Fatal("reconciliation changed the caller's observation order")
		}
	}
}

func TestScopedReconciliationPrimaryIncludesEveryProvider(t *testing.T) {
	at := time.Date(2026, 9, 7, 0, 0, 0, 0, time.UTC)
	one := scopedReconciliationObservation(t, "one", "shared", 1000, at)
	second := scopedReconciliationObservation(t, "two", "shared", 2000, at)
	builder, err := catalogs.NewBuilderFrom(second.Catalog)
	if err != nil {
		t.Fatal(err)
	}
	provider, _ := second.Catalog.Providers().Get("provider-a")
	provider.ID = "provider-b"
	if err := builder.DeleteProvider("provider-a"); err != nil {
		t.Fatal(err)
	}
	if err := builder.SetProvider(*provider); err != nil {
		t.Fatal(err)
	}
	catalog, err := builder.Build()
	if err != nil {
		t.Fatal(err)
	}
	binding := *second.ProviderBinding
	binding.ProviderID = "provider-b"
	two, err := sources.NewObservation(sources.ProvidersID, catalog, sources.ObservationMetadata{
		ProviderBinding: &binding, ObservedAt: at, Revision: sources.Revision{Kind: sources.RevisionKindContentDigest},
		Completeness: sources.ObservationCompletenessComplete, Status: sources.ObservationStatusSucceeded,
		Records: sources.ObservationRecordCounts{Accepted: 1},
	})
	if err != nil {
		t.Fatal(err)
	}
	baseline := scopedReconciliationBaseline(t, one, two)
	reconciler, err := New(WithBaseline(baseline))
	if err != nil {
		t.Fatal(err)
	}
	result, err := reconciler.Sources(t.Context(), sources.ProvidersID, []sources.Observation{one, two})
	if err != nil {
		t.Fatal(err)
	}
	for _, id := range []catalogs.ProviderID{"provider-a", "provider-b"} {
		provider, exists := result.Catalog.Providers().Get(id)
		if !exists || provider.Models["shared"] == nil {
			t.Fatalf("primary filtering lost %s", id)
		}
	}
	entries := result.Catalog.Provenance().FindModelField("provider-b", "shared", "limits.context_window")
	if len(entries) != 1 || entries[0].ObservationID != two.ID {
		t.Fatal("same model ID borrowed another provider's receipt")
	}
}

func scopedReconciliationBaseline(t *testing.T, observations ...sources.Observation) *catalogs.Catalog {
	t.Helper()
	builder := catalogs.NewEmpty()
	for _, observation := range observations {
		if err := builder.MergeWith(observation.Catalog, catalogs.WithStrategy(catalogs.MergeEnrichEmpty)); err != nil {
			t.Fatal(err)
		}
	}
	for _, provider := range builder.Providers().List() {
		provider.Models = nil
		if err := builder.SetProvider(provider); err != nil {
			t.Fatal(err)
		}
	}
	baseline, err := builder.Build()
	if err != nil {
		t.Fatal(err)
	}
	return baseline
}

func TestScopedReconciliationKeepsPeerOfferingsAndReceipts(t *testing.T) {
	at := time.Date(2026, 9, 7, 0, 0, 0, 0, time.UTC)
	one := scopedReconciliationObservation(t, "one", "model-one", 1000, at)
	two := scopedReconciliationObservation(t, "two", "model-two", 2000, at.Add(time.Minute))
	baseline := scopedReconciliationBaseline(t, one, two)
	for _, reverse := range []bool{false, true} {
		observations := []sources.Observation{one, two}
		if reverse {
			slices.Reverse(observations)
		}
		reconciler, err := New(WithBaseline(baseline))
		if err != nil {
			t.Fatal(err)
		}
		result, err := reconciler.Sources(t.Context(), sources.ProvidersID, observations)
		if err != nil {
			t.Fatal(err)
		}
		provider, found := result.Catalog.Providers().Get("provider-a")
		if !found || len(provider.Models) != 2 {
			t.Fatalf("lost a scoped offering (reverse=%t): %#v", reverse, provider)
		}
		for _, observation := range []sources.Observation{one, two} {
			for _, original := range observation.Catalog.Providers().List()[0].Models {
				model := provider.Models[original.ID]
				if model == nil || model.Limits == nil || model.Limits.ContextWindow != original.Limits.ContextWindow {
					t.Fatal("scope facts changed")
				}
				entries := result.Catalog.Provenance().FindModelField("provider-a", original.ID, "limits.context_window")
				if len(entries) != 1 || entries[0].ObservationID != observation.ID || entries[0].EvidenceChecksum != observation.EvidenceChecksum {
					t.Fatalf("model %s borrowed a peer receipt: %#v", original.ID, entries)
				}
			}
		}
	}
}

func TestScopedReconciliationEqualTimeProviderMembershipAndConflicts(t *testing.T) {
	at := time.Date(2026, 9, 7, 0, 0, 0, 0, time.UTC)
	one := scopedReconciliationObservation(t, "one", "model-one", 1000, at)
	two := scopedReconciliationObservation(t, "two", "model-two", 2000, at)
	baseline := scopedReconciliationBaseline(t, one, two)
	reconciler, err := New(WithBaseline(baseline))
	if err != nil {
		t.Fatal(err)
	}
	result, err := reconciler.Sources(t.Context(), sources.ProvidersID, []sources.Observation{one, two})
	if err != nil {
		t.Fatal(err)
	}
	provider, _ := result.Catalog.Providers().Get("provider-a")
	if len(provider.Models) != 2 {
		t.Fatal("equal-time provider membership erased a peer offering")
	}
	builder, err := catalogs.NewBuilderFrom(two.Catalog)
	if err != nil {
		t.Fatal(err)
	}
	provider, _ = builder.Providers().Get("provider-a")
	provider.Name = "Conflicting Provider"
	if err := builder.SetProvider(*provider); err != nil {
		t.Fatal(err)
	}
	catalog, err := builder.Build()
	if err != nil {
		t.Fatal(err)
	}
	changed, err := sources.NewObservation(two.SourceID, catalog, sources.ObservationMetadata{
		ProviderBinding: two.ProviderBinding, ObservedAt: two.ObservedAt, Revision: sources.Revision{Kind: sources.RevisionKindContentDigest},
		Completeness: two.Completeness, Status: two.Status, Records: two.Records,
	})
	if err != nil {
		t.Fatal(err)
	}
	var conflict *errors.ConflictError
	if result, err := reconciler.Sources(t.Context(), sources.ProvidersID, []sources.Observation{one, changed}); result != nil || !stderrors.As(err, &conflict) {
		t.Fatalf("conflicting provider facts did not stop reconciliation: %v", err)
	}
}

func TestScopedReconciliationSelectsNewestSharedOffering(t *testing.T) {
	at := time.Date(2026, 9, 7, 0, 0, 0, 0, time.UTC)
	old := scopedReconciliationObservation(t, "one", "shared", 1000, at)
	newer := scopedReconciliationObservation(t, "two", "shared", 2000, at.Add(time.Minute))
	baseline := scopedReconciliationBaseline(t, old, newer)
	for _, observations := range [][]sources.Observation{{old, newer}, {newer, old}} {
		reconciler, err := New(WithBaseline(baseline))
		if err != nil {
			t.Fatal(err)
		}
		result, err := reconciler.Sources(t.Context(), sources.ProvidersID, observations)
		if err != nil {
			t.Fatal(err)
		}
		provider, _ := result.Catalog.Providers().Get("provider-a")
		if provider.Models["shared"].Limits.ContextWindow != 2000 {
			t.Fatal("input order selected an older scoped offering")
		}
		assertModelEvidenceSource(t, result.Catalog, "limits.context_window", sources.ProvidersID, newer.ID)
		if result.ProviderAPICounts["provider-a"] != 1 {
			t.Fatal("duplicate scoped membership inflated the API model count")
		}
		if !slices.Equal(result.Metadata.Sources, []sources.ID{sources.ProvidersID}) {
			t.Fatal("scoped observations inflated the source-type report")
		}
		state, err := reconciler.initialize(t.Context(), sources.ProvidersID, observations)
		if err != nil {
			t.Fatal(err)
		}
		candidateObservation, found := state.collector.reviewCandidateObservation(provider, "shared")
		if !found || candidateObservation.ID != newer.ID {
			t.Fatal("review candidates did not select the winning record's receipt")
		}
	}
}
