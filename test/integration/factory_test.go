package integration

import (
	"testing"

	"github.com/agentstation/starmap"
)

func TestEmbeddedCatalog(t *testing.T) {
	catalog, err := starmap.NewEmbeddedCatalog()
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
	catalog2, err := starmap.NewEmbeddedCatalog(starmap.WithEmbeddedNoAutoLoad())
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

func TestFilesCatalog(t *testing.T) {
	tempDir := t.TempDir()

	catalog, err := starmap.NewFilesCatalog(tempDir, starmap.WithFilesNoAutoLoad())
	if err != nil {
		t.Fatalf("Failed to create files catalog: %v", err)
	}
	if catalog == nil {
		t.Fatal("Expected catalog, got nil")
	}

	// Test catalog functionality
	if catalog.Providers() == nil {
		t.Error("Expected providers collection")
	}
}

func TestFilesCatalogWithoutPath(t *testing.T) {
	_, err := starmap.NewFilesCatalog("")
	if err == nil {
		t.Error("Expected error for files catalog without base path")
	}
}

func TestMemoryCatalog(t *testing.T) {
	catalog, err := starmap.NewMemoryCatalog()
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
	catalog2, err := starmap.NewMemoryCatalog(starmap.WithMemoryReadOnly(true))
	if err != nil {
		t.Fatalf("Failed to create read-only memory catalog: %v", err)
	}
	if catalog2 == nil {
		t.Fatal("Expected catalog, got nil")
	}
}

func TestCatalogOptions(t *testing.T) {
	// Test embedded options
	t.Run("EmbeddedOptions", func(t *testing.T) {
		_, err := starmap.NewEmbeddedCatalog(
			starmap.WithEmbeddedAutoLoad(true),
			starmap.WithEmbeddedNoAutoLoad(),
		)
		if err != nil {
			t.Errorf("Failed to create embedded catalog with options: %v", err)
		}
	})

	// Test files options
	t.Run("FilesOptions", func(t *testing.T) {
		tempDir := t.TempDir()
		_, err := starmap.NewFilesCatalog(tempDir,
			starmap.WithFilesAutoLoad(false),
			starmap.WithFilesReadOnly(true),
		)
		if err != nil {
			t.Errorf("Failed to create files catalog with options: %v", err)
		}
	})

	// Test memory options
	t.Run("MemoryOptions", func(t *testing.T) {
		testData := []byte(`{"test": "data"}`)
		_, err := starmap.NewMemoryCatalog(
			starmap.WithMemoryReadOnly(false),
			starmap.WithMemoryPreload(testData),
		)
		if err != nil {
			t.Errorf("Failed to create memory catalog with options: %v", err)
		}
	})
}
