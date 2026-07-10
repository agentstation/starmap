package starmap

import (
	"context"
	stderrors "errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"

	"github.com/agentstation/starmap/pkg/catalogs"
	"github.com/agentstation/starmap/pkg/catalogscheduler"
	"github.com/agentstation/starmap/pkg/catalogstore"
	pkgerrors "github.com/agentstation/starmap/pkg/errors"
	pkgsync "github.com/agentstation/starmap/pkg/sync"
)

type lastKnownGoodFaultStore struct {
	*catalogstore.Memory
	fail    atomic.Bool
	entered chan struct{}
	release chan struct{}
	once    sync.Once
}

func (s *lastKnownGoodFaultStore) Commit(ctx context.Context, generation catalogstore.Generation, expected string) error {
	if !s.fail.Load() {
		return s.Memory.Commit(ctx, generation, expected)
	}
	s.once.Do(func() { close(s.entered) })
	select {
	case <-s.release:
	case <-ctx.Done():
		return ctx.Err()
	}
	return &pkgerrors.IOError{
		Operation: "commit", Path: "last-known-good-fault", Message: "injected transient failure",
		Err: pkgerrors.ErrProviderUnavailable,
	}
}

type updateSyncer struct {
	client *Client
	store  catalogstore.Store
}

const lastKnownGoodCommitGateTimeout = 30 * time.Second

func (s updateSyncer) Sync(ctx context.Context, _ ...pkgsync.Option) (*pkgsync.Result, error) {
	if err := s.client.Update(ctx); err != nil {
		return nil, err
	}
	current, err := s.store.Current(ctx)
	if err != nil {
		return nil, err
	}
	return &pkgsync.Result{
		GenerationID:       current.Manifest.GenerationID,
		SyncRunID:          current.Manifest.SyncRunID,
		SourceObservations: append([]catalogs.SourceObservationLink(nil), current.Manifest.SourceObservations...),
	}, nil
}

func TestSchedulerLastKnownGoodSurvivesFailedRefreshAndRetry(t *testing.T) {
	store := &lastKnownGoodFaultStore{
		Memory: catalogstore.NewMemory(), entered: make(chan struct{}), release: make(chan struct{}),
	}
	var updateCalls atomic.Int32
	client, err := New(
		WithCatalogStore(store),
		WithUpdateFunc(func(_ context.Context, candidate *catalogs.Builder) (*catalogs.Builder, error) {
			if updateCalls.Add(1) == 1 {
				if err := candidate.SetProvider(catalogs.Provider{ID: "last-known-good", Name: "Last Known Good"}); err != nil {
					return nil, err
				}
				return candidate, nil
			}
			if err := candidate.DeleteProvider("last-known-good"); err != nil {
				return nil, err
			}
			if err := candidate.SetProvider(catalogs.Provider{ID: "failed-candidate", Name: "Failed Candidate"}); err != nil {
				return nil, err
			}
			return candidate, nil
		}),
	)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	if err := client.Update(context.Background()); err != nil {
		t.Fatalf("establish last known good: %v", err)
	}
	beforeCatalog := client.Catalog()
	beforeGeneration, err := store.Current(context.Background())
	if err != nil {
		t.Fatalf("Current before fault: %v", err)
	}
	if _, err := beforeCatalog.Provider("last-known-good"); err != nil {
		t.Fatalf("last-known-good provider: %v", err)
	}

	ledger := catalogscheduler.NewMemoryRunLedger()
	runner, err := catalogscheduler.NewRunner(
		updateSyncer{client: client, store: store}, catalogscheduler.NewMemoryLease(),
		catalogscheduler.LeaseRequest{Key: catalogscheduler.DefaultLeaseKey, Owner: "last-known-good-replica", TTL: catalogscheduler.DefaultLeaseTTL},
		catalogscheduler.WithRetryPolicy(catalogscheduler.RetryPolicy{
			MaxAttempts: 2, BaseDelay: time.Millisecond, MaxDelay: time.Millisecond,
		}),
		catalogscheduler.WithRunLedger(ledger, client),
	)
	if err != nil {
		t.Fatalf("NewRunner: %v", err)
	}
	store.fail.Store(true)
	releaseStore := sync.OnceFunc(func() { close(store.release) })
	defer releaseStore()
	type runOutcome struct {
		result catalogscheduler.RunResult
		err    error
	}
	outcomes := make(chan runOutcome, 1)
	go func() {
		result, runErr := runner.RunScheduledOnce(context.Background(), 0)
		outcomes <- runOutcome{result: result, err: runErr}
	}()
	commitGateTimer := time.NewTimer(lastKnownGoodCommitGateTimeout)
	defer commitGateTimer.Stop()
	select {
	case <-store.entered:
	case outcome := <-outcomes:
		t.Fatalf("scheduled refresh exited before commit gate: %#v/%v", outcome.result, outcome.err)
	case <-commitGateTimer.C:
		t.Fatalf("failed candidate did not reach commit gate within %s", lastKnownGoodCommitGateTimeout)
	}

	var readers sync.WaitGroup
	for range 8 {
		readers.Add(1)
		go func() {
			defer readers.Done()
			for range 25 {
				catalog := client.Catalog()
				if _, err := catalog.Provider("last-known-good"); err != nil {
					t.Errorf("reader lost last-known-good provider: %v", err)
					return
				}
				if _, err := catalog.Provider("failed-candidate"); err == nil {
					t.Error("reader observed uncommitted failed candidate")
					return
				}
			}
		}()
	}
	readers.Wait()
	releaseStore()
	outcome := <-outcomes
	result, runErr := outcome.result, outcome.err
	if !stderrors.Is(runErr, pkgerrors.ErrProviderUnavailable) || result.Status != catalogscheduler.RunStatusFailed || result.Attempts != 2 {
		t.Fatalf("scheduled failed refresh = %#v/%v", result, runErr)
	}

	afterGeneration, err := store.Current(context.Background())
	if err != nil {
		t.Fatalf("Current after fault: %v", err)
	}
	if diff := cmp.Diff(beforeGeneration, afterGeneration); diff != "" {
		t.Fatalf("failed refresh changed durable generation (-before +after):\n%s", diff)
	}
	if client.Catalog() != beforeCatalog || client.CurrentGenerationID() != beforeGeneration.Manifest.GenerationID {
		t.Fatalf("failed refresh changed published identity/pointer: %q", client.CurrentGenerationID())
	}
	if _, err := client.Catalog().Provider("last-known-good"); err != nil {
		t.Fatalf("last known good unavailable after failure: %v", err)
	}
	if _, err := client.Catalog().Provider("failed-candidate"); err == nil {
		t.Fatal("failed candidate replaced the current catalog")
	}
	retained, err := store.Get(context.Background(), beforeGeneration.Manifest.GenerationID)
	if err != nil {
		t.Fatalf("Get retained last-known-good generation: %v", err)
	}
	if diff := cmp.Diff(beforeGeneration, retained); diff != "" {
		t.Fatalf("retained generation changed (-want +got):\n%s", diff)
	}
	record, err := ledger.Get(context.Background(), result.RunID)
	if err != nil {
		t.Fatalf("Get failed run: %v", err)
	}
	if record.Status != catalogscheduler.RunStatusFailed || record.BaseGenerationID != beforeGeneration.Manifest.GenerationID ||
		record.PublishedGenerationID != "" || len(record.Attempts) != 2 {
		t.Fatalf("failed run record = %#v", record)
	}
}
