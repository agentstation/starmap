package local

import (
	"testing"

	"github.com/agentstation/starmap/pkg/catalogs"
	"github.com/agentstation/starmap/pkg/errors"
)

func TestLocalAliasProjectionRequiresUnchangedBaseline(t *testing.T) {
	for _, mode := range []string{"provided", "path"} {
		for _, state := range []catalogs.CanonicalAliasState{catalogs.CanonicalAliasActive, catalogs.CanonicalAliasRemoved} {
			t.Run(mode+"/"+string(state), func(t *testing.T) {
				builder := catalogs.NewEmpty()
				author := catalogs.Author{ID: "author", Name: "Author"}
				if err := builder.SetAuthor(author); err != nil {
					t.Fatal(err)
				}
				if err := builder.SetAuthorModel(author.ID, catalogs.Model{ID: "current", Name: "Current", Authors: []catalogs.Author{author}}); err != nil {
					t.Fatal(err)
				}
				alias := catalogs.CanonicalAlias{ID: "author/old", TargetID: "author/current", PublisherID: "baseline", State: catalogs.CanonicalAliasActive}
				if err := builder.SetCanonicalAliasRecords([]catalogs.CanonicalAlias{alias}); err != nil {
					t.Fatal(err)
				}
				baseline, err := builder.Build()
				if err != nil {
					t.Fatal(err)
				}
				alias.State = state
				if err := builder.SetCanonicalAliasRecords([]catalogs.CanonicalAlias{alias}); err != nil {
					t.Fatal(err)
				}
				catalog, err := builder.Build()
				if err != nil {
					t.Fatal(err)
				}
				option := WithCatalog(catalog)
				if mode == "path" {
					directory := t.TempDir()
					if err := builder.SaveTo(directory); err != nil {
						t.Fatal(err)
					}
					option = WithCatalogPath(directory)
				}
				observation, err := New(option, WithAliasBaseline(baseline)).Observe(t.Context())
				if state == catalogs.CanonicalAliasRemoved {
					if !errors.IsConflict(err) {
						t.Fatalf("local alias edit = %v", err)
					}
					return
				}
				if err != nil {
					t.Fatal(err)
				}
				if err := observation.Validate(); err != nil {
					t.Fatal(err)
				}
				if len(observation.Catalog.CanonicalAliasRecords()) != 0 || len(baseline.CanonicalAliasRecords()) != 1 || len(catalog.CanonicalAliasRecords()) != 1 {
					t.Fatal("local facts changed alias authority or the original inventory")
				}
			})
		}
	}
}
