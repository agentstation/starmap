package starmap

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/agentstation/starmap/pkg/catalogs"
	"github.com/agentstation/starmap/pkg/catalogstore"
)

func TestQueuedUpdateHonorsContextCancellation(t *testing.T) {
	release := make(chan struct{})
	entered := make(chan struct{}, 1)
	var calls atomic.Int32
	client := &Client{
		options: &options{
			catalogStore: catalogstore.NewMemory(),
			updateFunc: func(_ context.Context, catalog *catalogs.Builder) (*catalogs.Builder, error) {
				calls.Add(1)
				entered <- struct{}{}
				<-release
				return catalog, nil
			},
		},
		catalog: mustTestCatalog(t, catalogs.NewEmpty()),
		hooks:   newHooks(),
	}

	firstDone := make(chan error, 1)
	go func() {
		firstDone <- client.Update(context.Background())
	}()
	select {
	case <-entered:
	case <-time.After(time.Second):
		t.Fatal("first update did not enter")
	}

	ctx, cancel := context.WithCancel(context.Background())
	secondDone := make(chan error, 1)
	go func() {
		secondDone <- client.Update(ctx)
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

	client := &Client{
		options: &options{
			catalogStore: catalogstore.NewMemory(),
			updateFunc: func(_ context.Context, catalog *catalogs.Builder) (*catalogs.Builder, error) {
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
				return catalog, nil
			},
		},
		catalog: mustTestCatalog(t, catalogs.NewEmpty()),
		hooks:   newHooks(),
	}

	start := make(chan struct{})
	var wg sync.WaitGroup
	errs := make(chan error, 2)
	for range 2 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			errs <- client.Update(context.Background())
		}()
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
