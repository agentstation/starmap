package starmap

import (
	"context"
	"testing"

	"github.com/agentstation/starmap/pkg/catalogs"
	"github.com/agentstation/starmap/pkg/catalogs/storage"
)

// BenchmarkClientCatalog measures the public O(1) immutable catalog accessor.
func BenchmarkClientCatalog(b *testing.B) {
	client, err := New()
	if err != nil {
		b.Fatalf("New: %v", err)
	}

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if catalog := client.Catalog(); catalog == nil {
			b.Fatal("Catalog returned nil")
		}
	}
}

// BenchmarkClientUpdatePublication measures a complete immutable publication
// of the production embedded catalog through generation encoding, validation,
// durable in-memory CAS, and atomic client activation.
func BenchmarkClientUpdatePublication(b *testing.B) {
	client, err := New(WithCatalogStore(storage.NewMemory()))
	if err != nil {
		b.Fatalf("New: %v", err)
	}
	candidate, err := NewCandidate(client.Catalog(), CandidateEvidence{})
	if err != nil {
		b.Fatalf("NewCandidate: %v", err)
	}

	ctx := context.Background()
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		publication, updateErr := client.Update(
			ctx,
			func(context.Context, *catalogs.Catalog) (*Candidate, error) {
				return candidate, nil
			},
		)
		if updateErr != nil {
			b.Fatalf("Update: %v", updateErr)
		}
		if !publication.Published {
			b.Fatal("Update did not publish")
		}
	}
}
