package permission

import (
	"context"
	stderrors "errors"
	"sync"
	"testing"
	"time"

	"github.com/agentstation/starmap/pkg/catalogs"
	"github.com/agentstation/starmap/pkg/catalogs/storage"
	"github.com/agentstation/starmap/pkg/errors"
)

type publicationCurrentHook struct {
	storage.Store
	read func(context.Context) (catalogs.Generation, error)
}

func (s publicationCurrentHook) Current(ctx context.Context) (catalogs.Generation, error) {
	return s.read(ctx)
}

func TestPublisherConcurrentWritersUseStoreCAS(t *testing.T) {
	store := storage.NewMemory()
	first := issuerGeneration(t, "first-writer-base", 1)
	if err := store.Commit(t.Context(), first, ""); err != nil {
		t.Fatal(err)
	}
	ready, release := make(chan struct{}), make(chan struct{})
	waiting := publicationCurrentHook{Store: store, read: func(ctx context.Context) (catalogs.Generation, error) {
		current, err := store.Current(ctx)
		close(ready)
		select {
		case <-release:
			return current, err
		case <-ctx.Done():
			return catalogs.Generation{}, ctx.Err()
		}
	}}
	firstWriter, secondWriter := newTestPublisher(t, waiting), newTestPublisher(t, store)
	stale := issuerGeneration(t, "delayed-writer", 2)
	latest := issuerGeneration(t, "winning-writer", 3)
	ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
	var group sync.WaitGroup
	t.Cleanup(func() { cancel(); group.Wait() })
	result := make(chan error, 1)
	group.Go(func() { result <- firstWriter.Commit(ctx, stale, first.Manifest.GenerationID) })
	select {
	case <-ready:
	case <-ctx.Done():
		t.Fatal(ctx.Err())
	}
	if err := secondWriter.Commit(ctx, latest, first.Manifest.GenerationID); err != nil {
		t.Fatal(err)
	}
	close(release)
	select {
	case err := <-result:
		var conflict *errors.ConflictError
		if !stderrors.As(err, &conflict) {
			t.Fatalf("delayed writer error=%v", err)
		}
	case <-ctx.Done():
		t.Fatal(ctx.Err())
	}
	current, err := store.Current(ctx)
	if err != nil || current.Manifest.AuthorityHead != latest.Manifest.AuthorityHead {
		t.Fatalf("head=%+v error=%v", current.Manifest.AuthorityHead, err)
	}
}

func TestPublisherUnreadablePredecessorCannotResetSequence(t *testing.T) {
	store := storage.NewMemory()
	current := issuerGeneration(t, "unreadable-predecessor", 2)
	if err := store.Commit(t.Context(), current, ""); err != nil {
		t.Fatal(err)
	}
	reader := publicationCurrentHook{Store: store, read: func(context.Context) (catalogs.Generation, error) {
		return catalogs.Generation{}, &errors.NotFoundError{Resource: "catalog generation", ID: current.Manifest.GenerationID}
	}}
	publisher := newTestPublisher(t, reader)
	older := issuerGeneration(t, "attempted-reset", 1)
	for _, expected := range []string{"", current.Manifest.GenerationID} {
		if err := publisher.Commit(t.Context(), older, expected); err == nil {
			t.Fatal("unreadable predecessor allowed an authority sequence reset")
		}
	}
	got, err := store.Current(t.Context())
	if err != nil || got.Manifest.AuthorityHead != current.Manifest.AuthorityHead {
		t.Fatalf("head=%+v error=%v", got.Manifest.AuthorityHead, err)
	}
}
