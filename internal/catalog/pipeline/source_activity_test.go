package pipeline

import (
	"context"
	"encoding/json"
	stderrors "errors"
	"strings"
	"testing"
	"time"

	"github.com/agentstation/starmap/internal/catalog/reconciler"
	"github.com/agentstation/starmap/pkg/catalogs"
	pkgerrors "github.com/agentstation/starmap/pkg/errors"
	"github.com/agentstation/starmap/pkg/sources"
	pkgsync "github.com/agentstation/starmap/pkg/sync"
)

func TestPipelineReportsSourceEligibilityAndActualAttempt(t *testing.T) {
	catalog := asSnapshot(catalogs.NewEmpty())
	local := &lifecycleTestSource{id: sources.LocalCatalogID, catalog: catalog}
	unavailable := &lifecycleTestSource{id: sources.ModelsDevGitID, catalog: catalog}
	runner := newStubPipeline(&pipelineTestStore{catalog: catalog}, nil)
	runner.createSources = func(*pkgsync.Options, catalogInputs) []sources.Source { return []sources.Source{local, unavailable} }
	runner.resolveDependencies = func(context.Context, []sources.Source, *pkgsync.Options) ([]sources.Source, []error, error) {
		return []sources.Source{local}, []error{&pkgerrors.DependencyError{Source: string(sources.ModelsDevGitID), Message: "fixture unavailable"}}, nil
	}
	runner.observe = observe
	runner.reconcile = func(ctx context.Context, current *catalogs.Catalog, observations []sources.Observation) (*reconciler.Result, error) {
		return reconciler.ReconcileObservations(ctx, current, observations)
	}
	result, err := runner.Sync(t.Context(), pkgsync.WithDryRun(true), pkgsync.WithSources(sources.LocalCatalogID, sources.ModelsDevGitID), pkgsync.WithModelsDevGitCommit(strings.Repeat("a", 40)))
	if err != nil {
		t.Fatal(err)
	}
	raw, err := json.Marshal(result)
	if err != nil {
		t.Fatal(err)
	}
	var report struct {
		Sources []struct {
			Source      sources.ID `json:"source"`
			Supported   bool       `json:"supported"`
			Enabled     bool       `json:"enabled"`
			Eligibility string     `json:"eligibility"`
			Attempted   bool       `json:"attempted"`
		} `json:"source_activities"`
	}
	if err := json.Unmarshal(raw, &report); err != nil {
		t.Fatal(err)
	}
	if len(report.Sources) != 4 {
		t.Fatalf("source inventory length=%d, want four acquisition sources", len(report.Sources))
	}
	for _, state := range report.Sources {
		if !state.Supported {
			t.Fatalf("registered source unsupported: %s", state.Source)
		}
		switch state.Source {
		case sources.LocalCatalogID:
			if !state.Enabled || state.Eligibility != "eligible" || !state.Attempted {
				t.Fatalf("local state=%+v", state)
			}
		case sources.ModelsDevGitID:
			if !state.Enabled || state.Eligibility != "ineligible" || state.Attempted {
				t.Fatalf("missing dependency state=%+v", state)
			}
		default:
			if state.Enabled || state.Attempted {
				t.Fatalf("disabled source state=%+v", state)
			}
		}
	}
	calls, _, _ := unavailable.counts()
	if calls != 0 {
		t.Fatal("ineligible source ran")
	}
}

func TestPipelineSourceActivitySurvivesFatalPreflight(t *testing.T) {
	catalog := asSnapshot(catalogs.NewEmpty())
	runner := newStubPipeline(&pipelineTestStore{catalog: catalog}, nil)
	failure := &pkgerrors.DependencyError{Source: string(sources.ModelsDevGitID), Message: "fixture unavailable"}
	runner.resolveDependencies = func(context.Context, []sources.Source, *pkgsync.Options) ([]sources.Source, []error, error) {
		return nil, nil, failure
	}
	_, err := runner.Prepare(t.Context(), catalog, pkgsync.WithSources(sources.ModelsDevGitID), pkgsync.WithModelsDevGitCommit(strings.Repeat("a", 40)))
	if !stderrors.Is(err, failure) {
		t.Fatalf("dependency error identity lost: %v", err)
	}
	report := sources.ActivityFromError(err)
	if len(report) != 4 {
		t.Fatalf("fatal preflight report=%v", report)
	}
	for _, state := range report {
		if state.Source == sources.ModelsDevGitID && (state.Eligibility != sources.EligibilityIneligible || state.Attempted) {
			t.Fatalf("fatal source state=%+v", state)
		}
	}
	report[0].Source = "caller-change"
	if sources.ActivityFromError(err)[0].Source == "caller-change" {
		t.Fatal("error report aliases caller state")
	}
}

type heldActivitySource struct {
	*lifecycleTestSource
	entered chan struct{}
	release chan struct{}
}

func (s *heldActivitySource) Observe(ctx context.Context, opts ...sources.Option) (sources.Observation, error) {
	close(s.entered)
	select {
	case <-ctx.Done():
		return sources.Observation{}, ctx.Err()
	case <-s.release:
		return s.lifecycleTestSource.Observe(ctx, opts...)
	}
}

func TestPipelineSourceActivityOwnsConcurrentReports(t *testing.T) {
	catalog := asSnapshot(catalogs.NewEmpty())
	held := &heldActivitySource{lifecycleTestSource: &lifecycleTestSource{id: sources.LocalCatalogID, catalog: catalog}, entered: make(chan struct{}), release: make(chan struct{})}
	runner := newStubPipeline(&pipelineTestStore{catalog: catalog}, nil)
	runner.createSources = func(opts *pkgsync.Options, _ catalogInputs) []sources.Source {
		if opts.Sources[0] == sources.LocalCatalogID {
			return []sources.Source{held}
		}
		return []sources.Source{&lifecycleTestSource{id: sources.ModelsDevHTTPID, catalog: catalog}}
	}
	runner.observe = observe
	runner.reconcile = func(ctx context.Context, current *catalogs.Catalog, observations []sources.Observation) (*reconciler.Result, error) {
		return reconciler.ReconcileObservations(ctx, current, observations)
	}
	ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
	defer cancel()
	type result struct {
		prepared *Prepared
		err      error
	}
	first := make(chan result, 1)
	go func() {
		prepared, err := runner.Prepare(ctx, catalog, pkgsync.WithSources(sources.LocalCatalogID), pkgsync.WithDryRun(true))
		first <- result{prepared, err}
	}()
	select {
	case <-held.entered:
	case <-ctx.Done():
		t.Fatal(ctx.Err())
	}
	second, err := runner.Prepare(ctx, catalog, pkgsync.WithSources(sources.ModelsDevHTTPID), pkgsync.WithDryRun(true))
	if err != nil {
		t.Fatal(err)
	}
	for _, state := range second.Result.SourceActivities {
		if state.Attempted != (state.Source == sources.ModelsDevHTTPID) {
			t.Fatalf("second run contains another attempt: %+v", state)
		}
	}
	close(held.release)
	select {
	case done := <-first:
		if done.err != nil {
			t.Fatal(done.err)
		}
		for _, state := range done.prepared.Result.SourceActivities {
			if state.Attempted != (state.Source == sources.LocalCatalogID) {
				t.Fatalf("first run contains another attempt: %+v", state)
			}
		}
	case <-ctx.Done():
		t.Fatal(ctx.Err())
	}
}
