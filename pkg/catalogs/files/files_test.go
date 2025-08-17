package files

import (
	"testing"
)

func TestFilesCatalog(t *testing.T) {
	tempDir := t.TempDir()

	catalog, err := New(tempDir, WithNoAutoLoad())
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
	_, err := New("")
	if err == nil {
		t.Error("Expected error for files catalog without base path")
	}
}

func TestFilesCatalogOptions(t *testing.T) {
	// Test files options
	t.Run("FilesOptions", func(t *testing.T) {
		tempDir := t.TempDir()
		_, err := New(tempDir,
			WithAutoLoad(false),
			WithReadOnly(true),
		)
		if err != nil {
			t.Errorf("Failed to create files catalog with options: %v", err)
		}
	})
}