package pipeline

import (
	"context"
	stderrors "errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/agentstation/starmap/internal/catalog/reconciler"
	"github.com/agentstation/starmap/internal/catalog/workspace"
	"github.com/agentstation/starmap/pkg/catalogmeta"
	"github.com/agentstation/starmap/pkg/catalogs"
	"github.com/agentstation/starmap/pkg/differ"
	pkgerrors "github.com/agentstation/starmap/pkg/errors"
	"github.com/agentstation/starmap/pkg/logging"
	"github.com/agentstation/starmap/pkg/sources"
	pkgsync "github.com/agentstation/starmap/pkg/sync"
)

type pipelineTestStore struct {
	catalog *catalogs.Catalog
	err     error

	applyCalls       int
	appliedCatalog   *catalogs.Builder
	appliedOptions   *pkgsync.Options
	appliedChanges   *differ.Changeset
	observations     []sources.Observation
	reviewCandidates []catalogmeta.ReviewCandidate
	workspaceInput   workspace.InputExpectation
}

func (s *pipelineTestStore) Catalog() (*catalogs.Catalog, error) {
	if s.err != nil {
		return nil, s.err
	}
	return s.catalog, nil
}

func (s *pipelineTestStore) Apply(
	_ context.Context,
	catalog *catalogs.Builder,
	options *pkgsync.Options,
	changeset *differ.Changeset,
	observations []sources.Observation,
	reviewCandidates []catalogmeta.ReviewCandidate,
	workspaceInput workspace.InputExpectation,
) (Publication, error) {
	s.applyCalls++
	s.appliedCatalog = catalog
	s.appliedOptions = options
	s.appliedChanges = changeset
	s.observations = append([]sources.Observation(nil), observations...)
	s.reviewCandidates = append([]catalogmeta.ReviewCandidate(nil), reviewCandidates...)
	s.workspaceInput = workspaceInput
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
	runner.loadWorkspace = func(string) (*catalogs.Builder, error) {
		return catalogs.NewEmpty(), nil
	}

	sourceWorkStarted := false
	runner.createSources = func(*pkgsync.Options, catalogInputs) []sources.Source {
		sourceWorkStarted = true
		return nil
	}

	_, err := runner.Sync(context.Background(), pkgsync.WithProvider("missing-provider"))
	if err == nil {
		t.Fatal("Expected missing provider validation to fail")
	}
	if sourceWorkStarted {
		t.Fatal("Expected validation to fail before source construction")
	}
}

func TestPipelineRejectsSourceStateOverlapBeforeReadingWorkspace(t *testing.T) {
	root := t.TempDir()
	store := &pipelineTestStore{catalog: asSnapshot(catalogs.NewEmpty())}
	runner := New(store)
	workspaceRead := false
	runner.loadWorkspace = func(string) (*catalogs.Builder, error) {
		workspaceRead = true
		return catalogs.NewEmpty(), nil
	}

	_, err := runner.Sync(
		context.Background(),
		pkgsync.WithCatalogPath(filepath.Join(root, "catalog")),
		pkgsync.WithSourcesDir(root),
		pkgsync.WithDryRun(true),
	)
	var configErr *pkgerrors.ConfigError
	if !stderrors.As(err, &configErr) {
		t.Fatalf("Sync error = %T %v, want *errors.ConfigError", err, err)
	}
	if workspaceRead {
		t.Fatal("overlapping machine source state was rejected after reading the human workspace")
	}
}

func TestPipelineUsesOneCatalogPathForLocalObservation(t *testing.T) {
	store := &pipelineTestStore{catalog: asSnapshot(catalogs.NewEmpty())}
	runner := newStubPipeline(store, &reconciler.Result{
		Catalog:           catalogs.NewEmpty(),
		Changeset:         emptyChangeset(),
		ProviderAPICounts: map[catalogs.ProviderID]int{},
		ModelProviderMap:  map[string]catalogs.ProviderID{},
	})
	catalogPath := filepath.Join(t.TempDir(), "catalog")
	var loadedPath string
	runner.loadWorkspace = func(path string) (*catalogs.Builder, error) {
		loadedPath = path
		return catalogs.NewEmpty(), nil
	}

	if _, err := runner.Sync(
		context.Background(),
		pkgsync.WithDryRun(true),
		pkgsync.WithCatalogPath(catalogPath),
	); err != nil {
		t.Fatalf("Sync: %v", err)
	}
	if loadedPath != catalogPath {
		t.Fatalf("local path = %q, want catalog path %q", loadedPath, catalogPath)
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

func TestPipelineFirstExplicitUpdateAppliesUnchangedCandidateToAbsentWorkspace(t *testing.T) {
	store := &pipelineTestStore{catalog: asSnapshot(catalogs.NewEmpty())}
	runner := newStubPipeline(store, &reconciler.Result{
		Catalog: catalogs.NewEmpty(), Changeset: emptyChangeset(),
		ProviderAPICounts: map[catalogs.ProviderID]int{}, ModelProviderMap: map[string]catalogs.ProviderID{},
	})
	path := filepath.Join(t.TempDir(), "catalog")

	if _, err := runner.Sync(context.Background(), pkgsync.WithCatalogPath(path)); err != nil {
		t.Fatalf("Sync: %v", err)
	}
	if store.applyCalls != 1 {
		t.Fatalf("apply calls = %d, want one seed publication", store.applyCalls)
	}
	if store.workspaceInput.Path != path || store.workspaceInput.Exists {
		t.Fatalf("workspace input = %#v, want absent %q", store.workspaceInput, path)
	}
}

func TestPipelineDryRunDoesNotSeedAbsentWorkspace(t *testing.T) {
	store := &pipelineTestStore{catalog: asSnapshot(catalogs.NewEmpty())}
	runner := newStubPipeline(store, &reconciler.Result{
		Catalog: catalogs.NewEmpty(), Changeset: emptyChangeset(),
		ProviderAPICounts: map[catalogs.ProviderID]int{}, ModelProviderMap: map[string]catalogs.ProviderID{},
	})
	path := filepath.Join(t.TempDir(), "catalog")

	if _, err := runner.Sync(
		context.Background(),
		pkgsync.WithCatalogPath(path),
		pkgsync.WithDryRun(true),
	); err != nil {
		t.Fatalf("Sync: %v", err)
	}
	if store.applyCalls != 0 {
		t.Fatalf("dry-run apply calls = %d, want zero", store.applyCalls)
	}
	if _, err := os.Lstat(path); !stderrors.Is(err, os.ErrNotExist) {
		t.Fatalf("dry run created workspace: %v", err)
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

func TestPipelinePublishesReviewCandidatesWithoutCatalogChanges(t *testing.T) {
	store := &pipelineTestStore{catalog: asSnapshot(catalogs.NewEmpty())}
	runner := newStubPipeline(store, nil)
	runner.reconcile = func(
		_ context.Context,
		_ *catalogs.Catalog,
		observations []sources.Observation,
	) (*reconciler.Result, error) {
		observation := observations[0]
		return &reconciler.Result{
			Catalog:           catalogs.NewEmpty(),
			Changeset:         emptyChangeset(),
			ProviderAPICounts: map[catalogs.ProviderID]int{},
			ModelProviderMap:  map[string]catalogs.ProviderID{},
			ReviewCandidates: []catalogmeta.ReviewCandidate{{
				Code:                catalogmeta.ReviewCandidateUnresolvedModelReference,
				ProviderID:          "provider",
				ProviderModelID:     "opaque/model@2026",
				SourceID:            observation.SourceID,
				SourceObservationID: observation.ID,
				SourceRevision:      observation.Revision,
				EvidenceChecksum:    observation.EvidenceChecksum,
				Reason:              "provider model has no reviewed canonical model link",
			}},
		}, nil
	}

	result, err := runner.Sync(context.Background())
	if err != nil {
		t.Fatalf("Sync: %v", err)
	}
	if result.HasChanges() {
		t.Fatal("review-candidate publication reported a catalog change")
	}
	if store.applyCalls != 1 {
		t.Fatalf("apply calls = %d, want one evidence publication", store.applyCalls)
	}
	if len(store.reviewCandidates) != 1 ||
		store.reviewCandidates[0] != result.ReviewCandidates[0] {
		t.Fatalf("applied review candidates = %#v, result = %#v", store.reviewCandidates, result.ReviewCandidates)
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

func TestPipelineReturnsCallerOwnedReviewCandidates(t *testing.T) {
	t.Parallel()

	issue := catalogmeta.ReviewCandidate{
		Code:            catalogmeta.ReviewCandidateUnresolvedModelReference,
		ProviderID:      "provider",
		ProviderModelID: "new-model",
		Reason:          "quarantined",
	}
	reconcileResult := &reconciler.Result{
		Catalog:           catalogs.NewEmpty(),
		Changeset:         emptyChangeset(),
		ProviderAPICounts: map[catalogs.ProviderID]int{},
		ModelProviderMap:  map[string]catalogs.ProviderID{},
		ReviewCandidates:  []catalogmeta.ReviewCandidate{issue},
	}
	store := &pipelineTestStore{catalog: asSnapshot(catalogs.NewEmpty())}
	runner := newStubPipeline(store, reconcileResult)

	result, err := runner.Sync(context.Background(), pkgsync.WithDryRun(true))
	if err != nil {
		t.Fatalf("Sync: %v", err)
	}
	if len(result.ReviewCandidates) != 1 || result.ReviewCandidates[0] != issue {
		t.Fatalf("review candidates = %#v, want %#v", result.ReviewCandidates, issue)
	}
	result.ReviewCandidates[0].Reason = "caller mutation"
	if reconcileResult.ReviewCandidates[0].Reason != issue.Reason {
		t.Fatal("sync result retained review-candidate storage")
	}
}

func TestPipelineContinuesNonStrictAfterSourceFailureWithDegradedEvidence(t *testing.T) {
	existing := catalogs.NewEmpty()
	if err := existing.SetProvider(catalogs.Provider{ID: "last-known-good", Name: "Last Known Good"}); err != nil {
		t.Fatalf("SetProvider existing: %v", err)
	}
	store := &pipelineTestStore{catalog: asSnapshot(existing)}
	runner := newStubPipeline(store, nil)
	sourceErr := stderrors.New("provider timed out")
	failed, err := failedSourceObservation(sources.ProvidersID, sourceErr)
	if err != nil {
		t.Fatalf("failedSourceObservation: %v", err)
	}
	runner.observe = func(context.Context, []sources.Source, []sources.Option) ([]sources.Observation, error) {
		return []sources.Observation{failed}, sourceErr
	}
	runner.reconcile = func(_ context.Context, baseline *catalogs.Catalog, observations []sources.Observation) (*reconciler.Result, error) {
		if _, err := baseline.Provider("last-known-good"); err != nil {
			t.Fatalf("baseline lost after source failure: %v", err)
		}
		if len(observations) != 1 ||
			observations[0].Status != sources.ObservationStatusDegraded ||
			observations[0].Completeness != sources.ObservationCompletenessPartial {
			t.Fatalf("degraded observations = %#v", observations)
		}
		return &reconciler.Result{
			Catalog:           existing,
			Changeset:         emptyChangeset(),
			ProviderAPICounts: map[catalogs.ProviderID]int{},
			ModelProviderMap:  map[string]catalogs.ProviderID{},
		}, nil
	}

	result, err := runner.Sync(context.Background(), pkgsync.WithDryRun(true))
	if err != nil {
		t.Fatalf("non-strict Sync: %v", err)
	}
	if len(result.SourceObservations) != 1 ||
		result.SourceObservations[0].Status != sources.ObservationStatusDegraded {
		t.Fatalf("sync observations = %#v", result.SourceObservations)
	}
}

func TestPipelineRequireAllSourcesRejectsSourceFailureBeforeReconciliation(t *testing.T) {
	store := &pipelineTestStore{catalog: asSnapshot(catalogs.NewEmpty())}
	runner := newStubPipeline(store, nil)
	sourceErr := stderrors.New("provider timed out")
	failed, err := failedSourceObservation(sources.ProvidersID, sourceErr)
	if err != nil {
		t.Fatalf("failedSourceObservation: %v", err)
	}
	runner.observe = func(context.Context, []sources.Source, []sources.Option) ([]sources.Observation, error) {
		return []sources.Observation{failed}, sourceErr
	}
	runner.reconcile = func(context.Context, *catalogs.Catalog, []sources.Observation) (*reconciler.Result, error) {
		t.Fatal("strict source failure reached reconciliation")
		return nil, nil
	}

	if _, err := runner.Sync(context.Background(), pkgsync.WithRequireAllSources(true)); !stderrors.Is(err, sourceErr) {
		t.Fatalf("strict Sync error = %v, want source failure", err)
	}
}

func TestPipelineFreshRejectsDegradedObservationBeforeEmptyBaselinePublication(t *testing.T) {
	existing := catalogs.NewEmpty()
	if err := existing.SetProvider(catalogs.Provider{ID: "last-known-good", Name: "Last Known Good"}); err != nil {
		t.Fatalf("SetProvider existing: %v", err)
	}
	store := &pipelineTestStore{catalog: asSnapshot(existing)}
	runner := newStubPipeline(store, nil)
	failed, err := failedSourceObservation(sources.ProvidersID, stderrors.New("provider timed out"))
	if err != nil {
		t.Fatalf("failedSourceObservation: %v", err)
	}
	runner.observe = func(context.Context, []sources.Source, []sources.Option) ([]sources.Observation, error) {
		return []sources.Observation{failed}, nil
	}
	runner.reconcile = func(context.Context, *catalogs.Catalog, []sources.Observation) (*reconciler.Result, error) {
		t.Fatal("degraded fresh sync reached reconciliation")
		return nil, nil
	}

	if _, err := runner.Sync(context.Background(), pkgsync.WithFresh(true)); err == nil {
		t.Fatal("degraded fresh Sync returned nil error")
	}
	if store.applyCalls != 0 {
		t.Fatalf("degraded fresh Sync applied %d candidates", store.applyCalls)
	}
}

func newStubPipeline(store Store, result *reconciler.Result) *Pipeline {
	runner := New(store)
	runner.loadWorkspace = func(string) (*catalogs.Builder, error) {
		return catalogs.NewEmpty(), nil
	}
	runner.loadEmbedded = func() (*catalogs.Builder, error) {
		return catalogs.NewEmpty(), nil
	}
	runner.createSources = func(*pkgsync.Options, catalogInputs) []sources.Source {
		return []sources.Source{&lifecycleTestSource{id: sources.LocalCatalogID, catalog: asSnapshot(catalogs.NewEmpty())}}
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
		Models:    &differ.ModelChangeset{},
		Providers: &differ.ProviderChangeset{},
		Authors:   &differ.AuthorChangeset{},
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
