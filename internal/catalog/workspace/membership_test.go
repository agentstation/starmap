package workspace

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/agentstation/starmap/internal/constants"
	"github.com/agentstation/starmap/pkg/catalogs"
)

func TestProjectPreservesAndRemovesMembershipRecords(t *testing.T) {
	builder := catalogs.NewEmpty()
	scope := catalogs.ProviderMembershipScope{
		PublisherID: "upstream", BindingID: "provider", BindingRevision: "1", ProviderID: "provider",
		Public: true, Region: "default", APISurface: "models", Authority: catalogs.MembershipScopeAuthority,
		Inventory: &catalogs.MembershipInventory{ObservationID: "complete", ObservedAt: time.Date(2026, 9, 16, 0, 0, 0, 0, time.UTC), ModelIDs: []string{"one"}},
	}
	path := filepath.Join(t.TempDir(), "catalog")
	for _, mode := range []string{"complete", "empty", "removed"} {
		t.Run(mode, func(t *testing.T) {
			scopes := []catalogs.ProviderMembershipScope{scope}
			if mode == "empty" {
				scopes[0].Inventory.ModelIDs = []string{}
			}
			if mode == "removed" {
				scopes = nil
			}
			if err := builder.SetMembershipScopes(scopes); err != nil {
				t.Fatal(err)
			}
			catalog, err := builder.Build()
			if err != nil {
				t.Fatal(err)
			}
			before, err := catalogs.EncodeCatalogPayload(catalog)
			if err != nil {
				t.Fatal(err)
			}
			identity := Identity{GenerationID: mode, PayloadChecksum: catalogs.DescribeCatalogPayload(before).Checksum}
			if _, err := Project(t.Context(), path, catalog, identity); err != nil {
				t.Fatal(err)
			}
			loaded, err := catalogs.NewFromPath(path)
			if err != nil {
				t.Fatal(err)
			}
			after, err := catalogs.EncodeCatalogPayload(loaded)
			if err != nil {
				t.Fatal(err)
			}
			if !bytes.Equal(before, after) {
				t.Fatal("projection changed scope membership")
			}
			notes := filepath.Join(path, "notes.txt")
			if mode == "complete" {
				if err := os.WriteFile(notes, []byte("operator notes\n"), constants.FilePermissions); err != nil {
					t.Fatal(err)
				}
			} else if data, err := os.ReadFile(notes); err != nil || string(data) != "operator notes\n" {
				t.Fatalf("operator notes changed: %v", err)
			}
			if mode == "removed" {
				if _, err := os.Stat(filepath.Join(path, "membership-scopes.yaml")); !os.IsNotExist(err) {
					t.Fatalf("removed scope file survived: %v", err)
				}
			}
		})
	}
}
