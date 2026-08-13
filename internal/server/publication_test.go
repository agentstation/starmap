package server

import (
	"context"
	stderrors "errors"
	"fmt"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/rs/zerolog"

	"github.com/agentstation/starmap"
	"github.com/agentstation/starmap/pkg/catalogs"
	"github.com/agentstation/starmap/pkg/catalogs/storage"
	pkgerrors "github.com/agentstation/starmap/pkg/errors"
)

type publicationFaultStore struct {
	*storage.Memory
	fail atomic.Bool
}

func (s *publicationFaultStore) Commit(
	ctx context.Context,
	generation catalogs.Generation,
	expected string,
) error {
	if s.fail.Load() {
		return &pkgerrors.IOError{
			Operation: "commit", Path: "publication-test", Err: stderrors.New("injected"),
		}
	}
	return s.Memory.Commit(ctx, generation, expected)
}

func TestCacheGenerationEventMatchesAtomicPublicationAndFailedCommitChangesNeither(t *testing.T) {
	store := &publicationFaultStore{Memory: storage.NewMemory()}
	var phase atomic.Int32
	update := serverCatalogUpdate(func(candidate *catalogs.Builder) error {
		id := catalogs.ProviderID("published-one")
		if phase.Add(1) > 1 {
			id = "failed-two"
		}
		return candidate.SetProvider(catalogs.Provider{ID: id, Name: string(id)})
	})
	client, err := starmap.New(starmap.WithCatalogStore(store))
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	logger := zerolog.Nop()
	server, err := New(&mockApplication{logger: &logger, sm: client}, Config{
		PathPrefix: "/api/v1", CacheTTL: time.Minute,
	})
	if err != nil {
		t.Fatalf("New server: %v", err)
	}
	server.Start()
	t.Cleanup(func() { _ = server.Shutdown(context.Background()) })
	stream := openPublicationStream(t, server)

	initial := client.CurrentCatalogState()
	server.cache.SetGeneration(initial.Sequence, initial.GenerationID, "models", "old")
	if _, err := client.Update(context.Background(), update); err != nil {
		t.Fatalf("Update: %v", err)
	}
	published := client.CurrentCatalogState()
	event := stream.wait(t)
	if event.GenerationID != published.GenerationID || event.Sequence != published.Sequence {
		t.Fatalf("publication event = %#v, state = %#v", event, published)
	}
	current, err := store.Current(context.Background())
	if err != nil {
		t.Fatalf("Current: %v", err)
	}
	if server.cache.GenerationID() != published.GenerationID ||
		server.cache.GetStats().Sequence != published.Sequence ||
		server.cache.ItemCount() != 0 {
		t.Fatalf(
			"event/cache/current mismatch: event=%#v cache=%#v current=%#v",
			event,
			server.cache.GetStats(),
			current.Manifest,
		)
	}

	store.fail.Store(true)
	if _, err := client.Update(context.Background(), update); err == nil {
		t.Fatal("faulted commit succeeded")
	}
	if after := client.CurrentCatalogState(); after.GenerationID != published.GenerationID ||
		after.Sequence != published.Sequence {
		t.Fatalf("failed commit changed state: %#v -> %#v", published, after)
	}
	if stats := server.cache.GetStats(); stats.GenerationID != published.GenerationID ||
		stats.Sequence != published.Sequence {
		t.Fatalf("failed commit changed cache: %#v", stats)
	}
	stream.assertNone(t, 50*time.Millisecond)
}

func TestCatalogPublicationEventsAndCacheCannotReorder(t *testing.T) {
	store := storage.NewMemory()
	var phase atomic.Int32
	update := serverCatalogUpdate(func(candidate *catalogs.Builder) error {
		index := phase.Add(1)
		id := catalogs.ProviderID(fmt.Sprintf("ordered-%d", index))
		return candidate.SetProvider(catalogs.Provider{ID: id, Name: string(id)})
	})
	client, err := starmap.New(starmap.WithCatalogStore(store))
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	firstStarted := make(chan struct{})
	releaseFirst := make(chan struct{})
	var releaseOnce sync.Once
	t.Cleanup(func() { releaseOnce.Do(func() { close(releaseFirst) }) })
	client.OnCatalogPublished(func(event starmap.CatalogPublishedEvent) error {
		if event.Sequence == 2 {
			close(firstStarted)
			<-releaseFirst
		}
		return nil
	})

	logger := zerolog.Nop()
	server, err := New(&mockApplication{logger: &logger, sm: client}, Config{
		PathPrefix: "/api/v1", CacheTTL: time.Minute,
	})
	if err != nil {
		t.Fatalf("New server: %v", err)
	}
	server.Start()
	t.Cleanup(func() { _ = server.Shutdown(context.Background()) })
	stream := openPublicationStream(t, server)

	if _, err := client.Update(context.Background(), update); err != nil {
		t.Fatalf("first Update: %v", err)
	}
	select {
	case <-firstStarted:
	case <-time.After(time.Second):
		t.Fatal("first publication hook did not start")
	}
	first := stream.wait(t)
	if first.Sequence != 2 {
		t.Fatalf("first event sequence = %d, want 2", first.Sequence)
	}

	if _, err := client.Update(context.Background(), update); err != nil {
		t.Fatalf("second Update: %v", err)
	}
	stream.assertNone(t, 25*time.Millisecond)
	releaseOnce.Do(func() { close(releaseFirst) })

	second := stream.wait(t)
	if second.Sequence != 3 {
		t.Fatalf("second event sequence = %d, want 3", second.Sequence)
	}
	state := client.CurrentCatalogState()
	current, err := store.Current(context.Background())
	if err != nil {
		t.Fatalf("Current: %v", err)
	}
	if server.cache.GetStats().Sequence != state.Sequence ||
		server.cache.GenerationID() != state.GenerationID ||
		current.Manifest.GenerationID != state.GenerationID {
		t.Fatalf(
			"final ordering mismatch: cache=%#v state=%#v current=%#v",
			server.cache.GetStats(),
			state,
			current.Manifest,
		)
	}
}
