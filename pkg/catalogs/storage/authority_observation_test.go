package storage

import (
	"context"
	"testing"

	"github.com/agentstation/starmap/pkg/catalogs"
)

type authorityObservationReader interface {
	CurrentAuthorityHead(context.Context) (catalogs.CatalogAuthorityHead, error)
}

func requireAuthorityObservationReader(t *testing.T, store Store) authorityObservationReader {
	t.Helper()
	reader, ok := store.(authorityObservationReader)
	if !ok {
		t.Fatal("store cannot observe current authority independently of the catalog payload")
	}
	return reader
}

func TestCatalogStoreCurrentAuthorityObservation(t *testing.T) {
	for name, factory := range catalogStoreFactories() {
		t.Run(name, func(t *testing.T) {
			store := factory(t)
			first := authorityStoredGeneration("observation-first", 1)
			if err := store.Commit(t.Context(), first, ""); err != nil {
				t.Fatal(err)
			}
			reader := requireAuthorityObservationReader(t, store)
			got, err := reader.CurrentAuthorityHead(t.Context())
			if err != nil || got != first.Manifest.AuthorityHead {
				t.Fatalf("head=%+v error=%v, want first publication", got, err)
			}
			second := authorityStoredGeneration("observation-withdrawal", 2)
			if err := store.Commit(t.Context(), second, first.Manifest.GenerationID); err != nil {
				t.Fatal(err)
			}
			got, err = reader.CurrentAuthorityHead(t.Context())
			if err != nil || got != second.Manifest.AuthorityHead {
				t.Fatalf("head=%+v error=%v, want withdrawal publication", got, err)
			}
			ordinary := testGeneration("observation-ordinary", "ordinary")
			if err := store.Commit(t.Context(), ordinary, second.Manifest.GenerationID); err != nil {
				t.Fatal(err)
			}
			if got, err := reader.CurrentAuthorityHead(t.Context()); err == nil || got != (catalogs.CatalogAuthorityHead{}) {
				t.Fatalf("ordinary publication returned authority head=%+v error=%v", got, err)
			}
		})
	}
}

func TestFilesystemAuthorityObservationSeesAnotherWriter(t *testing.T) {
	root := privateFilesystemRoot(t)
	writer, err := NewFilesystem(root)
	if err != nil {
		t.Fatal(err)
	}
	observer, err := NewFilesystem(root)
	if err != nil {
		t.Fatal(err)
	}
	first := authorityStoredGeneration("writer-first", 1)
	if err := writer.Commit(t.Context(), first, ""); err != nil {
		t.Fatal(err)
	}
	reader := requireAuthorityObservationReader(t, observer)
	if _, err := reader.CurrentAuthorityHead(t.Context()); err != nil {
		t.Fatal(err)
	}
	second := authorityStoredGeneration("writer-withdrawal", 2)
	if err := writer.Commit(t.Context(), second, first.Manifest.GenerationID); err != nil {
		t.Fatal(err)
	}
	got, err := reader.CurrentAuthorityHead(t.Context())
	if err != nil || got != second.Manifest.AuthorityHead {
		t.Fatalf("head=%+v error=%v, want another writer's withdrawal", got, err)
	}
}

type ordinaryObjectReads struct{ backend *MemoryObjectBackend }

func (b ordinaryObjectReads) Get(ctx context.Context, key string) (ObjectValue, error) {
	return b.backend.Get(ctx, key)
}

func (b ordinaryObjectReads) Put(ctx context.Context, key string, data []byte, condition ObjectPutCondition) (ObjectValue, error) {
	return b.backend.Put(ctx, key, data, condition)
}

func TestObjectAuthorityObservationRequiresCurrentReadCapability(t *testing.T) {
	backend := ordinaryObjectReads{backend: NewMemoryObjectBackend()}
	store, err := NewObject(backend, "ordinary-read-capability")
	if err != nil {
		t.Fatal(err)
	}
	generation := authorityStoredGeneration("current-read-required", 1)
	if err := store.Commit(t.Context(), generation, ""); err != nil {
		t.Fatal(err)
	}
	assertStoredGeneration(t, store, generation)
	reader := requireAuthorityObservationReader(t, store)
	if got, err := reader.CurrentAuthorityHead(t.Context()); err == nil || got != (catalogs.CatalogAuthorityHead{}) {
		t.Fatalf("unqualified backend returned head=%+v error=%v", got, err)
	}
}

func TestAuthorityObservationRejectsCanceledReads(t *testing.T) {
	for name, factory := range catalogStoreFactories() {
		t.Run(name, func(t *testing.T) {
			store := factory(t)
			generation := authorityStoredGeneration("canceled-observation", 1)
			if err := store.Commit(t.Context(), generation, ""); err != nil {
				t.Fatal(err)
			}
			ctx, cancel := context.WithCancel(t.Context())
			cancel()
			reader := requireAuthorityObservationReader(t, store)
			if got, err := reader.CurrentAuthorityHead(ctx); err == nil || got != (catalogs.CatalogAuthorityHead{}) {
				t.Fatalf("canceled read returned head=%+v error=%v", got, err)
			}
			got, err := reader.CurrentAuthorityHead(t.Context())
			if err != nil || got != generation.Manifest.AuthorityHead {
				t.Fatalf("head=%+v error=%v", got, err)
			}
		})
	}
}

func TestObjectAuthorityObservationSeesAnotherWriter(t *testing.T) {
	backend := NewMemoryObjectBackend()
	writer, err := NewObject(backend, "shared-observation")
	if err != nil {
		t.Fatal(err)
	}
	observer, err := NewObject(backend, "shared-observation")
	if err != nil {
		t.Fatal(err)
	}
	first := authorityStoredGeneration("object-first", 1)
	if err := writer.Commit(t.Context(), first, ""); err != nil {
		t.Fatal(err)
	}
	if _, err := observer.CurrentAuthorityHead(t.Context()); err != nil {
		t.Fatal(err)
	}
	second := authorityStoredGeneration("object-withdrawal", 2)
	if err := writer.Commit(t.Context(), second, first.Manifest.GenerationID); err != nil {
		t.Fatal(err)
	}
	got, err := observer.CurrentAuthorityHead(t.Context())
	if err != nil || got != second.Manifest.AuthorityHead {
		t.Fatalf("head=%+v error=%v", got, err)
	}
}
