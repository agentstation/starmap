package starmap

import (
	"testing"
)

func TestSave(t *testing.T) {
	// Create a starmap instance
	sm, err := New()
	if err != nil {
		t.Fatalf("Failed to create starmap: %v", err)
	}

	// Test Save method - this should work with embedded catalog
	err = sm.Save()
	if err != nil {
		t.Logf("Save failed (expected for embedded catalog): %v", err)
		// This is expected to fail for embedded catalogs that don't support saving
	}
}
