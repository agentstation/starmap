package permission

import (
	"context"
	stderrors "errors"
	"testing"

	"github.com/agentstation/starmap/pkg/catalogs/storage"
)

func TestPublisherGenerationLeaseDoesNotExposePublication(t *testing.T) {
	store := storage.NewMemory()
	selected := issuerGeneration(t, "leased-authority", 1)
	if err := store.Commit(t.Context(), selected, ""); err != nil {
		t.Fatal(err)
	}
	publisher := newTestPublisher(t, store)
	leaser, ok := storage.GenerationLeaserFor(publisher)
	if !ok || leaser == nil {
		t.Fatal("publisher hid its read lease capability")
	}
	if _, writable := leaser.(storage.Store); writable {
		t.Fatal("read lease exposes an unguarded publication store")
	}
	generation, release, err := leaser.AcquireGeneration(t.Context(), selected.Manifest.GenerationID)
	if err != nil || generation.Manifest.GenerationID != selected.Manifest.GenerationID {
		t.Fatalf("forwarded acquisition = %v", err)
	}
	t.Cleanup(func() { _ = release() })
	current := issuerGeneration(t, "current-authority", 2)
	if err := publisher.Commit(t.Context(), current, selected.Manifest.GenerationID); err != nil {
		t.Fatal(err)
	}
	report, err := store.Collect(t.Context(), storage.RetentionRequest{ExpectedGenerationID: current.Manifest.GenerationID, MaxGenerations: 1, MaxBytes: 1})
	if err != nil || report.Protected.Generations != 2 {
		t.Fatalf("forwarded lease lost protection: %+v, %v", report, err)
	}
	if err := release(); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	if _, _, err := leaser.AcquireGeneration(ctx, selected.Manifest.GenerationID); !stderrors.Is(err, context.Canceled) {
		t.Fatalf("canceled forwarded read = %v", err)
	}
}

type generationLeaseHiddenStore struct{ storage.Store }

func TestPublisherGenerationLeasePreservesUnsupportedStores(t *testing.T) {
	for name, publisher := range map[string]*Publisher{
		"nil":           nil,
		"unconstructed": {},
		"minimum store": newTestPublisher(t, generationLeaseHiddenStore{Store: storage.NewMemory()}),
	} {
		t.Run(name, func(t *testing.T) {
			if leaser, ok := storage.GenerationLeaserFor(publisher); ok || leaser != nil {
				t.Fatal("publisher invented a read lease capability")
			}
		})
	}
}
