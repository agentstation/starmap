package permission

import (
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/agentstation/starmap/pkg/catalogs"
	"github.com/agentstation/starmap/pkg/catalogs/storage"
)

func TestIssuerDoesNotRequireCatalogPayloadOrManifestCompatibility(t *testing.T) {
	root := filepath.Join(t.TempDir(), "catalog")
	store, err := storage.NewFilesystem(root)
	if err != nil {
		t.Fatal(err)
	}
	generation := issuerGeneration(t, "independent-issuer", 1)
	generation.Manifest.AuthorityHead.PermissionSchemaVersion++
	if err := store.Commit(t.Context(), generation, ""); err != nil {
		t.Fatal(err)
	}
	digest := sha256.Sum256([]byte(generation.Manifest.GenerationID))
	directory := filepath.Join(root, "generations", hex.EncodeToString(digest[:]))
	if err := os.Remove(filepath.Join(directory, "catalog.json")); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(directory, "manifest.json"), []byte("unsupported manifest"), 0); err != nil {
		t.Fatal(err)
	}
	if _, err := store.Current(t.Context()); err == nil {
		t.Fatal("catalog fixture remains readable")
	}
	issuer, err := NewIssuer(store, IssuerConfig{
		AuthorityID: "enterprise", PolicyID: "production",
		Clock: func() ClockReading {
			return ClockReading{Time: time.Date(2026, 9, 10, 12, 0, 0, 0, time.UTC), Known: true}
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	receipt, err := issuer.ReadPermission(t.Context())
	if err != nil || receipt.Head != generation.Manifest.AuthorityHead {
		t.Fatalf("receipt=%+v error=%v", receipt, err)
	}
	if receipt.Head.SupportsPermissions() {
		t.Fatal("future permission semantics became compatible")
	}
	if err := receipt.Validate(); err != nil {
		t.Fatal(err)
	}
	if receipt.Version != catalogs.CatalogPermissionEnvelopeVersion {
		t.Fatal("wrong envelope version")
	}
}
