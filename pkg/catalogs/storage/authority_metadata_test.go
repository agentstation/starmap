package storage

import (
	"bytes"
	"context"
	stderrors "errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/agentstation/starmap/internal/privatefiles"
	"github.com/agentstation/starmap/pkg/catalogs"
)

type authorityMetadataFixture struct {
	store   Store
	remove  func(string)
	replace func(string, []byte)
	read    func(string) []byte
}

func newAuthorityMetadataFixture(t *testing.T, backend string, generation catalogs.Generation) authorityMetadataFixture {
	t.Helper()
	var fixture authorityMetadataFixture
	if backend == "filesystem" {
		store, err := NewFilesystem(privateFilesystemRoot(t))
		if err != nil {
			t.Fatal(err)
		}
		fixture.store = store
		fixture.remove = func(name string) {
			t.Helper()
			if err := os.Remove(filepath.Join(store.generationDir(generation.Manifest.GenerationID), name)); err != nil {
				t.Fatal(err)
			}
		}
		fixture.replace = func(name string, data []byte) {
			t.Helper()
			directory, err := privatefiles.ExistingDirectory(store.generationDir(generation.Manifest.GenerationID))
			if err != nil {
				t.Fatal(err)
			}
			if err := directory.WriteFile(name, data, ".test-"); err != nil {
				t.Fatal(err)
			}
		}
		fixture.read = func(name string) []byte {
			t.Helper()
			data, err := readPrivateStoreFile(filepath.Join(store.generationDir(generation.Manifest.GenerationID), name))
			if err != nil {
				t.Fatal(err)
			}
			return data
		}
	} else {
		objects := NewMemoryObjectBackend()
		store, err := NewObject(objects, "authority-metadata")
		if err != nil {
			t.Fatal(err)
		}
		fixture.store = store
		key := func(name string) string { return store.generationKey(generation.Manifest.GenerationID, name) }
		fixture.remove = func(name string) { objects.mu.Lock(); defer objects.mu.Unlock(); delete(objects.objects, key(name)) }
		fixture.replace = func(name string, data []byte) {
			objects.mu.Lock()
			defer objects.mu.Unlock()
			value := objects.objects[key(name)]
			value.Data = bytes.Clone(data)
			objects.objects[key(name)] = value
		}
		fixture.read = func(name string) []byte {
			t.Helper()
			value, err := objects.Get(t.Context(), key(name))
			if err != nil {
				t.Fatal(err)
			}
			return value.Data
		}
	}
	if err := fixture.store.Commit(t.Context(), generation, ""); err != nil {
		t.Fatal(err)
	}
	return fixture
}

func TestAuthorityObservationIndependentOfManifestAndPayload(t *testing.T) {
	for _, backend := range []string{"filesystem", "object"} {
		t.Run(backend, func(t *testing.T) {
			generation := authorityStoredGeneration("independent-head", 1)
			fixture := newAuthorityMetadataFixture(t, backend, generation)
			fixture.remove(payloadFilename)
			fixture.replace(manifestFilename, []byte("{\"manifest_version\":999}"))
			if _, err := fixture.store.Current(t.Context()); err == nil {
				t.Fatal("fixture catalog remains readable")
			}
			reader := requireAuthorityObservationReader(t, fixture.store)
			got, err := reader.CurrentAuthorityHead(t.Context())
			if err != nil || got != generation.Manifest.AuthorityHead {
				t.Fatalf("head=%+v error=%v", got, err)
			}
		})
	}
}

func TestAuthorityMetadataRepairRequiresExplicitCommit(t *testing.T) {
	for _, backend := range []string{"filesystem", "object"} {
		t.Run(backend, func(t *testing.T) {
			generation := authorityStoredGeneration("legacy-authority", 1)
			fixture := newAuthorityMetadataFixture(t, backend, generation)
			fixture.remove(authorityFilename)
			reader := requireAuthorityObservationReader(t, fixture.store)
			for range 2 {
				if got, err := reader.CurrentAuthorityHead(t.Context()); err == nil || got != (catalogs.CatalogAuthorityHead{}) {
					t.Fatalf("missing metadata head=%+v error=%v", got, err)
				}
			}
			assertStoredGeneration(t, fixture.store, generation)
			if err := fixture.store.Commit(t.Context(), generation, ""); err != nil {
				t.Fatal(err)
			}
			got, err := reader.CurrentAuthorityHead(t.Context())
			if err != nil || got != generation.Manifest.AuthorityHead {
				t.Fatalf("repaired head=%+v error=%v", got, err)
			}
			if err := fixture.store.Commit(t.Context(), generation, ""); err != nil {
				t.Fatal(err)
			}
			assertStoredGeneration(t, fixture.store, generation)
		})
	}
}

func TestAuthorityMetadataConflictPreservesRecordAndCurrent(t *testing.T) {
	for _, backend := range []string{"filesystem", "object"} {
		t.Run(backend, func(t *testing.T) {
			generation := authorityStoredGeneration("conflicting-authority", 1)
			fixture := newAuthorityMetadataFixture(t, backend, generation)
			other := generation.Copy()
			other.Manifest.AuthorityHead.RequiredPermissionRevision = "sha256:" + strings.Repeat("c", 64)
			conflict, err := authorityRecordData(other)
			if err != nil {
				t.Fatal(err)
			}
			fixture.replace(authorityFilename, conflict)
			if err := fixture.store.Commit(t.Context(), generation, ""); err == nil {
				t.Fatal("replaced conflicting authority metadata")
			}
			if got := fixture.read(authorityFilename); !bytes.Equal(got, conflict) {
				t.Fatal("conflicting record changed")
			}
			assertStoredGeneration(t, fixture.store, generation)
		})
	}
}

func TestAuthorityObservationRejectsInvalidMetadata(t *testing.T) {
	for _, backend := range []string{"filesystem", "object"} {
		for _, invalid := range []string{"mismatched", "oversized", "malformed"} {
			t.Run(backend+"/"+invalid, func(t *testing.T) {
				generation := authorityStoredGeneration("validated-head", 1)
				fixture := newAuthorityMetadataFixture(t, backend, generation)
				data := []byte("{\"version\":1}")
				if invalid == "oversized" {
					data = bytes.Repeat([]byte(" "), catalogs.MaxCatalogAuthorityRecordBytes+1)
				}
				if invalid == "mismatched" {
					other := authorityStoredGeneration("another-head", 1)
					var err error
					data, err = authorityRecordData(other)
					if err != nil {
						t.Fatal(err)
					}
				}
				fixture.replace(authorityFilename, data)
				reader := requireAuthorityObservationReader(t, fixture.store)
				if got, err := reader.CurrentAuthorityHead(t.Context()); err == nil || got != (catalogs.CatalogAuthorityHead{}) {
					t.Fatalf("invalid metadata head=%+v error=%v", got, err)
				}
			})
		}
	}
}

type authorityWriteFaultBackend struct {
	*MemoryObjectBackend
	fault error
}

func (b *authorityWriteFaultBackend) Put(ctx context.Context, key string, data []byte, condition ObjectPutCondition) (ObjectValue, error) {
	if b.fault != nil && strings.HasSuffix(key, "/"+authorityFilename) {
		return ObjectValue{}, b.fault
	}
	return b.MemoryObjectBackend.Put(ctx, key, data, condition)
}

func TestAuthorityMetadataWriteFailurePreservesSelectedHead(t *testing.T) {
	backend := &authorityWriteFaultBackend{MemoryObjectBackend: NewMemoryObjectBackend()}
	store, err := NewObject(backend, "faulted-authority")
	if err != nil {
		t.Fatal(err)
	}
	first := authorityStoredGeneration("before-metadata-fault", 1)
	if err := store.Commit(t.Context(), first, ""); err != nil {
		t.Fatal(err)
	}
	backend.fault = stderrors.New("authority metadata write failure")
	second := authorityStoredGeneration("after-metadata-fault", 2)
	if err := store.Commit(t.Context(), second, first.Manifest.GenerationID); !stderrors.Is(err, backend.fault) {
		t.Fatalf("commit error=%v", err)
	}
	got, err := store.CurrentAuthorityHead(t.Context())
	if err != nil || got != first.Manifest.AuthorityHead {
		t.Fatalf("head=%+v error=%v", got, err)
	}
	assertStoredGeneration(t, store, first)
	backend.fault = nil
	if err := store.Commit(t.Context(), second, first.Manifest.GenerationID); err != nil {
		t.Fatal(err)
	}
	got, err = store.CurrentAuthorityHead(t.Context())
	if err != nil || got != second.Manifest.AuthorityHead {
		t.Fatalf("retried head=%+v error=%v", got, err)
	}
}

func TestObjectAuthorityObservationRejectsAmbiguousPointers(t *testing.T) {
	for name, data := range map[string][]byte{
		"empty":      nil,
		"null":       []byte("null"),
		"duplicate":  []byte("{\"generation_id\":\"pointer-head\",\"generation_id\":\"pointer-head\"}"),
		"unknown":    []byte("{\"generation_id\":\"pointer-head\",\"version\":2}"),
		"wrong case": []byte("{\"Generation_ID\":\"pointer-head\"}"),
		"trailing":   []byte("{\"generation_id\":\"pointer-head\"} {}"),
		"oversized":  bytes.Repeat([]byte(" "), catalogs.MaxCatalogAuthorityRecordBytes+1),
	} {
		t.Run(name, func(t *testing.T) {
			backend := NewMemoryObjectBackend()
			store, err := NewObject(backend, "pointer-validation")
			if err != nil {
				t.Fatal(err)
			}
			generation := authorityStoredGeneration("pointer-head", 1)
			if err := store.Commit(t.Context(), generation, ""); err != nil {
				t.Fatal(err)
			}
			current, err := backend.Get(t.Context(), store.currentKey())
			if err != nil {
				t.Fatal(err)
			}
			if _, err := backend.Put(t.Context(), store.currentKey(), data, ObjectPutCondition{IfVersion: current.Version}); err != nil {
				t.Fatal(err)
			}
			if got, err := store.CurrentAuthorityHead(t.Context()); err == nil || got != (catalogs.CatalogAuthorityHead{}) {
				t.Fatalf("ambiguous pointer returned head=%+v error=%v", got, err)
			}
		})
	}
}
