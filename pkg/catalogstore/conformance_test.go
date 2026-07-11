package catalogstore

import (
	"bytes"
	"context"
	stderrors "errors"
	"fmt"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"

	"github.com/agentstation/starmap/pkg/catalogmeta"
	"github.com/agentstation/starmap/pkg/catalogs"
	pkgerrors "github.com/agentstation/starmap/pkg/errors"
)

type storeFactory func(*testing.T) Store

func TestCatalogStoreConformance(t *testing.T) {
	for name, factory := range catalogStoreFactories() {
		t.Run(name, func(t *testing.T) {
			runCatalogStoreConformance(t, factory(t))
		})
	}
}

func catalogStoreFactories() map[string]storeFactory {
	return map[string]storeFactory{
		"memory": func(*testing.T) Store { return NewMemory() },
		"filesystem": func(t *testing.T) Store {
			store, err := NewFilesystem(t.TempDir())
			if err != nil {
				t.Fatalf("NewFilesystem: %v", err)
			}
			return store
		},
		"sql": newSQLConformanceStore,
		"object": func(t *testing.T) Store {
			store, err := NewObject(NewMemoryObjectBackend(), "conformance")
			if err != nil {
				t.Fatalf("NewObject: %v", err)
			}
			return store
		},
	}
}

func TestCatalogStoreConformanceStaleRecordsDoNotReappear(t *testing.T) {
	for name, factory := range catalogStoreFactories() {
		t.Run(name, func(t *testing.T) {
			store := factory(t)
			first := testGeneration("stale-first", "stale-record,current-record")
			if err := store.Commit(context.Background(), first, ""); err != nil {
				t.Fatalf("Commit first: %v", err)
			}
			replacement := testGeneration("stale-replacement", "current-record")
			if err := store.Commit(context.Background(), replacement, "stale-first"); err != nil {
				t.Fatalf("Commit replacement: %v", err)
			}
			current, err := store.Current(context.Background())
			if err != nil {
				t.Fatalf("Current: %v", err)
			}
			if bytes.Contains(current.Payload, []byte("stale-record")) {
				t.Fatalf("stale record reappeared in %q", current.Payload)
			}
			if !bytes.Contains(current.Payload, []byte("current-record")) {
				t.Fatalf("current record missing from %q", current.Payload)
			}
		})
	}
}

func runCatalogStoreConformance(t *testing.T, store Store) {
	t.Helper()
	ctx := context.Background()

	if _, err := store.Current(ctx); !stderrors.Is(err, pkgerrors.ErrNotFound) {
		t.Fatalf("empty Current error = %v, want ErrNotFound", err)
	}

	first := testGeneration("generation-1", "first")
	if err := store.Commit(ctx, first, ""); err != nil {
		t.Fatalf("Commit first: %v", err)
	}
	assertStoredGeneration(t, store, first)

	// A retry after an ambiguous result is idempotent even though the original
	// base generation no longer matches.
	if err := store.Commit(ctx, first, ""); err != nil {
		t.Fatalf("idempotent retry: %v", err)
	}

	// Inputs and outputs are caller-owned.
	first.Payload[0] ^= 0xff
	first.Manifest.SourceObservations[0].ObservationID = "mutated-input"
	stored, err := store.Get(ctx, "generation-1")
	if err != nil {
		t.Fatalf("Get first: %v", err)
	}
	stored.Payload[0] ^= 0xff
	stored.Manifest.Validation.Checks[0].Name = "mutated-output"
	pristine := testGeneration("generation-1", "first")
	assertStoredGeneration(t, store, pristine)

	second := testGeneration("generation-2", "second")
	if err := store.Commit(ctx, second, "generation-1"); err != nil {
		t.Fatalf("Commit second: %v", err)
	}
	assertStoredGeneration(t, store, second)
	gotFirst, err := store.Get(ctx, "generation-1")
	if err != nil {
		t.Fatalf("Get retained first generation: %v", err)
	}
	if diff := cmp.Diff(pristine, gotFirst); diff != "" {
		t.Fatalf("retained generation mismatch (-want +got):\n%s", diff)
	}

	third := testGeneration("generation-3", "third")
	err = store.Commit(ctx, third, "generation-1")
	var conflict *pkgerrors.ConflictError
	if !stderrors.As(err, &conflict) {
		t.Fatalf("stale Commit error = %v, want *errors.ConflictError", err)
	}
	if conflict.Expected != "generation-1" || conflict.Actual != "generation-2" {
		t.Fatalf("conflict = %#v", conflict)
	}
	assertStoredGeneration(t, store, second)
	if _, err := store.Get(ctx, "generation-3"); !stderrors.Is(err, pkgerrors.ErrNotFound) {
		t.Fatalf("stale generation persisted, error = %v", err)
	}

	corrupt := testGeneration("generation-corrupt", "corrupt")
	corrupt.Payload = append(corrupt.Payload, 'x')
	if err := store.Commit(ctx, corrupt, "generation-2"); !stderrors.Is(err, pkgerrors.ErrInvalidInput) {
		t.Fatalf("corrupt Commit error = %v, want ErrInvalidInput", err)
	}
	assertStoredGeneration(t, store, second)

	conflictingID := testGeneration("generation-2", "different bytes")
	if err := store.Commit(ctx, conflictingID, "generation-2"); !stderrors.Is(err, pkgerrors.ErrConflict) {
		t.Fatalf("generation ID collision error = %v, want ErrConflict", err)
	}
	assertStoredGeneration(t, store, second)

	canceled, cancel := context.WithCancel(ctx)
	cancel()
	if err := store.Commit(canceled, third, "generation-2"); !stderrors.Is(err, context.Canceled) {
		t.Fatalf("canceled Commit error = %v, want context.Canceled", err)
	}
	assertStoredGeneration(t, store, second)
}

func assertStoredGeneration(t *testing.T, store Store, want Generation) {
	t.Helper()
	got, err := store.Current(context.Background())
	if err != nil {
		t.Fatalf("Current: %v", err)
	}
	if diff := cmp.Diff(want, got); diff != "" {
		t.Fatalf("Current mismatch (-want +got):\n%s", diff)
	}
}

func testGeneration(id, value string) Generation {
	payload := []byte(fmt.Sprintf(`{"value":%q}`, value))
	evidence := catalogs.DescribeCatalogPayload([]byte("evidence:" + value))
	generatedAt := time.Date(2026, time.July, 9, 18, 0, 0, 0, time.UTC)
	return Generation{
		Manifest: catalogs.GenerationManifest{
			ManifestVersion: catalogs.CurrentGenerationManifestVersion,
			SchemaVersion:   catalogs.CurrentCatalogSchemaVersion,
			GenerationID:    id,
			GeneratedAt:     generatedAt,
			Payload:         catalogs.DescribeCatalogPayload(payload),
			Validation: catalogs.GenerationValidationReport{
				ValidatorVersion: "catalog-validator/v1",
				ValidatedAt:      generatedAt.Add(time.Second),
				Status:           catalogs.GenerationValidationPassed,
				Checks: []catalogs.GenerationValidationCheck{
					{Name: "schema", Status: catalogs.GenerationValidationCheckPassed},
				},
			},
			SyncRunID: "sync-" + id,
			SourceObservations: []catalogs.SourceObservationLink{
				{
					Source:        catalogmeta.ProvidersID,
					ObservationID: "observation-" + id,
					ObservedAt:    generatedAt,
					Revision: catalogmeta.ObservationRevision{
						Kind:  catalogmeta.ObservationRevisionKindContentDigest,
						Value: evidence.Checksum,
					},
					Completeness:     catalogmeta.ObservationCompletenessComplete,
					Status:           catalogmeta.ObservationStatusSucceeded,
					EvidenceChecksum: evidence.Checksum,
				},
			},
			Completeness: catalogs.GenerationCompletenessComplete,
			ConsumerCompatibility: catalogs.ConsumerCompatibility{
				MinSchemaVersion: catalogs.CurrentCatalogSchemaVersion,
				MaxSchemaVersion: catalogs.CurrentCatalogSchemaVersion,
			},
		},
		Payload: payload,
	}
}
