package catalogs

import (
	"testing"
)

func TestCatalogModes(t *testing.T) {
	t.Run("MemoryCatalog", func(t *testing.T) {
		// Create memory catalog (no filesystem)
		cat, err := New()
		if err != nil {
			t.Fatalf("Failed to create memory catalog: %v", err)
		}

		// Add a test model
		testModel := Model{
			ID:   "test-model-1",
			Name: "Test Model",
		}
		if err := cat.SetModel(testModel); err != nil {
			t.Fatalf("Failed to set model: %v", err)
		}

		// Verify it was added
		models := cat.Models().List()
		if len(models) != 1 {
			t.Errorf("Expected 1 model, got %d", len(models))
		}
		if models[0].ID != "test-model-1" {
			t.Errorf("Expected model ID 'test-model-1', got '%s'", models[0].ID)
		}
	})

	t.Run("EmbeddedCatalog", func(t *testing.T) {
		// Create embedded catalog
		cat, err := New(WithEmbedded())
		if err != nil {
			t.Fatalf("Failed to create embedded catalog: %v", err)
		}

		// Check for models
		models := cat.Models().List()
		if len(models) == 0 {
			t.Error("Embedded catalog should have models")
		}

		// Check for providers
		providers := cat.Providers().List()
		if len(providers) == 0 {
			t.Error("Embedded catalog should have providers")
		}
	})

	t.Run("FilesCatalog", func(t *testing.T) {
		// Create files catalog
		cat, err := New(WithFiles("../../internal/embedded/catalog"))
		if err != nil {
			t.Fatalf("Failed to create files catalog: %v", err)
		}

		// Check for models
		models := cat.Models().List()
		if len(models) == 0 {
			t.Error("Files catalog should have models")
		}

		// Check for providers
		providers := cat.Providers().List()
		if len(providers) == 0 {
			t.Error("Files catalog should have providers")
		}
	})

	t.Run("CatalogComparison", func(t *testing.T) {
		// Create both catalogs
		embCat, err := New(WithEmbedded())
		if err != nil {
			t.Fatalf("Failed to create embedded catalog: %v", err)
		}

		filesCat, err := New(WithFiles("../../internal/embedded/catalog"))
		if err != nil {
			t.Fatalf("Failed to create files catalog: %v", err)
		}

		// Compare model counts
		embModels := embCat.Models().List()
		fileModels := filesCat.Models().List()

		if len(embModels) != len(fileModels) {
			t.Errorf("Model count mismatch: embedded=%d, files=%d", len(embModels), len(fileModels))
		}

		// Compare provider counts
		embProviders := embCat.Providers().List()
		fileProviders := filesCat.Providers().List()

		if len(embProviders) != len(fileProviders) {
			t.Errorf("Provider count mismatch: embedded=%d, files=%d", len(embProviders), len(fileProviders))
		}
	})
}
