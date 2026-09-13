package pipeline

import (
	"context"
	stderrors "errors"
	"sync/atomic"
	"testing"
	"time"

	"github.com/agentstation/starmap/internal/catalog/reconciler"
	"github.com/agentstation/starmap/pkg/catalogs"
	pkgerrors "github.com/agentstation/starmap/pkg/errors"
	"github.com/agentstation/starmap/pkg/sources"
	pkgsync "github.com/agentstation/starmap/pkg/sync"
)

func TestCollectionStopsBeforeReconciliationAndPublication(t *testing.T) {
	for _, failed := range []bool{false, true} {
		name := "complete"
		if failed {
			name = "failed_source"
		}
		t.Run(name, func(t *testing.T) {
			catalog := asSnapshot(catalogs.NewEmpty())
			store := &pipelineTestStore{catalog: catalog}
			runner := newStubPipeline(store, nil)
			source := &collectionTestSource{lifecycleTestSource: &lifecycleTestSource{id: sources.LocalCatalogID, catalog: catalog}}
			if failed {
				source.fetchErr = &pkgerrors.ConfigError{Component: "test", Message: "source unavailable"}
			}
			runner.createSources = func(*pkgsync.Options, catalogInputs) []sources.Source {
				return []sources.Source{source}
			}
			runner.observe = observe
			runner.cleanup = cleanup
			runner.reconcile = func(context.Context, *catalogs.Catalog, []sources.Observation) (*reconciler.Result, error) {
				t.Fatal("collection called reconciliation before publication admission")
				return nil, nil
			}
			collected, err := runner.Collect(t.Context(), catalog, pkgsync.WithSources(sources.LocalCatalogID))
			if err != nil {
				t.Fatal(err)
			}
			if store.applyCalls != 0 || len(collected.Observations) != 1 {
				t.Fatalf("publication calls=%d, observations=%d", store.applyCalls, len(collected.Observations))
			}
			observation := collected.Observations[0]
			if err := observation.Validate(); err != nil {
				t.Fatal(err)
			}
			if observation.ObservedAt.Before(collected.StartedAt) || observation.ObservedAt.After(collected.CompletedAt) {
				t.Fatal("collection interval does not cover the source observation")
			}
			if failed != (len(collected.SourceFailures) > 0) {
				t.Fatalf("source failure evidence=%v", collected.SourceFailures)
			}
			summaries := collected.FailureSummaries()
			if failed != (len(summaries) > 0) || (failed && summaries[0].Source != sources.LocalCatalogID) {
				t.Fatalf("source failure summaries=%v", summaries)
			}
			if failed && observation.Status == sources.ObservationStatusSucceeded {
				t.Fatal("failed collection became successful evidence")
			}
			calls, _, cleanup := source.counts()
			if calls != 1 || cleanup != 1 {
				t.Fatalf("source calls=%d cleanup=%d", calls, cleanup)
			}
			if len(collected.SourceActivities) != 4 || !collected.SourceActivities[0].Attempted {
				t.Fatalf("source activity=%+v", collected.SourceActivities)
			}
		})
	}
}

func TestCollectionCancellationCannotReturnPublishableEvidence(t *testing.T) {
	for _, phase := range []string{"observe", "cleanup"} {
		t.Run(phase, func(t *testing.T) {
			catalog := asSnapshot(catalogs.NewEmpty())
			runner := newStubPipeline(&pipelineTestStore{catalog: catalog}, nil)
			ctx, cancel := context.WithCancel(t.Context())
			defer cancel()
			runner.observe = func(context.Context, []sources.Source, []sources.Option) ([]sources.Observation, error) {
				if phase == "observe" {
					cancel()
				}
				return nil, nil
			}
			runner.cleanup = func(context.Context, []sources.Source) error {
				if phase == "cleanup" {
					cancel()
				}
				return nil
			}
			collected, err := runner.Collect(ctx, catalog)
			if !stderrors.Is(err, context.Canceled) || collected != nil {
				t.Fatalf("canceled collection=%v, err=%v", collected, err)
			}
		})
	}
}

func TestCollectionKeepsIndependentProviderAttempts(t *testing.T) {
	builder, bindings := manualBindingFixture(t)
	var failed atomic.Bool
	var resolutions, calls atomic.Int32
	failed.Store(true)
	runner := manualBindingRunner(t, builder, bindings, &failed, &resolutions, &calls)
	runner.reconcile = func(context.Context, *catalogs.Catalog, []sources.Observation) (*reconciler.Result, error) {
		t.Fatal("provider collection reconciled evidence before admission")
		return nil, nil
	}
	collected, err := runner.Collect(t.Context(), buildCatalog(t, builder), pkgsync.WithCatalogPath(t.TempDir()), pkgsync.WithSources(sources.ProvidersID))
	if err != nil {
		t.Fatal(err)
	}
	if resolutions.Load() != 2 || calls.Load() != 2 || len(collected.ProviderAttempts) != 2 {
		t.Fatalf("resolutions=%d calls=%d attempts=%+v", resolutions.Load(), calls.Load(), collected.ProviderAttempts)
	}
	if collected.ProviderAttempts[1].Outcome != sources.ProviderOutcomeFailed {
		t.Fatalf("failed provider lost its attempt outcome: %+v", collected.ProviderAttempts[1])
	}
	seen := make(map[string]bool)
	for _, observation := range collected.Observations {
		if observation.SourceID != sources.ProvidersID {
			continue
		}
		if err := observation.Validate(); err != nil {
			t.Fatal(err)
		}
		if observation.ProviderBinding == nil {
			t.Fatal("provider evidence lost its account binding")
		}
		binding := *observation.ProviderBinding
		seen[binding.ID] = true
		if binding.ID == "one" && observation.Status != sources.ObservationStatusSucceeded {
			t.Fatal("unrelated provider failure degraded complete evidence")
		}
		if binding.ID == "two" && observation.Status != sources.ObservationStatusDegraded {
			t.Fatal("failed provider became complete evidence")
		}
	}
	if !seen["one"] || !seen["two"] {
		t.Fatalf("provider observation scopes=%v", seen)
	}
	collected.ProviderAttempts[0].BindingID = "caller-change"
	collected.SourceActivities[0].Supported = false
	next, err := runner.Collect(t.Context(), buildCatalog(t, builder), pkgsync.WithCatalogPath(t.TempDir()), pkgsync.WithSources(sources.ProvidersID))
	if err != nil {
		t.Fatal(err)
	}
	if next.ProviderAttempts[0].BindingID == "caller-change" || !next.SourceActivities[0].Supported {
		t.Fatal("collection retained caller-owned diagnostic changes")
	}
}

// collectionTestSource returns observations inside the measured acquisition interval.
type collectionTestSource struct{ *lifecycleTestSource }

func (s *collectionTestSource) Observe(ctx context.Context, opts ...sources.Option) (sources.Observation, error) {
	observation, err := s.lifecycleTestSource.Observe(ctx, opts...)
	if observation.Catalog == nil {
		return observation, err
	}
	current, receiptErr := sources.NewObservation(observation.SourceID, observation.Catalog, sources.ObservationMetadata{
		ObservedAt: time.Now().UTC(), Revision: observation.Revision, Completeness: observation.Completeness,
		Status: observation.Status, Issues: observation.Issues,
	})
	if receiptErr != nil {
		return sources.Observation{}, receiptErr
	}
	return current, err
}
