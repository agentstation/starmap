package memory

import (
	"testing"
)

func TestMemoryCatalog(t *testing.T) {
	catalog, err := New()
	if err != nil {
		t.Fatalf("Failed to create memory catalog: %v", err)
	}
	if catalog == nil {
		t.Fatal("Expected catalog, got nil")
	}

	// Test catalog functionality
	if catalog.Providers() == nil {
		t.Error("Expected providers collection")
	}

	// Test with read-only option
	catalog2, err := New(WithReadOnly(true))
	if err != nil {
		t.Fatalf("Failed to create read-only memory catalog: %v", err)
	}
	if catalog2 == nil {
		t.Fatal("Expected catalog, got nil")
	}
}

func TestMemoryCatalogOptions(t *testing.T) {
	// Test memory options
	t.Run("MemoryOptions", func(t *testing.T) {
		testData := []byte(`{"test": "data"}`)
		_, err := New(
			WithReadOnly(false),
			WithPreload(testData),
		)
		if err != nil {
			t.Errorf("Failed to create memory catalog with options: %v", err)
		}
	})
}