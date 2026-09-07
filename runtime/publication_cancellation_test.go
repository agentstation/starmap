package runtime

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/agentstation/starmap"
	"github.com/agentstation/starmap/pkg/catalogs"
	"github.com/agentstation/starmap/pkg/catalogs/storage"
)

type retentionCancelStore struct {
	*storage.Memory
	cancel context.CancelFunc
	before bool
}

func (s *retentionCancelStore) Commit(ctx context.Context, generation catalogs.Generation, expected string) error {
	if s.cancel != nil && s.before {
		s.cancel()
	}
	err := s.Memory.Commit(ctx, generation, expected)
	if s.cancel != nil && !s.before {
		s.cancel()
	}
	return err
}

func TestRetentionCancellationRespectsCatalogCommitBoundary(t *testing.T) {
	for _, before := range []bool{true, false} {
		name := "after"
		if before {
			name = "before"
		}
		t.Run(name, func(t *testing.T) {
			layer := scopedProviderLayer(t, "binding", "1", time.Date(2026, 9, 7, 0, 0, 0, 0, time.UTC))
			store := &retentionCancelStore{Memory: storage.NewMemory(), before: before}
			options := []Option{WithStateDirectory(privateRuntimeDirectory(t)), WithCatalogSource("embedded"), WithProviderBindings(*layer.Receipt.ProviderBinding), WithClientOptions(starmap.WithCatalogStore(store))}
			connected := openTestRuntime(t, options...)
			original := connected.State().GenerationID
			ctx, cancel := context.WithCancel(t.Context())
			defer cancel()
			store.cancel = cancel
			state, err := connected.publishProviders(ctx, []ProviderLayer{layer}, connected.lease.epoch())
			if before {
				if !errors.Is(err, context.Canceled) {
					t.Fatalf("pre-commit cancellation = %v", err)
				}
			} else {
				if err != nil {
					t.Fatal(err)
				}
				if state.GenerationID == original {
					t.Fatal("accepted commit lost its generation")
				}
				if pending, err := connected.store.loadInputPublication(); err != nil || pending != nil {
					t.Fatalf("accepted cancellation left recovery pending: %v", err)
				}
			}
			store.cancel = nil
			if err := connected.Close(); err != nil {
				t.Fatal(err)
			}
			recovered := openTestRuntime(t, options...)
			retained, err := recovered.store.loadProviders()
			if err != nil {
				t.Fatal(err)
			}
			if before && (len(retained) != 0 || recovered.State().GenerationID != original) {
				t.Fatal("canceled publication became active after restart")
			}
			if !before && (len(retained) != 1 || recovered.State().GenerationID != state.GenerationID) {
				t.Fatal("accepted publication did not survive cancellation and restart")
			}
		})
	}
}
