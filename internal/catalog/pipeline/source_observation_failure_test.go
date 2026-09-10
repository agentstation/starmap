package pipeline

import (
	"context"
	stderrors "errors"
	"testing"

	"github.com/agentstation/starmap/internal/catalog/reconciler"
	"github.com/agentstation/starmap/pkg/catalogs"
	pkgerrors "github.com/agentstation/starmap/pkg/errors"
	"github.com/agentstation/starmap/pkg/sources"
	pkgsync "github.com/agentstation/starmap/pkg/sync"
)

type invalidObservationSource struct{ *lifecycleTestSource }

func (s *invalidObservationSource) Observe(ctx context.Context, opts ...sources.Option) (sources.Observation, error) {
	observation, err := s.lifecycleTestSource.Observe(ctx, opts...)
	observation.ID = "invalid-observation"
	return observation, err
}

func TestPipelineReportsRejectedObservationWithoutReceipt(t *testing.T) {
	for _, fresh := range []bool{false, true} {
		name := "partial"
		if fresh {
			name = "fresh"
		}
		t.Run(name, func(t *testing.T) {
			catalog := asSnapshot(catalogs.NewEmpty())
			store := &pipelineTestStore{catalog: catalog}
			runner := newStubPipeline(store, nil)
			runner.createSources = func(*pkgsync.Options, catalogInputs) []sources.Source {
				return []sources.Source{&lifecycleTestSource{id: sources.LocalCatalogID, catalog: catalog}, &invalidObservationSource{&lifecycleTestSource{id: sources.ModelsDevHTTPID, catalog: catalog}}}
			}
			runner.observe = observe
			runner.reconcile = func(ctx context.Context, current *catalogs.Catalog, observations []sources.Observation) (*reconciler.Result, error) {
				if fresh {
					t.Fatal("fresh reset reached reconciliation after an invalid observation")
				}
				return reconciler.ReconcileObservations(ctx, current, observations)
			}
			result, err := runner.Sync(t.Context(), pkgsync.WithDryRun(true), pkgsync.WithFresh(fresh))
			if fresh {
				if err == nil {
					t.Fatal("fresh reset ignored invalid observation")
				}
				var rejected *pkgerrors.ResourceError
				if !stderrors.As(err, &rejected) || rejected.ID != string(sources.ModelsDevHTTPID) {
					t.Fatalf("fresh error lost source identity: %v", err)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if !result.Partial {
				t.Fatal("invalid source observation reported complete success")
			}
			if len(result.SourceFailures) != 1 || result.SourceFailures[0] != (sources.SourceFailure{Source: sources.ModelsDevHTTPID, Reason: sources.ProviderReasonResponseInvalid}) {
				t.Fatalf("source failures=%v", result.SourceFailures)
			}
			if len(result.SourceObservations) != 1 || result.SourceObservations[0].Source != sources.LocalCatalogID {
				t.Fatal("invalid observation became accepted evidence")
			}
		})
	}
}
