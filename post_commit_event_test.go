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
		go func() { done <- client.Update(context.Background()) }()
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
		go func() { done <- client.Update(context.Background()) }()
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

func (s *commitGateStore) Commit(ctx context.Context, generation catalogstore.Generation, expected string) error {
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
		WithUpdateFunc(func(_ context.Context, catalog *catalogs.Builder) (*catalogs.Builder, error) {
			if err := catalog.SetProvider(catalogs.Provider{ID: "post-commit", Name: "Post Commit"}); err != nil {
				return nil, err
			}
			return catalog, nil
		}),
	)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	return client
}

func assertNoCatalogEvent(t testing.TB, events <-chan CatalogPublishedEvent) {
	t.Helper()
	select {
	case event := <-events:
		t.Fatalf("unexpected catalog event: %#v", event)
	case <-time.After(25 * time.Millisecond):
	}
}
