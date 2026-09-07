package runtime

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/agentstation/starmap"
	"github.com/agentstation/starmap/pkg/catalogs"
	"github.com/agentstation/starmap/pkg/catalogs/storage"
)

type gatedReconciliationStore struct {
	storage.Store
	mu      sync.Mutex
	entered chan struct{}
	release chan struct{}
}

func (s *gatedReconciliationStore) Commit(ctx context.Context, generation catalogs.Generation, expected string) error {
	s.mu.Lock()
	entered, release := s.entered, s.release
	s.entered = nil
	s.mu.Unlock()
	if entered != nil {
		close(entered)
		select {
		case <-release:
		case <-ctx.Done():
			return ctx.Err()
		}
	}
	return s.Store.Commit(ctx, generation, expected)
}

func TestConcurrentRuntimeRebuildsPublishCompleteGenerationsInOrder(t *testing.T) {
	at := time.Date(2026, 9, 7, 0, 0, 0, 0, time.UTC)
	one := testProviderLayer(t, "concurrent-one", "one", "One", at)
	two := testProviderLayer(t, "concurrent-two", "two", "Two", at.Add(time.Minute))
	store := &gatedReconciliationStore{Store: storage.NewMemory()}
	connected := openTestRuntime(t, WithSource(testReviewedDefinitionsSource(t, []ProviderLayer{one, two})), WithClientOptions(starmap.WithCatalogStore(store)))
	if _, err := connected.RefreshSource(t.Context()); err != nil {
		t.Fatal(err)
	}
	before := connected.State()
	entered, release := make(chan struct{}), make(chan struct{})
	releaseOnce := sync.OnceFunc(func() { close(release) })
	defer releaseOnce()
	store.mu.Lock()
	store.entered, store.release = entered, release
	store.mu.Unlock()
	first := make(chan error, 1)
	go func() {
		_, err := connected.publishProviders(t.Context(), []ProviderLayer{one}, connected.lease.epoch())
		first <- err
	}()
	select {
	case <-entered:
	case <-time.After(10 * time.Second):
		t.Fatal("first commit did not enter store")
	}
	if connected.State().GenerationID != before.GenerationID || connected.Client().CurrentGenerationID() != before.GenerationID {
		t.Fatal("pending durable commit exposed an unaccepted generation")
	}
	if err := connected.retainProviders(t.Context(), []ProviderLayer{two}); err != nil {
		t.Fatal(err)
	}
	second := make(chan error, 1)
	go func() { _, err := connected.rebuild(t.Context(), connected.lease.epoch()); second <- err }()
	releaseOnce()
	for _, done := range []<-chan error{first, second} {
		select {
		case err := <-done:
			if err != nil {
				t.Fatal(err)
			}
		case <-time.After(10 * time.Second):
			t.Fatal("rebuild did not complete")
		}
	}
	current, err := store.Current(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	if connected.State().GenerationID != current.Manifest.GenerationID || connected.Client().CurrentGenerationID() != current.Manifest.GenerationID {
		t.Fatal("concurrent rebuilds activated different durable and served generations")
	}
	for _, layer := range []ProviderLayer{one, two} {
		provider, err := connected.Catalog().Provider(layer.ProviderID)
		if err != nil || len(provider.Models) != 1 {
			t.Fatal("concurrent rebuild discarded a retained provider")
		}
	}
	if len(current.Manifest.SourceObservations) != 2 {
		t.Fatal("final generation lost concurrent observation receipts")
	}
}
