package pipeline

import (
	"context"
	stderrors "errors"
	"testing"
	"time"

	"github.com/agentstation/starmap/pkg/catalogs"
	"github.com/agentstation/starmap/pkg/differ"
	pkgerrors "github.com/agentstation/starmap/pkg/errors"
	"github.com/agentstation/starmap/pkg/logging"
	"github.com/agentstation/starmap/pkg/reconciler"
	"github.com/agentstation/starmap/pkg/sources"
	pkgsync "github.com/agentstation/starmap/pkg/sync"
)

type pipelineTestStore struct {
	catalog *catalogs.Catalog
	err     error

	applyCalls     int
	appliedCatalog *catalogs.Builder
	appliedOptions *pkgsync.Options
	appliedChanges *differ.Changeset
	observations   []sources.Observation
}

func (s *pipelineTestStore) Catalog() (*catalogs.Catalog, error) {
	if s.err != nil {
		return nil, s.err
	}
	return s.catalog, nil
}

func (s *pipelineTestStore) Apply(_ context.Context, catalog *catalogs.Builder, options *pkgsync.Options, changeset *differ.Changeset, observations []sources.Observation) (Publication, error) {
	s.applyCalls++
	s.appliedCatalog = catalog
	s.appliedOptions = options
	s.appliedChanges = changeset
	s.observations = append([]sources.Observation(nil), observations...)
	return Publication{}, nil
}

func TestPipelineRequiresStore(t *testing.T) {
	_, err := New(nil).Sync(context.Background())
	if err == nil {
		t.Fatal("Expected missing store to fail")
	}
	var configErr *pkgerrors.ConfigError
	if !stderrors.As(err, &configErr) {
		t.Fatalf("Expected ConfigError, got %T: %v", err, err)
	}
}

func TestPipelineValidatesOptionsBeforeSourceWork(t *testing.T) {
	store := &pipelineTestStore{catalog: asSnapshot(catalogs.NewEmpty())}
	runner := New(store)
	runner.loadLocal = func(string) (*catalogs.Builder, error) {
		return catalogs.NewEmpty(), nil
	}

	sourceWorkStarted := false
	runner.createSources = func(*pkgsync.Options, *catalogs.Catalog) ([]sources.Source, error) {
		sourceWorkStarted = true
		return nil, nil
	}

	_, err := runner.Sync(context.Background(), pkgsync.WithProvider("missing-provider"))
	if err == nil {
		t.Fatal("Expected missing provider validation to fail")
	}
	if sourceWorkStarted {
		t.Fatal("Expected validation to fail before source construction")
	}
}

func TestPipelineFailsClosedWhenSourceConstructionFails(t *testing.T) {
	store := &pipelineTestStore{catalog: asSnapshot(catalogs.NewEmpty())}
	runner := New(store)
	runner.loadLocal = func(string) (*catalogs.Builder, error) { return catalogs.NewEmpty(), nil }
	constructionFailure := stderrors.New("cloud registry construction failed")
	runner.createSources = func(*pkgsync.Options, *catalogs.Catalog) ([]sources.Source, error) {
		return nil, constructionFailure
	}
	dependencyResolutionStarted := false
	runner.resolveDependencies = func(context.Context, []sources.Source, *pkgsync.Options) ([]sources.Source, error) {
		dependencyResolutionStarted = true
		return nil, nil
	}

	_, err := runner.Sync(context.Background())
	if !stderrors.Is(err, constructionFailure) {
		t.Fatalf("Sync error = %v, want source construction failure", err)
	}
	if dependencyResolutionStarted {
		t.Fatal("pipeline continued after source construction failure")
	}
}

func TestPipelineDryRunSkipsApplyEvenWithChanges(t *testing.T) {
	store := &pipelineTestStore{catalog: asSnapshot(catalogs.NewEmpty())}
	runner := newStubPipeline(store, &reconciler.Result{
		Catalog:           catalogs.NewEmpty(),
		Changeset:         changesetWithAddedModel("dry-run-model"),
		ProviderAPICounts: map[catalogs.ProviderID]int{"test-provider": 1},
		ModelProviderMap:  map[string]catalogs.ProviderID{"dry-run-model": "test-provider"},
	})

	result, err := runner.Sync(context.Background(), pkgsync.WithDryRun(true))
	if err != nil {
		t.Fatalf("Dry-run sync failed: %v", err)
	}
	if !result.DryRun {
		t.Fatal("Expected dry-run result")
	}
	if !result.HasChanges() {
		t.Fatal("Expected dry-run result to retain detected changes")
	}
	if len(result.SourceObservations) != 1 || result.SourceObservations[0].Source != sources.LocalCatalogID {
		t.Fatalf("dry-run source observations = %#v", result.SourceObservations)
	}
	if err := result.SourceObservations[0].Validate(); err != nil {
		t.Fatalf("dry-run source observation: %v", err)
	}
	if store.applyCalls != 0 {
		t.Fatalf("Expected dry run to skip apply, got %d calls", store.applyCalls)
	}
}

func TestPipelineAddsSourceRunCorrelationBeforeObservation(t *testing.T) {
	store := &pipelineTestStore{catalog: asSnapshot(catalogs.NewEmpty())}
	runner := newStubPipeline(store, &reconciler.Result{
		Catalog: catalogs.NewEmpty(), Changeset: emptyChangeset(),
		ProviderAPICounts: map[catalogs.ProviderID]int{}, ModelProviderMap: map[string]catalogs.ProviderID{},
	})
	originalObserve := runner.observe
	runner.observe = func(ctx context.Context, srcs []sources.Source, opts []sources.Option) ([]sources.Observation, error) {
		if logging.RunID(ctx) == "" {
			t.Fatal("source observation context has no run correlation ID")
		}
		return originalObserve(ctx, srcs, opts)
	}
	if _, err := runner.Sync(context.Background(), pkgsync.WithDryRun(true)); err != nil {
		t.Fatalf("Sync: %v", err)
	}
}

func TestPipelineNoChangeStillReportsSourceFreshnessObservation(t *testing.T) {
	store := &pipelineTestStore{catalog: asSnapshot(catalogs.NewEmpty())}
	runner := newStubPipeline(store, &reconciler.Result{
		Catalog: catalogs.NewEmpty(), Changeset: emptyChangeset(),
		ProviderAPICounts: map[catalogs.ProviderID]int{}, ModelProviderMap: map[string]catalogs.ProviderID{},
	})
	result, err := runner.Sync(context.Background(), pkgsync.WithDryRun(true))
	if err != nil {
		t.Fatalf("no-change Sync: %v", err)
	}
	if result.HasChanges() || len(result.SourceObservations) != 1 {
		t.Fatalf("no-change result = %#v", result)
	}
	if err := result.SourceObservations[0].Validate(); err != nil {
		t.Fatalf("source freshness observation: %v", err)
	}
}

func TestPipelineSkipsApplyWhenThereAreNoChanges(t *testing.T) {
	store := &pipelineTestStore{catalog: asSnapshot(catalogs.NewEmpty())}
	runner := newStubPipeline(store, &reconciler.Result{
		Catalog:           catalogs.NewEmpty(),
		Changeset:         emptyChangeset(),
		ProviderAPICounts: map[catalogs.ProviderID]int{},
		ModelProviderMap:  map[string]catalogs.ProviderID{},
	})

	result, err := runner.Sync(context.Background())
	if err != nil {
		t.Fatalf("Sync failed: %v", err)
	}
	if result.HasChanges() {
		t.Fatal("Expected no-change result")
	}
	if store.applyCalls != 0 {
		t.Fatalf("Expected no-change sync to skip apply, got %d calls", store.applyCalls)
	}
}

func TestPipelinePublishesCanonicalOnlyOfferingChange(t *testing.T) {
	store := &pipelineTestStore{catalog: asSnapshot(catalogs.NewEmpty())}
	changes := emptyChangeset()
	changes.Offerings.Updated = []differ.ProviderOfferingUpdate{{
		Key: catalogs.OfferingKey{ProviderID: "amazon-bedrock", ProviderModelID: "model"},
	}}
	changes.Summary = differ.ChangesetSummary{OfferingsUpdated: 1, TotalChanges: 1}
	runner := newStubPipeline(store, &reconciler.Result{
		Catalog: catalogs.NewEmpty(), Changeset: changes,
		ProviderAPICounts: map[catalogs.ProviderID]int{}, ModelProviderMap: map[string]catalogs.ProviderID{},
	})
	if _, err := runner.Sync(context.Background()); err != nil {
		t.Fatalf("Sync: %v", err)
	}
	if store.applyCalls != 1 || store.appliedChanges.Summary.OfferingsUpdated != 1 {
		t.Fatalf("canonical-only publication = calls %d changes %#v", store.applyCalls, store.appliedChanges)
	}
}

func TestPipelineForceSavesWhenReformatOrFreshIsSet(t *testing.T) {
	for _, tc := range []struct {
		name      string
		opt       pkgsync.Option
		wantFresh bool
	}{
		{name: "reformat", opt: pkgsync.WithReformat(true)},
		{name: "fresh", opt: pkgsync.WithFresh(true), wantFresh: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			store := &pipelineTestStore{catalog: asSnapshot(catalogs.NewEmpty())}
			finalCatalog := catalogs.NewEmpty()
			runner := newStubPipeline(store, &reconciler.Result{
				Catalog:           finalCatalog,
				Changeset:         emptyChangeset(),
				ProviderAPICounts: map[catalogs.ProviderID]int{},
				ModelProviderMap:  map[string]catalogs.ProviderID{},
			})

			result, err := runner.Sync(context.Background(), tc.opt)
			if err != nil {
				t.Fatalf("Force-save sync failed: %v", err)
			}
			if result.HasChanges() {
				t.Fatal("Expected force-save result to preserve no-change summary")
			}
			if result.Fresh != tc.wantFresh {
				t.Fatalf("Fresh result = %t, want %t", result.Fresh, tc.wantFresh)
			}
			if store.applyCalls != 1 {
				t.Fatalf("Expected force-save sync to apply once, got %d calls", store.applyCalls)
			}
			if store.appliedCatalog != finalCatalog {
				t.Fatal("Expected force-save sync to apply reconciled catalog")
			}
			if store.appliedOptions == nil {
				t.Fatal("Expected apply to receive sync options")
			}
			if store.appliedChanges == nil {
				t.Fatal("Expected apply to receive a non-nil changeset")
			}
			if len(store.observations) != 1 {
				t.Fatalf("Apply observations = %#v", store.observations)
			}
			if err := store.observations[0].Validate(); err != nil {
				t.Fatalf("Apply observation: %v", err)
			}
		})
	}
}

func TestPipelineFreshReconcilesAgainstEmptyBaseline(t *testing.T) {
	existing := catalogs.NewEmpty()
	if err := existing.SetProvider(catalogs.Provider{ID: "stale", Name: "Stale"}); err != nil {
		t.Fatalf("Seed existing catalog: %v", err)
	}

	store := &pipelineTestStore{catalog: asSnapshot(existing)}
	runner := newStubPipeline(store, &reconciler.Result{
		Catalog:           catalogs.NewEmpty(),
		Changeset:         emptyChangeset(),
		ProviderAPICounts: map[catalogs.ProviderID]int{},
		ModelProviderMap:  map[string]catalogs.ProviderID{},
	})
	runner.reconcile = func(_ context.Context, baseline *catalogs.Catalog, _ []sources.Observation) (*reconciler.Result, error) {
		if baseline.Providers().Len() != 0 {
			t.Fatalf("Fresh reconciliation baseline contains %d providers, want 0", baseline.Providers().Len())
		}
		return &reconciler.Result{
			Catalog:           catalogs.NewEmpty(),
			Changeset:         emptyChangeset(),
			ProviderAPICounts: map[catalogs.ProviderID]int{},
			ModelProviderMap:  map[string]catalogs.ProviderID{},
		}, nil
	}

	if _, err := runner.Sync(context.Background(), pkgsync.WithFresh(true)); err != nil {
		t.Fatalf("Fresh sync failed: %v", err)
	}
}

func newStubPipeline(store Store, result *reconciler.Result) *Pipeline {
	runner := New(store)
	runner.loadLocal = func(string) (*catalogs.Builder, error) {
		return catalogs.NewEmpty(), nil
	}
	runner.createSources = func(*pkgsync.Options, *catalogs.Catalog) ([]sources.Source, error) {
		return []sources.Source{&lifecycleTestSource{id: sources.LocalCatalogID, catalog: asSnapshot(catalogs.NewEmpty())}}, nil
	}
	runner.resolveDependencies = func(_ context.Context, srcs []sources.Source, _ *pkgsync.Options) ([]sources.Source, error) {
		return srcs, nil
	}
	runner.observe = func(_ context.Context, srcs []sources.Source, _ []sources.Option) ([]sources.Observation, error) {
		observation, err := sources.NewObservation(srcs[0].ID(), asSnapshot(catalogs.NewEmpty()), sources.ObservationMetadata{
			ObservedAt:   time.Date(2026, time.July, 9, 12, 0, 0, 0, time.UTC),
			Revision:     sources.Revision{Kind: sources.RevisionKindContentDigest},
			Completeness: sources.ObservationCompletenessComplete,
			Status:       sources.ObservationStatusSucceeded,
		})
		if err != nil {
			return nil, err
		}
		return []sources.Observation{observation}, nil
	}
	runner.cleanup = func(context.Context, []sources.Source) error {
		return nil
	}
	runner.reconcile = func(context.Context, *catalogs.Catalog, []sources.Observation) (*reconciler.Result, error) {
		return result, nil
	}
	return runner
}

func emptyChangeset() *differ.Changeset {
	return &differ.Changeset{
		Models:      &differ.ModelChangeset{},
		Providers:   &differ.ProviderChangeset{},
		Authors:     &differ.AuthorChangeset{},
		Definitions: &differ.ModelDefinitionChangeset{},
		Offerings:   &differ.ProviderOfferingChangeset{},
	}
}

func changesetWithAddedModel(modelID string) *differ.Changeset {
	changeset := emptyChangeset()
	changeset.Models.Added = []catalogs.Model{{ID: modelID}}
	changeset.Summary = differ.ChangesetSummary{
		ModelsAdded:  1,
		TotalChanges: 1,
	}
	return changeset
}
