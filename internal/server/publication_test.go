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
	"github.com/agentstation/starmap/internal/server/events"
	"github.com/agentstation/starmap/pkg/catalogs"
	"github.com/agentstation/starmap/pkg/catalogstore"
	pkgerrors "github.com/agentstation/starmap/pkg/errors"
)

type publicationFaultStore struct {
	*catalogstore.Memory
	fail atomic.Bool
}

func (s *publicationFaultStore) Commit(ctx context.Context, generation catalogstore.Generation, expected string) error {
	if s.fail.Load() {
		return &pkgerrors.IOError{Operation: "commit", Path: "publication-test", Err: stderrors.New("injected")}
	}
	return s.Memory.Commit(ctx, generation, expected)
}

type eventChannelSubscriber struct {
	events chan events.Event
}

func (s *eventChannelSubscriber) Send(event events.Event) error {
	s.events <- event
	return nil
}

func (s *eventChannelSubscriber) Close() error { return nil }

func TestCacheGenerationEventMatchesAtomicPublicationAndFailedCommitChangesNeither(t *testing.T) {
	store := &publicationFaultStore{Memory: catalogstore.NewMemory()}
	var phase atomic.Int32
	update := serverCatalogUpdate(func(candidate *catalogs.Builder) error {
		id := catalogs.ProviderID("published-one")
		if phase.Add(1) > 1 {
			id = "failed-two"
		}
		if err := candidate.SetProvider(catalogs.Provider{ID: id, Name: string(id)}); err != nil {
			return err
		}
		return nil
	})
	client, err := starmap.New(starmap.WithCatalogStore(store))
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	logger := zerolog.Nop()
	app := &mockApplication{logger: &logger, sm: client}
	server, err := New(app, Config{PathPrefix: "/api/v1", CacheTTL: time.Minute})
	if err != nil {
		t.Fatalf("New server: %v", err)
	}
	server.Start()
	t.Cleanup(func() { _ = server.Shutdown(context.Background()) })
	subscriber := &eventChannelSubscriber{events: make(chan events.Event, 32)}
	server.broker.Subscribe(subscriber)
	deadline := time.Now().Add(time.Second)
	for server.broker.SubscriberCount() < 3 && time.Now().Before(deadline) {
		time.Sleep(time.Millisecond)
	}

	initial := client.CurrentCatalogState()
	server.cache.SetGeneration(initial.Sequence, initial.GenerationID, "models", "old")
	if _, err := client.Update(context.Background(), update); err != nil {
		t.Fatalf("Update: %v", err)
	}
	published := client.CurrentCatalogState()
	event := waitForEventType(t, subscriber.events, events.CatalogPublished)
	data, ok := event.Data.(map[string]any)
	if !ok {
		t.Fatalf("event data = %T", event.Data)
	}
	if data["generation_id"] != published.GenerationID || data["sequence"] != published.Sequence {
		t.Fatalf("publication event = %#v, state = %#v", data, published)
	}
	current, err := store.Current(context.Background())
	if err != nil {
		t.Fatalf("Current: %v", err)
	}
	if data["sync_run_id"] != current.Manifest.SyncRunID || server.cache.GenerationID() != published.GenerationID ||
		server.cache.GetStats().Sequence != published.Sequence || server.cache.ItemCount() != 0 {
		t.Fatalf("event/cache/current mismatch: event=%#v cache=%#v current=%#v", data, server.cache.GetStats(), current.Manifest)
	}

	store.fail.Store(true)
	if _, err := client.Update(context.Background(), update); err == nil {
		t.Fatal("faulted commit succeeded")
	}
	if after := client.CurrentCatalogState(); after.GenerationID != published.GenerationID || after.Sequence != published.Sequence {
		t.Fatalf("failed commit changed state: %#v -> %#v", published, after)
	}
	if stats := server.cache.GetStats(); stats.GenerationID != published.GenerationID || stats.Sequence != published.Sequence {
		t.Fatalf("failed commit changed cache: %#v", stats)
	}
	assertNoEventType(t, subscriber.events, events.CatalogPublished, 50*time.Millisecond)
}

func TestCatalogPublicationEventsAndCacheCannotReorder(t *testing.T) {
	store := catalogstore.NewMemory()
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
	subscriber := &eventChannelSubscriber{events: make(chan events.Event, 32)}
	server.broker.Subscribe(subscriber)
	deadline := time.Now().Add(time.Second)
	for server.broker.SubscriberCount() < 3 && time.Now().Before(deadline) {
		time.Sleep(time.Millisecond)
	}

	if _, err := client.Update(context.Background(), update); err != nil {
		t.Fatalf("first Update: %v", err)
	}
	select {
	case <-firstStarted:
	case <-time.After(time.Second):
		t.Fatal("first publication hook did not start")
	}
	first := waitForEventType(t, subscriber.events, events.CatalogPublished)
	if sequence := publicationEventSequence(t, first); sequence != 2 {
		t.Fatalf("first event sequence = %d, want 2", sequence)
	}

	if _, err := client.Update(context.Background(), update); err != nil {
		t.Fatalf("second Update: %v", err)
	}
	assertNoEventType(t, subscriber.events, events.CatalogPublished, 25*time.Millisecond)
	releaseOnce.Do(func() { close(releaseFirst) })

	second := waitForEventType(t, subscriber.events, events.CatalogPublished)
	if sequence := publicationEventSequence(t, second); sequence != 3 {
		t.Fatalf("second event sequence = %d, want 3", sequence)
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

func publicationEventSequence(t testing.TB, event events.Event) uint64 {
	t.Helper()
	data, ok := event.Data.(map[string]any)
	if !ok {
		t.Fatalf("event data = %T, want map[string]any", event.Data)
	}
	sequence, ok := data["sequence"].(uint64)
	if !ok {
		t.Fatalf("event sequence = %#v, want uint64", data["sequence"])
	}
	return sequence
}

func waitForEventType(t *testing.T, source <-chan events.Event, eventType events.EventType) events.Event {
	t.Helper()
	deadline := time.After(time.Second)
	for {
		select {
		case event := <-source:
			if event.Type == eventType {
				return event
			}
		case <-deadline:
			t.Fatalf("timed out waiting for %s", eventType)
		}
	}
}

func assertNoEventType(t *testing.T, source <-chan events.Event, eventType events.EventType, duration time.Duration) {
	t.Helper()
	timer := time.NewTimer(duration)
	defer timer.Stop()
	for {
		select {
		case event := <-source:
			if event.Type == eventType {
				t.Fatalf("unexpected %s event: %#v", eventType, event)
			}
		case <-timer.C:
			return
		}
	}
}
