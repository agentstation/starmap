package starmap

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/agentstation/starmap/pkg/catalogs"
	"github.com/agentstation/starmap/pkg/catalogs/storage"
)

func TestQueuedUpdateHonorsContextCancellation(t *testing.T) {
	release := make(chan struct{})
	entered := make(chan struct{}, 1)
	var calls atomic.Int32
	update := func(ctx context.Context, catalog *catalogs.Catalog) (*Candidate, error) {
		calls.Add(1)
		entered <- struct{}{}
		select {
		case <-release:
			return NewCandidate(catalog, CandidateEvidence{})
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}
	client := &Client{
		options: &options{
			catalogStore: storage.NewMemory(),
		},
		catalog: mustTestCatalog(t, catalogs.NewEmpty()),
		hooks:   newHooks(),
	}

	firstDone := make(chan error, 1)
	go func() {
		_, err := client.Update(context.Background(), update)
		firstDone <- err
	}()
	select {
	case <-entered:
	case <-time.After(time.Second):
		t.Fatal("first update did not enter")
	}

	ctx, cancel := context.WithCancel(context.Background())
	secondDone := make(chan error, 1)
	go func() {
		_, err := client.Update(ctx, update)
		secondDone <- err
	}()
	cancel()

	select {
	case err := <-secondDone:
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("queued Update error = %v, want context.Canceled", err)
		}
	case <-time.After(time.Second):
		t.Fatal("queued Update did not honor context cancellation")
	}
	if got := calls.Load(); got != 1 {
		t.Fatalf("update callback calls before release = %d, want 1", got)
	}

	close(release)
	if err := <-firstDone; err != nil {
		t.Fatalf("first Update returned error: %v", err)
	}
}

func TestConcurrentUpdatesAreSerialized(t *testing.T) {
	release := make(chan struct{})
	entered := make(chan struct{}, 2)
	var active atomic.Int32
	var maxActive atomic.Int32
	update := func(_ context.Context, catalog *catalogs.Catalog) (*Candidate, error) {
		current := active.Add(1)
		defer active.Add(-1)
		for {
			maximum := maxActive.Load()
			if current <= maximum || maxActive.CompareAndSwap(maximum, current) {
				break
			}
		}
		entered <- struct{}{}
		<-release
		return NewCandidate(catalog, CandidateEvidence{})
	}

	client := &Client{
		options: &options{
			catalogStore: storage.NewMemory(),
		},
		catalog: mustTestCatalog(t, catalogs.NewEmpty()),
		hooks:   newHooks(),
	}

	start := make(chan struct{})
	var wg sync.WaitGroup
	errs := make(chan error, 2)
	for range 2 {
		wg.Go(func() {
			<-start
			_, err := client.Update(context.Background(), update)
			errs <- err
		})
	}
	close(start)

	select {
	case <-entered:
	case <-time.After(time.Second):
		t.Fatal("first update did not enter")
	}

	select {
	case <-entered:
	case <-time.After(100 * time.Millisecond):
	}
	close(release)
	wg.Wait()
	close(errs)
	for err := range errs {
		if err != nil {
			t.Fatalf("Update returned error: %v", err)
		}
	}

	if got := maxActive.Load(); got != 1 {
		t.Fatalf("maximum concurrent updates = %d, want 1", got)
	}
}
