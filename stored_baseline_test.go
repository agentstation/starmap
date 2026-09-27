package starmap_test

import (
	"context"
	"testing"

	"github.com/agentstation/starmap"
	"github.com/agentstation/starmap/pkg/catalogs"
	"github.com/agentstation/starmap/pkg/catalogs/storage"
)

type storedBaselineFixture struct {
	storage.Store
	generation catalogs.Generation
}

func (s storedBaselineFixture) Current(context.Context) (catalogs.Generation, error) {
	return s.generation.Copy(), nil
}

func TestStoredBaselineReusesVerifiedCatalog(t *testing.T) {
	client, err := starmap.NewContext(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	generation, err := client.CurrentGeneration(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	generation.Manifest.GenerationID = "retained-baseline-identity"
	restarted, err := starmap.NewContext(t.Context(), starmap.WithCatalogStore(storedBaselineFixture{
		Store: storage.NewMemory(), generation: generation,
	}))
	if err != nil {
		t.Fatal(err)
	}
	if restarted.Catalog() != client.EmbeddedCatalogState().Catalog {
		t.Fatal("stored baseline duplicates the verified immutable catalog")
	}
	if restarted.CurrentGenerationID() != generation.Manifest.GenerationID {
		t.Fatal("catalog reuse replaced the retained generation identity")
	}
	if restarted.Readiness().Embedded.Active {
		t.Fatal("stored generation was classified as an uncommitted bootstrap")
	}
}

func TestStoredBaselineReuseStillValidatesGeneration(t *testing.T) {
	client, err := starmap.NewContext(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	baseline, err := client.CurrentGeneration(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		name   string
		change func(*catalogs.Generation)
	}{
		{"checksum", func(g *catalogs.Generation) { g.Payload[0] ^= 1 }},
		{"schema", func(g *catalogs.Generation) { g.Manifest.SchemaVersion-- }},
		{"missing observations", func(g *catalogs.Generation) { g.Manifest.SourceObservations = nil }},
		{"unrelated observations", func(g *catalogs.Generation) {
			for i := range g.Manifest.SourceObservations {
				g.Manifest.SourceObservations[i].ObservationID += "-unrelated"
			}
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			generation := baseline.Copy()
			tc.change(&generation)
			connected, err := starmap.NewContext(t.Context(), starmap.WithCatalogStore(storedBaselineFixture{
				Store: storage.NewMemory(), generation: generation,
			}))
			if err == nil || connected != nil {
				t.Fatal("invalid stored generation was accepted")
			}
		})
	}
}
