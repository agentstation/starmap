package pipeline

import (
	"context"
	stderrors "errors"
	"testing"
	"time"

	"github.com/agentstation/starmap/internal/catalog/workspace"
	"github.com/agentstation/starmap/pkg/catalogs"
	"github.com/agentstation/starmap/pkg/errors"
	"github.com/agentstation/starmap/pkg/reconciler"
	"github.com/agentstation/starmap/pkg/sources"
	pkgsync "github.com/agentstation/starmap/pkg/sync"
)

func TestPipelineRequireAllSourcesRejectsUnhealthyObservationBeforeReconciliation(t *testing.T) {
	modelCatalog := strictTestCatalog(t, true)
	emptyCatalog := strictTestCatalog(t, false)
	requiredSource := &lifecycleTestSource{id: "required", catalog: modelCatalog}

	tests := []struct {
		name         string
		observations []sources.Observation
	}{
		{
			name: "missing credentials",
			observations: []sources.Observation{strictTestObservation(t, "required", emptyCatalog, sources.ObservationMetadata{
				Completeness: sources.ObservationCompletenessPartial,
				Status:       sources.ObservationStatusDegraded,
				Issues: []sources.ObservationIssue{{
					Scope: sources.ObservationIssueScopeProvider, Code: sources.ObservationIssueCodeMissingCredentials,
					Subject: "provider", Message: "credentials unavailable",
				}},
			})},
		},
		{
			name: "stale fallback",
			observations: []sources.Observation{strictTestObservation(t, "required", modelCatalog, sources.ObservationMetadata{
				Completeness: sources.ObservationCompletenessComplete,
				Status:       sources.ObservationStatusDegraded,
				Issues: []sources.ObservationIssue{{
					Scope: sources.ObservationIssueScopeStaleFallback, Code: sources.ObservationIssueCodeStaleFallback,
					Message: "using stale cache",
				}},
			})},
		},
		{
			name: "record quarantine",
			observations: []sources.Observation{strictTestObservation(t, "required", modelCatalog, sources.ObservationMetadata{
				Completeness: sources.ObservationCompletenessPartial,
				Status:       sources.ObservationStatusDegraded,
				Records:      sources.ObservationRecordCounts{Accepted: 1, Rejected: 1},
				Issues: []sources.ObservationIssue{{
					Scope: sources.ObservationIssueScopeRecord, Code: sources.ObservationIssueCodeInvalidRecord,
					Subject: "provider/invalid", Message: "record rejected",
				}},
			})},
		},
		{
			name: "empty success without issue",
			observations: []sources.Observation{strictTestObservation(t, "required", emptyCatalog, sources.ObservationMetadata{
				Completeness: sources.ObservationCompletenessComplete,
				Status:       sources.ObservationStatusSucceeded,
			})},
		},
		{
			name:         "missing observation",
			observations: nil,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			store := &pipelineTestStore{catalog: modelCatalog}
			runner := newStubPipeline(store, nil)
			runner.createSources = func(
				*pkgsync.Options,
				*catalogs.Catalog,
				catalogs.LoadReport,
				workspace.InputExpectation,
			) []sources.Source {
				return []sources.Source{requiredSource}
			}
			runner.observe = func(context.Context, []sources.Source, []sources.Option) ([]sources.Observation, error) {
				return test.observations, nil
			}
			runner.reconcile = func(context.Context, *catalogs.Catalog, []sources.Observation) (*reconciler.Result, error) {
				t.Fatal("unhealthy required source reached reconciliation")
				return nil, nil
			}

			result, err := runner.Sync(
				context.Background(),
				pkgsync.WithDryRun(true),
				pkgsync.WithRequireAllSources(true),
			)
			if err == nil || result != nil {
				t.Fatalf("Sync = %#v, %v; want strict failure", result, err)
			}
			var syncErr *errors.SyncError
			var validationErr *errors.ValidationError
			if !stderrors.As(err, &syncErr) || !stderrors.As(err, &validationErr) {
				t.Fatalf("error = %T: %v, want typed sync and validation errors", err, err)
			}
			if store.applyCalls != 0 {
				t.Fatalf("strict failure applied %d candidates", store.applyCalls)
			}
		})
	}
}

func TestPipelineRequireAllSourcesAcceptsCompleteNonemptyObservation(t *testing.T) {
	catalog := strictTestCatalog(t, true)
	requiredSource := &lifecycleTestSource{id: "required", catalog: catalog}
	observation := strictTestObservation(t, "required", catalog, sources.ObservationMetadata{
		Completeness: sources.ObservationCompletenessComplete,
		Status:       sources.ObservationStatusSucceeded,
		Records:      sources.ObservationRecordCounts{Accepted: 1},
	})
	reconciled := false
	runner := newStubPipeline(&pipelineTestStore{catalog: catalog}, nil)
	runner.createSources = func(
		*pkgsync.Options,
		*catalogs.Catalog,
		catalogs.LoadReport,
		workspace.InputExpectation,
	) []sources.Source {
		return []sources.Source{requiredSource}
	}
	runner.observe = func(context.Context, []sources.Source, []sources.Option) ([]sources.Observation, error) {
		return []sources.Observation{observation}, nil
	}
	runner.reconcile = func(context.Context, *catalogs.Catalog, []sources.Observation) (*reconciler.Result, error) {
		reconciled = true
		return &reconciler.Result{
			Catalog:           catalogs.NewEmpty(),
			Changeset:         emptyChangeset(),
			ProviderAPICounts: map[catalogs.ProviderID]int{},
			ModelProviderMap:  map[string]catalogs.ProviderID{},
		}, nil
	}

	if _, err := runner.Sync(
		context.Background(),
		pkgsync.WithDryRun(true),
		pkgsync.WithRequireAllSources(true),
	); err != nil {
		t.Fatalf("Sync: %v", err)
	}
	if !reconciled {
		t.Fatal("healthy required observation did not reach reconciliation")
	}
}

func strictTestCatalog(t *testing.T, withModel bool) *catalogs.Catalog {
	t.Helper()
	builder := catalogs.NewEmpty()
	provider := catalogs.Provider{ID: "provider", Name: "Provider"}
	if withModel {
		provider.Models = map[string]*catalogs.Model{
			"model": {ID: "model", Name: "Model"},
		}
	}
	if err := builder.SetProvider(provider); err != nil {
		t.Fatalf("SetProvider: %v", err)
	}
	catalog, err := builder.Build()
	if err != nil {
		t.Fatalf("Build: %v", err)
	}
	return catalog
}

func strictTestObservation(
	t *testing.T,
	sourceID sources.ID,
	catalog *catalogs.Catalog,
	metadata sources.ObservationMetadata,
) sources.Observation {
	t.Helper()
	metadata.ObservedAt = time.Date(2026, time.July, 28, 18, 0, 0, 0, time.UTC)
	metadata.Revision = sources.Revision{Kind: sources.RevisionKindContentDigest}
	observation, err := sources.NewObservation(sourceID, catalog, metadata)
	if err != nil {
		t.Fatalf("NewObservation: %v", err)
	}
	return observation
}
