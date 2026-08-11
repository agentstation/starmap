package remote

import (
	"context"
	"testing"

	"github.com/agentstation/starmap"
	"github.com/agentstation/starmap/pkg/catalogstore"
)

// BenchmarkSubscriberActivation measures verification, decoding, durable
// in-memory commit, and atomic activation of the complete embedded generation.
// Subscriber construction is deliberately outside the timed region.
func BenchmarkSubscriberActivation(b *testing.B) {
	source, err := starmap.New()
	if err != nil {
		b.Fatalf("New source: %v", err)
	}
	generation, err := source.CurrentGeneration(context.Background())
	if err != nil {
		b.Fatalf("CurrentGeneration: %v", err)
	}

	ctx := context.Background()
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		b.StopTimer()
		subscriber, newErr := New(Config{
			BaseURL: "https://starmap.invalid", CatalogStore: catalogstore.NewMemory(),
		})
		if newErr != nil {
			b.Fatalf("New subscriber: %v", newErr)
		}
		b.StartTimer()

		published, activateErr := subscriber.activate(ctx, generation)
		if activateErr != nil {
			b.Fatalf("activate: %v", activateErr)
		}
		if !published {
			b.Fatal("generation was not activated")
		}
	}
}
