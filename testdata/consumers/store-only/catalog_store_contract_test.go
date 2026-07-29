package consumer

import (
	"context"
	stderrors "errors"
	"fmt"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"

	"github.com/agentstation/starmap/pkg/catalogmeta"
	"github.com/agentstation/starmap/pkg/catalogs"
	"github.com/agentstation/starmap/pkg/catalogstore"
	"github.com/agentstation/starmap/pkg/errors"
)

func TestStarportOwnedStoreBehavioralContract(t *testing.T) {
	t.Parallel()
	store := newStarportStore()
	ctx := context.Background()

	if _, err := store.Current(ctx); !errors.IsNotFound(err) {
		t.Fatalf("empty Current error = %v, want typed not found", err)
	}

	first := externalGeneration("external-generation-1", "first")
	pristineFirst := first.Copy()
	if err := store.Commit(ctx, first, ""); err != nil {
		t.Fatalf("Commit first: %v", err)
	}
	if err := store.Commit(ctx, pristineFirst, ""); err != nil {
		t.Fatalf("idempotent retry: %v", err)
	}
	first.Payload[0] ^= 0xff
	first.Manifest.SourceObservations[0].ObservationID = "mutated-input"
	assertExternalGeneration(t, store, pristineFirst)

	returned, err := store.Current(ctx)
	if err != nil {
		t.Fatalf("Current: %v", err)
	}
	returned.Payload[0] ^= 0xff
	returned.Manifest.Validation.Checks[0].Name = "mutated-output"
	assertExternalGeneration(t, store, pristineFirst)

	second := externalGeneration("external-generation-2", "second")
	if err := store.Commit(ctx, second, "external-generation-1"); err != nil {
		t.Fatalf("Commit second: %v", err)
	}
	retained, err := store.Get(ctx, "external-generation-1")
	if err != nil {
		t.Fatalf("Get retained generation: %v", err)
	}
	if diff := cmp.Diff(pristineFirst, retained); diff != "" {
		t.Fatalf("retained generation mismatch (-want +got):\n%s", diff)
	}

	stale := externalGeneration("external-generation-stale", "stale")
	if err := store.Commit(ctx, stale, "external-generation-1"); !errors.IsConflict(err) {
		t.Fatalf("stale Commit error = %v, want typed conflict", err)
	}
	if _, err := store.Get(ctx, "external-generation-stale"); !errors.IsNotFound(err) {
		t.Fatalf("stale candidate error = %v, want typed not found", err)
	}
	assertExternalGeneration(t, store, second)

	corrupt := externalGeneration("external-generation-corrupt", "corrupt")
	corrupt.Payload = append(corrupt.Payload, 'x')
	if err := store.Commit(ctx, corrupt, "external-generation-2"); !stderrors.Is(err, errors.ErrInvalidInput) {
		t.Fatalf("corrupt Commit error = %v, want typed invalid input", err)
	}
	assertExternalGeneration(t, store, second)

	if err := store.Commit(ctx, pristineFirst, "external-generation-2"); err != nil {
		t.Fatalf("rollback to retained generation: %v", err)
	}
	assertExternalGeneration(t, store, pristineFirst)

	canceled, cancel := context.WithCancel(ctx)
	cancel()
	if err := store.Commit(canceled, second, "external-generation-1"); !stderrors.Is(err, context.Canceled) {
		t.Fatalf("canceled Commit error = %v, want context.Canceled", err)
	}
	assertExternalGeneration(t, store, pristineFirst)
}

func assertExternalGeneration(
	t *testing.T,
	store catalogstore.Store,
	want catalogstore.Generation,
) {
	t.Helper()
	got, err := store.Current(context.Background())
	if err != nil {
		t.Fatalf("Current: %v", err)
	}
	if diff := cmp.Diff(want, got); diff != "" {
		t.Fatalf("Current mismatch (-want +got):\n%s", diff)
	}
}

func externalGeneration(id, value string) catalogstore.Generation {
	payload := fmt.Appendf(nil, `{"value":%q}`, value)
	evidence := catalogs.DescribeCatalogPayload([]byte("evidence:" + value))
	generatedAt := time.Date(2026, time.July, 29, 19, 30, 0, 0, time.UTC)
	return catalogstore.Generation{
		Manifest: catalogs.GenerationManifest{
			ManifestVersion: catalogs.CurrentGenerationManifestVersion,
			SchemaVersion:   catalogs.CurrentCatalogSchemaVersion,
			GenerationID:    id,
			GeneratedAt:     generatedAt,
			Payload:         catalogs.DescribeCatalogPayload(payload),
			Validation: catalogs.GenerationValidationReport{
				ValidatorVersion: "external-store-validator/v1",
				ValidatedAt:      generatedAt.Add(time.Second),
				Status:           catalogs.GenerationValidationPassed,
				Checks: []catalogs.GenerationValidationCheck{
					{Name: "schema", Status: catalogs.GenerationValidationCheckPassed},
				},
			},
			SyncRunID: "sync-" + id,
			SourceObservations: []catalogs.SourceObservationLink{
				{
					Source:        catalogmeta.LocalCatalogID,
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
