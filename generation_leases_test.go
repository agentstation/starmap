package starmap

import (
	"bytes"
	"context"
	stderrors "errors"
	"path/filepath"
	"testing"

	"github.com/agentstation/starmap/pkg/catalogs"
	"github.com/agentstation/starmap/pkg/catalogs/storage"
	"github.com/agentstation/starmap/pkg/errors"
)

func TestClientGenerationLeaseProtectsStoredBytes(t *testing.T) {
	for _, name := range []string{"memory", "filesystem"} {
		t.Run(name, func(t *testing.T) {
			var store storage.RetainingStore = storage.NewMemory()
			if name == "filesystem" {
				filesystem, err := storage.NewFilesystem(filepath.Join(t.TempDir(), "catalog"))
				if err != nil {
					t.Fatal(err)
				}
				store = filesystem
			}
			testClientGenerationLeaseProtectsStoredBytes(t, store)
		})
	}
}

func testClientGenerationLeaseProtectsStoredBytes(t *testing.T, store storage.RetainingStore) {
	t.Helper()
	selected := rootRemoteGeneration(t)
	if err := store.Commit(t.Context(), selected, ""); err != nil {
		t.Fatal(err)
	}
	client, err := NewContext(t.Context(), WithCatalogStore(store))
	if err != nil {
		t.Fatal(err)
	}
	if !client.CanLeaseGenerations() {
		t.Fatal("memory lease capability is absent")
	}
	ctx, cancel := context.WithCancel(t.Context())
	lease, release, err := client.AcquireGeneration(ctx, selected.Manifest.GenerationID)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = release() })
	lease.Payload[0] ^= 1
	cancel()
	current := selected.Copy()
	current.Manifest.GenerationID += "-current"
	if err := store.Commit(t.Context(), current, selected.Manifest.GenerationID); err != nil {
		t.Fatal(err)
	}
	request := storage.RetentionRequest{ExpectedGenerationID: current.Manifest.GenerationID, MaxGenerations: 1, MaxBytes: 1}
	if _, err := store.Collect(t.Context(), request); err != nil {
		t.Fatal(err)
	}
	stored, err := store.Get(t.Context(), selected.Manifest.GenerationID)
	if err != nil || !bytes.Equal(stored.Payload, selected.Payload) {
		t.Fatalf("leased content changed or disappeared: %v", err)
	}
	for range 2 {
		if err := release(); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := store.Collect(t.Context(), request); err != nil {
		t.Fatal(err)
	}
	if _, err := store.Get(t.Context(), selected.Manifest.GenerationID); !errors.IsNotFound(err) {
		t.Fatalf("released generation remains: %v", err)
	}
}

type generationLeaseHiddenStore struct{ storage.Store }

func TestClientGenerationLeaseRequiresCapability(t *testing.T) {
	for name, client := range map[string]*Client{"nil": nil, "unconstructed": {}} {
		t.Run(name, func(t *testing.T) {
			if client.CanLeaseGenerations() {
				t.Fatal("unconstructed client reports leases")
			}
			if _, release, err := client.AcquireGeneration(t.Context(), "missing"); err == nil || release != nil {
				t.Fatalf("invalid acquisition = %v, release present = %v", err, release != nil)
			}
		})
	}
	store := storage.NewMemory()
	client, err := NewContext(t.Context(), WithCatalogStore(generationLeaseHiddenStore{Store: store}))
	if err != nil {
		t.Fatal(err)
	}
	if client.CanLeaseGenerations() {
		t.Fatal("minimum store reports an unavailable capability")
	}
	_, release, err := client.AcquireGeneration(t.Context(), "missing")
	var config *errors.ConfigError
	if !stderrors.As(err, &config) || release != nil {
		t.Fatalf("unsupported acquisition = %v, release present = %v", err, release != nil)
	}
}

func TestClientGenerationLeaseEmbeddedAndReadErrors(t *testing.T) {
	store := storage.NewMemory()
	client, err := NewContext(t.Context(), WithCatalogStore(store))
	if err != nil {
		t.Fatal(err)
	}
	generation, release, err := client.AcquireGeneration(nil, client.CurrentGenerationID())
	if err != nil || generation.Manifest.GenerationID != client.CurrentGenerationID() || release == nil {
		t.Fatalf("embedded acquisition = %v", err)
	}
	if err := release(); err != nil {
		t.Fatal(err)
	}
	if _, err := store.Current(t.Context()); !errors.IsNotFound(err) {
		t.Fatalf("embedded read wrote to storage: %v", err)
	}
	if _, release, err := client.AcquireGeneration(t.Context(), "missing"); !errors.IsNotFound(err) || release != nil {
		t.Fatalf("missing acquisition = %v, release present = %v", err, release != nil)
	}
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	if _, release, err := client.AcquireGeneration(ctx, client.CurrentGenerationID()); !stderrors.Is(err, context.Canceled) || release != nil {
		t.Fatalf("canceled acquisition = %v, release present = %v", err, release != nil)
	}
}

type generationLeaseCheckedStore struct {
	*storage.Memory
	read         func(context.Context, string) (catalogs.Generation, error)
	releaseError error
	released     bool
}

func (s *generationLeaseCheckedStore) Get(ctx context.Context, id string) (catalogs.Generation, error) {
	return s.read(ctx, id)
}

func (s *generationLeaseCheckedStore) AcquireGeneration(ctx context.Context, id string) (catalogs.Generation, func() error, error) {
	generation, release, err := s.Memory.AcquireGeneration(ctx, id)
	if err != nil {
		return generation, release, err
	}
	return generation, func() error {
		s.released = true
		return stderrors.Join(release(), s.releaseError)
	}, nil
}

func TestClientGenerationLeasePreservesConfiguredReads(t *testing.T) {
	readFailure := stderrors.New("configured read refused")
	releaseFailure := stderrors.New("release failed")
	for _, name := range []string{"read error", "identity", "payload", "manifest", "cancellation"} {
		t.Run(name, func(t *testing.T) {
			selected := rootRemoteGeneration(t)
			store := &generationLeaseCheckedStore{Memory: storage.NewMemory(), releaseError: releaseFailure}
			if err := store.Commit(t.Context(), selected, ""); err != nil {
				t.Fatal(err)
			}
			client, err := NewContext(t.Context(), WithCatalogStore(store))
			if err != nil {
				t.Fatal(err)
			}
			ctx, cancel := context.WithCancel(t.Context())
			defer cancel()
			store.read = func(ctx context.Context, id string) (catalogs.Generation, error) {
				generation := selected.Copy()
				switch name {
				case "read error":
					return catalogs.Generation{}, readFailure
				case "identity":
					generation.Manifest.GenerationID += "-wrong"
				case "payload":
					generation.Payload[0] ^= 1
				case "manifest":
					generation.Manifest.SyncRunID += "-different"
				case "cancellation":
					cancel()
					return catalogs.Generation{}, ctx.Err()
				}
				return generation, nil
			}
			_, release, err := client.AcquireGeneration(ctx, selected.Manifest.GenerationID)
			if err == nil || release != nil || !store.released || !stderrors.Is(err, releaseFailure) {
				t.Fatalf("failed read did not release: error %v, release present %v, released %v", err, release != nil, store.released)
			}
			if name == "read error" && !stderrors.Is(err, readFailure) {
				t.Fatalf("read failure lost: %v", err)
			}
			if name == "cancellation" && !stderrors.Is(err, context.Canceled) {
				t.Fatalf("cancellation lost: %v", err)
			}
			current := selected.Copy()
			current.Manifest.GenerationID += "-current"
			if err := store.Commit(t.Context(), current, selected.Manifest.GenerationID); err != nil {
				t.Fatal(err)
			}
			if _, err := store.Collect(t.Context(), storage.RetentionRequest{
				ExpectedGenerationID: current.Manifest.GenerationID, MaxGenerations: 1, MaxBytes: 1,
			}); err != nil {
				t.Fatal(err)
			}
			if _, err := store.Memory.Get(t.Context(), selected.Manifest.GenerationID); !errors.IsNotFound(err) {
				t.Fatalf("failed acquisition leaked its lease: %v", err)
			}
		})
	}
}

func TestClientGenerationLeaseEmbeddedPreservesConfiguredReads(t *testing.T) {
	for _, name := range []string{"refused", "different manifest"} {
		t.Run(name, func(t *testing.T) {
			store := &generationLeaseCheckedStore{Memory: storage.NewMemory()}
			client, err := NewContext(t.Context(), WithCatalogStore(store))
			if err != nil {
				t.Fatal(err)
			}
			compiled, err := client.CurrentGeneration(t.Context())
			if err != nil {
				t.Fatal(err)
			}
			refused := stderrors.New("configured embedded read refused")
			store.read = func(context.Context, string) (catalogs.Generation, error) {
				if name == "refused" {
					return catalogs.Generation{}, refused
				}
				compiled.Manifest.SyncRunID += "-different"
				return compiled, nil
			}
			_, release, err := client.AcquireGeneration(t.Context(), compiled.Manifest.GenerationID)
			if err == nil || release != nil {
				t.Fatalf("embedded fallback bypassed configured read: %v", err)
			}
			if name == "refused" && !stderrors.Is(err, refused) {
				t.Fatalf("configured error lost: %v", err)
			}
		})
	}
}
