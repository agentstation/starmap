package permission

import (
	"testing"
	"time"

	"github.com/agentstation/starmap/pkg/catalogs"
	"github.com/agentstation/starmap/pkg/catalogs/evidence"
	"github.com/agentstation/starmap/pkg/catalogs/storage"
)

func issuerGeneration(t *testing.T, id string, sequence uint64) catalogs.Generation {
	t.Helper()
	payload, err := catalogs.EncodeCatalogPayload(catalogs.NewEmpty())
	if err != nil {
		t.Fatal(err)
	}
	at := time.Date(2026, 9, 10, 12, 0, 0, 0, time.UTC)
	description := catalogs.DescribeCatalogPayload(payload)
	generation := catalogs.Generation{
		Manifest: catalogs.GenerationManifest{
			ManifestVersion: catalogs.AuthorityGenerationManifestVersion,
			SchemaVersion:   catalogs.CurrentCatalogSchemaVersion, GenerationID: id, GeneratedAt: at,
			Payload: description, SyncRunID: "issuer-" + id,
			Validation: catalogs.GenerationValidationReport{
				ValidatorVersion: "issuer-test/v1", ValidatedAt: at, Status: catalogs.GenerationValidationPassed,
				Checks: []catalogs.GenerationValidationCheck{{Name: "schema", Status: catalogs.GenerationValidationCheckPassed}},
			},
			SourceObservations: []catalogs.SourceObservationLink{{
				Source: evidence.ProvidersID, ObservationID: "issuer-" + id, ObservedAt: at,
				Revision:     evidence.ObservationRevision{Kind: evidence.ObservationRevisionKindContentDigest, Value: description.Checksum},
				Completeness: evidence.ObservationCompletenessComplete, Status: evidence.ObservationStatusSucceeded,
				EvidenceChecksum: description.Checksum,
			}},
			ReviewCandidates: []evidence.ReviewCandidate{}, Completeness: catalogs.GenerationCompletenessComplete,
			ConsumerCompatibility: catalogs.ConsumerCompatibility{MinSchemaVersion: catalogs.CurrentCatalogSchemaVersion, MaxSchemaVersion: catalogs.CurrentCatalogSchemaVersion},
			AuthorityHead: catalogs.CatalogAuthorityHead{
				AuthorityID: "enterprise", PolicyID: "production", Sequence: sequence, GenerationID: id,
				PayloadChecksum: description.Checksum, RequiredPermissionRevision: catalogs.DescribeCatalogPayload([]byte(id)).Checksum,
				PermissionSchemaVersion: catalogs.CatalogPermissionSchemaVersion,
			},
		},
		Payload: payload,
	}
	if err := generation.Validate(); err != nil {
		t.Fatal(err)
	}
	return generation
}

func TestIssuerObservesCurrentAuthority(t *testing.T) {
	store := storage.NewMemory()
	first := issuerGeneration(t, "first-publication", 1)
	if err := store.Commit(t.Context(), first, ""); err != nil {
		t.Fatal(err)
	}
	now := time.Date(2026, 9, 10, 12, 1, 0, 0, time.UTC)
	issuer, err := NewIssuer(store, IssuerConfig{
		AuthorityID: "enterprise", PolicyID: "production", Lifetime: 5 * time.Minute,
		Clock: func() ClockReading { return ClockReading{Time: now, Known: true} },
	})
	if err != nil {
		t.Fatal(err)
	}
	receipt, err := issuer.ReadPermission(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	if receipt.Head != first.Manifest.AuthorityHead || !receipt.IssuedAt.Equal(now) || !receipt.ValidUntil.Equal(now.Add(5*time.Minute)) {
		t.Fatalf("receipt=%+v", receipt)
	}
	second := issuerGeneration(t, "withdrawal-publication", 2)
	if err := store.Commit(t.Context(), second, first.Manifest.GenerationID); err != nil {
		t.Fatal(err)
	}
	now = now.Add(time.Minute)
	receipt, err = issuer.ReadPermission(t.Context())
	if err != nil || receipt.Head != second.Manifest.AuthorityHead {
		t.Fatalf("receipt=%+v error=%v", receipt, err)
	}
}
