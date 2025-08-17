package embedded

import (
	"testing"
)

func TestEmbeddedCatalog(t *testing.T) {
	catalog, err := New()
	if err != nil {
		t.Fatalf("Failed to create embedded catalog: %v", err)
	}
	if catalog == nil {
		t.Fatal("Expected catalog, got nil")
	}

	// Test catalog functionality
	if catalog.Providers().Len() == 0 {
		t.Error("Expected embedded catalog to have providers")
	}

	// Test with no auto-load option
	catalog2, err := New(WithNoAutoLoad())
	if err != nil {
		t.Fatalf("Failed to create embedded catalog with no auto-load: %v", err)
	}
	if catalog2 == nil {
		t.Fatal("Expected catalog, got nil")
	}

	// Should not be loaded yet
	if catalog2.Providers().Len() > 0 {
		t.Error("Expected empty catalog when auto-load is disabled")
	}
}

func TestEmbeddedCatalogOptions(t *testing.T) {
	// Test embedded options
	t.Run("EmbeddedOptions", func(t *testing.T) {
		_, err := New(
			WithAutoLoad(true),
			WithNoAutoLoad(),
		)
		if err != nil {
			t.Errorf("Failed to create embedded catalog with options: %v", err)
		}
	})
}