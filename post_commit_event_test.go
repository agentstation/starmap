package starmap

import (
	"context"
	stderrors "errors"
	"sync"
	"testing"
	"time"

	"github.com/agentstation/starmap/pkg/catalogs"
	"github.com/agentstation/starmap/pkg/catalogstore"
	pkgerrors "github.com/agentstation/starmap/pkg/errors"
)

func TestPostCommitEventOrdering(t *testing.T) {
	t.Run("failed commit emits no event", func(t *testing.T) {
		store := newCommitGateStore(true)
		client := newPostCommitEventClient(t, store)
		events := make(chan CatalogPublishedEvent, 1)
		client.OnCatalogPublished(func(event CatalogPublishedEvent) error {
			events <- event
			return nil
		})

		done := make(chan error, 1)
		go func() {
			_, updateErr := client.Update(context.Background(), postCommitTestUpdate)
			done <- updateErr
		}()
		<-store.entered
		assertNoCatalogEvent(t, events)
		close(store.release)
		if err := <-done; err == nil {
			t.Fatal("Update succeeded after injected commit failure")
		}
		assertNoCatalogEvent(t, events)
	})

	t.Run("successful commit emits matching asynchronous event", func(t *testing.T) {
		store := newCommitGateStore(false)
		client := newPostCommitEventClient(t, store)
		events := make(chan CatalogPublishedEvent, 1)
		hookStarted := make(chan struct{})
		hookRelease := make(chan struct{})
		var once sync.Once
		client.OnCatalogPublished(func(event CatalogPublishedEvent) error {
			once.Do(func() { close(hookStarted) })
			events <- event
			<-hookRelease
			return nil
		})

		done := make(chan error, 1)
		go func() {
			_, updateErr := client.Update(context.Background(), postCommitTestUpdate)
			done <- updateErr
		}()
		<-store.entered
		assertNoCatalogEvent(t, events)
		close(store.release)

		select {
		case err := <-done:
			if err != nil {
				t.Fatalf("Update: %v", err)
			}
		case <-time.After(time.Second):
			t.Fatal("Update waited for asynchronous publication hook")
		}
		select {
		case <-hookStarted:
		case <-time.After(time.Second):
			t.Fatal("publication hook did not start")
		}
		event := <-events
		current, err := store.Current(context.Background())
		if err != nil {
			t.Fatalf("Current: %v", err)
		}
		if event.GenerationID == "" || event.SyncRunID == "" {
			t.Fatalf("event IDs = (%q, %q), want non-empty", event.GenerationID, event.SyncRunID)
		}
		if event.GenerationID != current.Manifest.GenerationID || event.SyncRunID != current.Manifest.SyncRunID {
			t.Fatalf("event IDs = (%q, %q), manifest = (%q, %q)", event.GenerationID, event.SyncRunID, current.Manifest.GenerationID, current.Manifest.SyncRunID)
		}
		state := client.CurrentCatalogState()
		if event.Sequence != state.Sequence || event.Catalog != state.Catalog ||
			event.GenerationID != state.GenerationID ||
			!state.GeneratedAt.Equal(current.Manifest.GeneratedAt) {
			t.Fatalf("event = %#v, atomic state = %#v", event, state)
		}
		retained, err := store.Get(context.Background(), event.GenerationID)
		if err != nil {
			t.Fatalf("Get published generation: %v", err)
		}
		if retained.Manifest.Payload.Checksum != current.Manifest.Payload.Checksum {
			t.Fatalf(
				"retained checksum = %q, current = %q",
				retained.Manifest.Payload.Checksum,
				current.Manifest.Payload.Checksum,
			)
		}
		close(hookRelease)
	})
}

type commitGateStore struct {
	*catalogstore.Memory
	entered chan struct{}
	release chan struct{}
	fail    bool
	once    sync.Once
}

func newCommitGateStore(fail bool) *commitGateStore {
	return &commitGateStore{
		Memory:  catalogstore.NewMemory(),
		entered: make(chan struct{}),
		release: make(chan struct{}),
		fail:    fail,
	}
}

func (s *commitGateStore) Commit(ctx context.Context, generation catalogs.Generation, expected string) error {
	s.once.Do(func() { close(s.entered) })
	select {
	case <-s.release:
	case <-ctx.Done():
		return ctx.Err()
	}
	if s.fail {
		return &pkgerrors.IOError{Operation: "commit", Path: "fault-store", Err: stderrors.New("injected failure")}
	}
	return s.Memory.Commit(ctx, generation, expected)
}

func newPostCommitEventClient(t testing.TB, store catalogstore.Store) *Client {
	t.Helper()
	client, err := New(
		WithCatalogStore(store),
	)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	return client
}

var postCommitTestUpdate = catalogUpdate(func(catalog *catalogs.Builder) error {
	return catalog.SetProvider(catalogs.Provider{ID: "post-commit", Name: "Post Commit"})
})

func assertNoCatalogEvent(t testing.TB, events <-chan CatalogPublishedEvent) {
	t.Helper()
	select {
	case event := <-events:
		t.Fatalf("unexpected catalog event: %#v", event)
	case <-time.After(25 * time.Millisecond):
	}
}
