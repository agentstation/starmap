package main

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/agentstation/starmap/pkg/catalogs"
)

func TestBootstrapManifestPreservesCanonicalRenameHistory(t *testing.T) {
	for _, change := range []string{"unchanged", "removed", "dropped", "reassigned"} {
		t.Run(change, func(t *testing.T) {
			builder := catalogs.NewEmpty()
			author := catalogs.Author{ID: "author", Name: "Author"}
			if err := builder.SetAuthor(author); err != nil {
				t.Fatal(err)
			}
			for _, id := range []string{"current", "other"} {
				if err := builder.SetAuthorModel(author.ID, catalogs.Model{ID: id, Name: id, Authors: []catalogs.Author{author}}); err != nil {
					t.Fatal(err)
				}
			}
			alias := catalogs.CanonicalAlias{ID: "author/old", TargetID: "author/current", PublisherID: "baseline", State: catalogs.CanonicalAliasActive}
			if err := builder.SetCanonicalAliasRecords([]catalogs.CanonicalAlias{alias}); err != nil {
				t.Fatal(err)
			}
			previous := t.TempDir()
			if err := builder.SaveTo(previous); err != nil {
				t.Fatal(err)
			}
			records := []catalogs.CanonicalAlias{alias}
			switch change {
			case "removed":
				records[0].State = catalogs.CanonicalAliasRemoved
			case "dropped":
				records = nil
			case "reassigned":
				records[0].TargetID = "author/other"
			}
			if err := builder.SetCanonicalAliasRecords(records); err != nil {
				t.Fatal(err)
			}
			candidate := t.TempDir()
			if err := builder.SaveTo(candidate); err != nil {
				t.Fatal(err)
			}
			output := filepath.Join(t.TempDir(), "generation.json")
			err := run([]string{"--catalog-dir", candidate, "--previous-catalog-dir", previous, "--output", output}, &bytes.Buffer{}, time.Now().UTC())
			if change == "dropped" || change == "reassigned" {
				if err == nil {
					t.Fatal("producer accepted invalid rename history")
				}
				if _, statErr := os.Stat(output); !os.IsNotExist(statErr) {
					t.Fatal("invalid history wrote generation metadata")
				}
			} else if err != nil {
				t.Fatal(err)
			}
		})
	}
}
