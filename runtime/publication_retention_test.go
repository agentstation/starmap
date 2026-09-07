package runtime

import (
	"context"
	"io/fs"
	"sync/atomic"
	"testing"
	"time"

	"github.com/agentstation/starmap"
	"github.com/agentstation/starmap/pkg/catalogs"
	"github.com/agentstation/starmap/pkg/catalogs/storage"
)

type retentionRejectingStore struct {
	*storage.Memory
	reject atomic.Bool
}

func (s *retentionRejectingStore) Commit(ctx context.Context, generation catalogs.Generation, expected string) error {
	if s.reject.Load() {
		return fs.ErrPermission
	}
	return s.Memory.Commit(ctx, generation, expected)
}

func TestRejectedProviderPublicationDoesNotRetainInputs(t *testing.T) {
	layer := scopedProviderLayer(t, "binding", "1", time.Date(2026, 9, 7, 0, 0, 0, 0, time.UTC))
	store := &retentionRejectingStore{Memory: storage.NewMemory()}
	options := []Option{WithStateDirectory(privateRuntimeDirectory(t)), WithCatalogSource("embedded"), WithProviderBindings(*layer.Receipt.ProviderBinding), WithClientOptions(starmap.WithCatalogStore(store))}
	connected := openTestRuntime(t, options...)
	before := connected.State().GenerationID
	store.reject.Store(true)
	if _, err := connected.publishProviders(t.Context(), []ProviderLayer{layer}, connected.lease.epoch()); err == nil {
		t.Fatal("rejected publication succeeded")
	}
	if connected.State().GenerationID != before {
		t.Error("failed publication changed active generation")
	}
	retained, err := connected.store.loadProviders()
	if err != nil {
		t.Fatal(err)
	}
	if len(retained) != 0 {
		t.Error("rejected publication retained provider inputs")
	}
	if err := connected.Close(); err != nil {
		t.Fatal(err)
	}
	store.reject.Store(false)
	restarted := openTestRuntime(t, options...)
	if restarted.State().GenerationID != before {
		t.Error("restart activated inputs from the rejected publication")
	}
}

func TestRejectedSourcePublicationDoesNotRetainInputs(t *testing.T) {
	store := &retentionRejectingStore{Memory: storage.NewMemory()}
	source := newStubSource("rejected-source")
	source.replies = []SourceRead{testSourceRead(t, "source-generation", testCatalogPayload(t, "new-provider", "new-model", "New Model"), time.Now().UTC())}
	options := []Option{WithStateDirectory(privateRuntimeDirectory(t)), WithSource(source), WithClientOptions(starmap.WithCatalogStore(store))}
	connected := openTestRuntime(t, options...)
	before := connected.State().GenerationID
	store.reject.Store(true)
	if _, err := connected.RefreshSource(t.Context()); err == nil {
		t.Fatal("rejected source publication succeeded")
	}
	retained, err := connected.store.loadSource()
	if err != nil {
		t.Fatal(err)
	}
	if retained != nil {
		t.Fatal("rejected source publication retained new inputs")
	}
	if err := connected.Close(); err != nil {
		t.Fatal(err)
	}
	store.reject.Store(false)
	restarted := openTestRuntime(t, options...)
	if restarted.State().GenerationID != before {
		t.Fatal("restart activated the rejected source")
	}
}
