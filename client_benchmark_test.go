package starmap

import "testing"

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
