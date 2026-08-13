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
	"github.com/agentstation/starmap/pkg/catalogs/storage"
	pkgerrors "github.com/agentstation/starmap/pkg/errors"
)

type lastKnownGoodFaultStore struct {
	*storage.Memory
	fail    atomic.Bool
	entered chan struct{}
	release chan struct{}
	once    sync.Once
}

func (s *lastKnownGoodFaultStore) Commit(ctx context.Context, generation catalogs.Generation, expected string) error {
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

const lastKnownGoodCommitGateTimeout = 30 * time.Second

func TestLastKnownGoodSurvivesFailedUpdateAndPublishesRetry(t *testing.T) {
	store := &lastKnownGoodFaultStore{
		Memory: storage.NewMemory(), entered: make(chan struct{}), release: make(chan struct{}),
	}
	var updateCalls atomic.Int32
	update := catalogUpdate(func(candidate *catalogs.Builder) error {
		if updateCalls.Add(1) == 1 {
			if err := candidate.SetProvider(catalogs.Provider{ID: "last-known-good", Name: "Last Known Good"}); err != nil {
				return err
			}
			return nil
		}
		if err := candidate.DeleteProvider("last-known-good"); err != nil {
			return err
		}
		if err := candidate.SetProvider(catalogs.Provider{ID: "failed-candidate", Name: "Failed Candidate"}); err != nil {
			return err
		}
		return nil
	})
	client, err := New(WithCatalogStore(store))
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	if _, err := client.Update(context.Background(), update); err != nil {
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

	store.fail.Store(true)
	releaseStore := sync.OnceFunc(func() { close(store.release) })
	defer releaseStore()
	outcomes := make(chan error, 1)
	go func() {
		_, updateErr := client.Update(context.Background(), update)
		outcomes <- updateErr
	}()
	commitGateTimer := time.NewTimer(lastKnownGoodCommitGateTimeout)
	defer commitGateTimer.Stop()
	select {
	case <-store.entered:
	case updateErr := <-outcomes:
		t.Fatalf("update exited before commit gate: %v", updateErr)
	case <-commitGateTimer.C:
		t.Fatalf("failed candidate did not reach commit gate within %s", lastKnownGoodCommitGateTimeout)
	}

	var readers sync.WaitGroup
	for range 8 {
		readers.Go(func() {
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
		})
	}
	readers.Wait()
	releaseStore()
	updateErr := <-outcomes
	if !stderrors.Is(updateErr, pkgerrors.ErrProviderUnavailable) {
		t.Fatalf("failed update error = %v", updateErr)
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

	store.fail.Store(false)
	if _, err := client.Update(context.Background(), update); err != nil {
		t.Fatalf("retry update: %v", err)
	}
	if client.CurrentGenerationID() == beforeGeneration.Manifest.GenerationID {
		t.Fatal("successful retry did not publish a new generation")
	}
	if _, err := client.Catalog().Provider("failed-candidate"); err != nil {
		t.Fatalf("retry candidate was not published: %v", err)
	}
	if _, err := client.Catalog().Provider("last-known-good"); err == nil {
		t.Fatal("successful retry retained superseded provider")
	}
}
