package embedded

import (
	"testing"

	"github.com/agentstation/starmap/internal/catalogs/files"
)

// TestEmbeddedLoad tests that the embedded catalog can load its embedded data
func TestEmbeddedLoad(t *testing.T) {
	catalog := NewCatalog()
	if err := catalog.Load(); err != nil {
		t.Fatalf("Failed to load embedded catalog: %v", err)
	}

	// Basic sanity checks - just verify some data loaded
	if catalog.Providers().Len() == 0 {
		t.Error("Expected some providers to be loaded from embedded data")
	}
	if catalog.Models().Len() == 0 {
		t.Error("Expected some models to be loaded from embedded data")
	}
}

// For testing file loading logic, use the files catalog which is designed for that
func TestFileLoadingLogic(t *testing.T) {
	// Just test that the files catalog exists and can be instantiated
	catalog := files.NewCatalog("testdata")
	if catalog == nil {
		t.Error("Expected files catalog to be creatable")
	}

	// The files catalog has its own tests in the files package
	// We don't need to duplicate that logic here
}
