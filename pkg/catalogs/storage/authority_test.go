package storage

import (
	"errors"
	"strings"
	"testing"

	"github.com/agentstation/starmap/pkg/catalogs"
)

func authorityStoredGeneration(id string, sequence uint64) catalogs.Generation {
	generation := testGeneration(id, id)
	generation.Manifest.ManifestVersion = catalogs.AuthorityGenerationManifestVersion
	generation.Manifest.AuthorityHead = catalogs.CatalogAuthorityHead{
		AuthorityID: "enterprise", PolicyID: "production", Sequence: sequence,
		GenerationID: id, PayloadChecksum: generation.Manifest.Payload.Checksum,
		RequiredPermissionRevision: "sha256:" + strings.Repeat("a", 64),
		PermissionSchemaVersion:    catalogs.CatalogPermissionSchemaVersion,
	}
	return generation
}

func TestCatalogStoresCommitAuthorityWithGeneration(t *testing.T) {
	for name, factory := range catalogStoreFactories() {
		t.Run(name, func(t *testing.T) {
			store := factory(t)
			first := authorityStoredGeneration("authority-first", 1)
			if err := store.Commit(t.Context(), first, ""); err != nil {
				t.Fatal(err)
			}
			assertStoredGeneration(t, store, first)
			got, err := store.Current(t.Context())
			if err != nil {
				t.Fatal(err)
			}
			got.Manifest.AuthorityHead.RequiredPermissionRevision = "sha256:" + strings.Repeat("b", 64)
			assertStoredGeneration(t, store, first)
			if err := store.Commit(t.Context(), got, first.Manifest.GenerationID); err == nil {
				t.Fatal("changed the permission revision under an existing immutable identity")
			}
			second := authorityStoredGeneration("authority-second", 2)
			second.Manifest.AuthorityHead.RequiredPermissionRevision = "sha256:" + strings.Repeat("c", 64)
			if err := store.Commit(t.Context(), second, "wrong-head"); err == nil {
				t.Fatal("accepted a stale catalog head")
			}
			assertStoredGeneration(t, store, first)
			if err := store.Commit(t.Context(), second, first.Manifest.GenerationID); err != nil {
				t.Fatal(err)
			}
			assertStoredGeneration(t, store, second)
			if err := store.Commit(t.Context(), second, first.Manifest.GenerationID); err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestFilesystemAuthorityHeadSurvivesFailedPublicationAndReopen(t *testing.T) {
	root := privateFilesystemRoot(t)
	store, err := NewFilesystem(root)
	if err != nil {
		t.Fatal(err)
	}
	first := authorityStoredGeneration("authority-before", 1)
	if err := store.Commit(t.Context(), first, ""); err != nil {
		t.Fatal(err)
	}
	second := authorityStoredGeneration("authority-after", 2)
	second.Manifest.AuthorityHead.RequiredPermissionRevision = "sha256:" + strings.Repeat("c", 64)
	fault := errors.New("interrupt before authority head publication")
	store.beforeCurrentPromotion = func() error { return fault }
	if err := store.Commit(t.Context(), second, first.Manifest.GenerationID); !errors.Is(err, fault) {
		t.Fatalf("commit error=%v", err)
	}
	reopened, err := NewFilesystem(root)
	if err != nil {
		t.Fatal(err)
	}
	assertStoredGeneration(t, reopened, first)
	staged, err := reopened.Get(t.Context(), second.Manifest.GenerationID)
	if err != nil {
		t.Fatal(err)
	}
	if staged.Manifest.AuthorityHead != second.Manifest.AuthorityHead {
		t.Fatal("staged generation lost its required permission revision")
	}
	if err := reopened.Commit(t.Context(), second, first.Manifest.GenerationID); err != nil {
		t.Fatal(err)
	}
	assertStoredGeneration(t, reopened, second)
}
