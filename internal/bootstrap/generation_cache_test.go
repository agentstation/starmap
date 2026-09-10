package bootstrap

import (
	"sync"
	"testing"

	"github.com/agentstation/starmap/pkg/catalogs"
)

func TestEmbeddedGenerationOwnsEveryReturnedCopy(t *testing.T) {
	original, err := Generation()
	if err != nil {
		t.Fatal(err)
	}
	var workers sync.WaitGroup
	for range 8 {
		workers.Go(func() {
			current, err := Generation()
			if err != nil {
				t.Error(err)
				return
			}
			current.Payload[0] = '!'
			current.Manifest.Validation.Checks[0].Name = "changed"
			current.Manifest.SourceObservations[0].Revision.Value = "changed"
		})
	}
	workers.Wait()
	current, err := Generation()
	if err != nil {
		t.Fatal(err)
	}
	if current.Manifest.Payload.Checksum != original.Manifest.Payload.Checksum {
		t.Fatal("caller changed the embedded payload identity")
	}
	if err := current.Validate(); err != nil {
		t.Fatalf("caller mutation reached retained generation: %v", err)
	}
	if current.Manifest.Validation.Checks[0].Name != original.Manifest.Validation.Checks[0].Name || current.Manifest.SourceObservations[0].Revision.Value != original.Manifest.SourceObservations[0].Revision.Value {
		t.Fatal("caller mutation reached retained metadata")
	}
}

var benchmarkGeneration catalogs.Generation

func BenchmarkEmbeddedGeneration(b *testing.B) {
	warm, err := Generation()
	if err != nil {
		b.Fatal(err)
	}
	b.SetBytes(int64(len(warm.Payload)))
	b.ReportAllocs()
	for b.Loop() {
		generation, err := Generation()
		if err != nil {
			b.Fatal(err)
		}
		benchmarkGeneration = generation
	}
}
