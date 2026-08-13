package consumer

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/agentstation/starmap"
	"github.com/agentstation/starmap/pkg/catalogs"
	"github.com/agentstation/starmap/pkg/catalogs/storage"
)

func TestPublish(t *testing.T) {
	if err := Publish(context.Background()); err != nil {
		t.Fatalf("Publish: %v", err)
	}
}

func TestNewContextConstructsStorageBackedClient(t *testing.T) {
	client, err := starmap.NewContext(
		context.Background(),
		starmap.WithCatalogStore(storage.NewMemory()),
	)
	if err != nil {
		t.Fatalf("NewContext: %v", err)
	}
	if client.Catalog() == nil {
		t.Fatal("Catalog returned nil after successful construction")
	}
}

func TestNewContextPropagatesCancellationToStore(t *testing.T) {
	store := &blockingStore{started: make(chan struct{})}
	ctx, cancel := context.WithCancel(context.Background())
	result := make(chan error, 1)
	go func() {
		_, err := starmap.NewContext(ctx, starmap.WithCatalogStore(store))
		result <- err
	}()

	select {
	case <-store.started:
	case <-time.After(10 * time.Second):
		t.Fatal("store Current was not called")
	}
	cancel()

	select {
	case err := <-result:
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("NewContext error = %v, want context.Canceled", err)
		}
	case <-time.After(10 * time.Second):
		t.Fatal("NewContext did not return after cancellation")
	}
}

func TestNewContextHonorsExpiredDeadline(t *testing.T) {
	ctx, cancel := context.WithDeadline(context.Background(), time.Now().Add(-time.Second))
	defer cancel()

	client, err := starmap.NewContext(
		ctx,
		starmap.WithCatalogStore(storage.NewMemory()),
	)
	if client != nil {
		t.Fatal("NewContext returned a client after its deadline")
	}
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("NewContext error = %v, want context.DeadlineExceeded", err)
	}
}

func TestNewContextRejectsNilContext(t *testing.T) {
	client, err := starmap.NewContext(
		nil,
		starmap.WithCatalogStore(storage.NewMemory()),
	)
	if client != nil {
		t.Fatal("NewContext returned a client for a nil context")
	}
	if err == nil {
		t.Fatal("NewContext returned nil error for a nil context")
	}
}

func TestNilClientCatalogIsDefined(t *testing.T) {
	var client *starmap.Client
	if catalog := client.Catalog(); catalog != nil {
		t.Fatalf("nil Client.Catalog() = %#v, want nil", catalog)
	}
}

type blockingStore struct {
	started chan struct{}
}

func (s *blockingStore) Current(ctx context.Context) (catalogs.Generation, error) {
	close(s.started)
	<-ctx.Done()
	return catalogs.Generation{}, ctx.Err()
}

func (*blockingStore) Get(context.Context, string) (catalogs.Generation, error) {
	return catalogs.Generation{}, errors.New("unexpected Get call")
}

func (*blockingStore) Commit(context.Context, catalogs.Generation, string) error {
	return errors.New("unexpected Commit call")
}
