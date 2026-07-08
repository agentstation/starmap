package pipeline

import (
	"context"
	stderrors "errors"
	"testing"

	"github.com/agentstation/starmap/pkg/catalogs"
	"github.com/agentstation/starmap/pkg/differ"
	pkgerrors "github.com/agentstation/starmap/pkg/errors"
	"github.com/agentstation/starmap/pkg/reconciler"
	"github.com/agentstation/starmap/pkg/sources"
	pkgsync "github.com/agentstation/starmap/pkg/sync"
)

type pipelineTestStore struct {
	catalog catalogs.Catalog
	err     error

	applyCalls     int
	appliedCatalog catalogs.Catalog
	appliedOptions *pkgsync.Options
	appliedChanges *differ.Changeset
}

func (s *pipelineTestStore) Catalog() (catalogs.Catalog, error) {
	if s.err != nil {
		return nil, s.err
	}
	return s.catalog, nil
}

func (s *pipelineTestStore) Apply(catalog catalogs.Catalog, options *pkgsync.Options, changeset *differ.Changeset) error {
	s.applyCalls++
	s.appliedCatalog = catalog
	s.appliedOptions = options
	s.appliedChanges = changeset
	return nil
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
	store := &pipelineTestStore{catalog: catalogs.NewEmpty()}
	runner := New(store)
	runner.loadLocal = func(string) (catalogs.Catalog, error) {
		return catalogs.NewEmpty(), nil
	}

	sourceWorkStarted := false
	runner.createSources = func(*pkgsync.Options, catalogs.Catalog) []sources.Source {
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

func TestPipelineDryRunSkipsApplyEvenWithChanges(t *testing.T) {
	store := &pipelineTestStore{catalog: catalogs.NewEmpty()}
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
	if store.applyCalls != 0 {
		t.Fatalf("Expected dry run to skip apply, got %d calls", store.applyCalls)
	}
}

func TestPipelineSkipsApplyWhenThereAreNoChanges(t *testing.T) {
	store := &pipelineTestStore{catalog: catalogs.NewEmpty()}
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

func TestPipelineForceSavesWhenReformatOrFreshIsSet(t *testing.T) {
	for _, tc := range []struct {
		name string
		opt  pkgsync.Option
	}{
		{name: "reformat", opt: pkgsync.WithReformat(true)},
		{name: "fresh", opt: pkgsync.WithFresh(true)},
	} {
		t.Run(tc.name, func(t *testing.T) {
			store := &pipelineTestStore{catalog: catalogs.NewEmpty()}
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
		})
	}
}

func newStubPipeline(store Store, result *reconciler.Result) *Pipeline {
	runner := New(store)
	runner.loadLocal = func(string) (catalogs.Catalog, error) {
		return catalogs.NewEmpty(), nil
	}
	runner.createSources = func(*pkgsync.Options, catalogs.Catalog) []sources.Source {
		return []sources.Source{&lifecycleTestSource{id: sources.LocalCatalogID, catalog: catalogs.NewEmpty()}}
	}
	runner.resolveDependencies = func(_ context.Context, srcs []sources.Source, _ *pkgsync.Options) ([]sources.Source, error) {
		return srcs, nil
	}
	runner.fetch = func(context.Context, []sources.Source, []sources.Option) error {
		return nil
	}
	runner.cleanup = func(context.Context, []sources.Source) error {
		return nil
	}
	runner.reconcile = func(context.Context, catalogs.Catalog, []sources.Source) (*reconciler.Result, error) {
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
