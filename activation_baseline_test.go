package starmap

import (
	"bytes"
	"testing"

	"github.com/agentstation/starmap/pkg/catalogs"
	"github.com/agentstation/starmap/pkg/catalogs/storage"
)

func TestActivateEmbeddedBaselineReusesVerifiedFacts(t *testing.T) {
	store := storage.NewMemory()
	client, err := NewContext(t.Context(), WithCatalogStore(store))
	if err != nil {
		t.Fatal(err)
	}
	baseline := client.EmbeddedCatalogState()
	generation, err := client.CurrentGeneration(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	if _, err := client.Activate(t.Context(), rootRemoteGeneration(t)); err != nil {
		t.Fatal(err)
	}
	if _, err := client.Activate(t.Context(), generation); err != nil {
		t.Fatal(err)
	}
	if client.Catalog() != baseline.Catalog {
		t.Fatal("activation duplicated verified immutable baseline facts")
	}
	stored, err := store.Current(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	if stored.Manifest.GenerationID != baseline.GenerationID || !bytes.Equal(stored.Payload, generation.Payload) {
		t.Fatal("activation changed baseline identity or payload")
	}
}

func TestActivateEmbeddedBaselineRejectsInvalidEvidence(t *testing.T) {
	client, err := NewContext(t.Context())
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
		{"payload", func(g *catalogs.Generation) { g.Payload[0] ^= 1 }},
		{"schema", func(g *catalogs.Generation) { g.Manifest.SchemaVersion-- }},
		{"missing observations", func(g *catalogs.Generation) { g.Manifest.SourceObservations = nil }},
		{"unrelated observations", func(g *catalogs.Generation) {
			for i := range g.Manifest.SourceObservations {
				g.Manifest.SourceObservations[i].ObservationID += "-unrelated"
			}
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			store := storage.NewMemory()
			client, err := NewContext(t.Context(), WithCatalogStore(store))
			if err != nil {
				t.Fatal(err)
			}
			before := client.CurrentCatalogState()
			candidate := baseline.Copy()
			tc.change(&candidate)
			if _, err := client.Activate(t.Context(), candidate); err == nil {
				t.Fatal("activation accepted invalid baseline evidence")
			}
			if client.CurrentCatalogState() != before {
				t.Fatal("rejected evidence changed serving state")
			}
			if _, err := store.Current(t.Context()); err == nil {
				t.Fatal("rejected evidence reached the store")
			}
		})
	}
}

func BenchmarkActivateEmbeddedBaseline(b *testing.B) {
	client, err := NewContext(b.Context())
	if err != nil {
		b.Fatal(err)
	}
	generation, err := client.CurrentGeneration(b.Context())
	if err != nil {
		b.Fatal(err)
	}
	b.ReportAllocs()
	for b.Loop() {
		client, err := NewContext(b.Context(), WithCatalogStore(storage.NewMemory()))
		if err != nil {
			b.Fatal(err)
		}
		if _, err := client.Activate(b.Context(), generation); err != nil {
			b.Fatal(err)
		}
	}
}
