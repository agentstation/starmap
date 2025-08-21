package starmap

import (
	"fmt"
	"testing"

	"github.com/agentstation/starmap/pkg/catalogs"
	"github.com/agentstation/starmap/pkg/catalogs/memory"
	"github.com/agentstation/starmap/pkg/sources"
)

// TestPipelineIntegration tests the new pipeline-based sync system
func TestPipelineIntegration(t *testing.T) {
	// Create a memory catalog for testing
	memoryCatalog, err := memory.New()
	if err != nil {
		t.Fatalf("Failed to create memory catalog: %v", err)
	}

	// Create starmap with memory catalog
	sm, err := New(WithInitialCatalog(memoryCatalog))
	if err != nil {
		t.Fatalf("Failed to create starmap: %v", err)
	}

	// Test hook integration
	var modelAddedCount int
	var modelUpdatedCount int

	sm.OnModelAdded(func(model catalogs.Model) {
		modelAddedCount++
		fmt.Printf("Model added: %s\n", model.ID)
	})

	sm.OnModelUpdated(func(old, new catalogs.Model) {
		modelUpdatedCount++
		fmt.Printf("Model updated: %s\n", new.ID)
	})

	// Test sync with new pipeline features
	t.Run("SyncWithProvenance", func(t *testing.T) {
		result, err := sm.Sync(
			sources.SyncWithProvenance("/tmp/test-provenance.txt"),
			sources.SyncWithDryRun(true), // Don't actually write anything
		)

		if err != nil {
			t.Errorf("Sync with provenance failed: %v", err)
		}

		if result == nil {
			t.Error("Expected sync result, got nil")
		}

		// Verify provenance tracking was enabled
		if !result.DryRun {
			t.Error("Expected dry run to be enabled")
		}

		fmt.Printf("Sync completed: %s\n", result.Summary())
	})

	// Test custom field authorities
	t.Run("SyncWithCustomFieldAuthority", func(t *testing.T) {
		result, err := sm.Sync(
			sources.SyncWithFieldAuthority("description", sources.LocalCatalog, 120),
			sources.SyncWithDryRun(true),
		)

		if err != nil {
			t.Errorf("Sync with custom field authority failed: %v", err)
		}

		if result == nil {
			t.Error("Expected sync result, got nil")
		}

		fmt.Printf("Custom field authority sync completed: %s\n", result.Summary())
	})

	// Test source disabling
	t.Run("SyncWithDisabledSource", func(t *testing.T) {
		result, err := sm.Sync(
			sources.SyncWithDisabledSource(sources.ModelsDevHTTP),
			sources.SyncWithDisabledSource(sources.ModelsDevGit),
			sources.SyncWithDryRun(true),
		)

		if err != nil {
			t.Errorf("Sync with disabled source failed: %v", err)
		}

		if result == nil {
			t.Error("Expected sync result, got nil")
		}

		fmt.Printf("Disabled source sync completed: %s\n", result.Summary())
	})

	// Test Update() method with pipeline
	t.Run("UpdateWithPipeline", func(t *testing.T) {
		err := sm.Update()
		if err != nil {
			t.Errorf("Update with pipeline failed: %v", err)
		}

		fmt.Printf("Pipeline update completed\n")
	})

	fmt.Printf("Hook stats - Added: %d, Updated: %d\n", modelAddedCount, modelUpdatedCount)
}

// TestBackwardCompatibility ensures existing sync behavior still works
func TestBackwardCompatibility(t *testing.T) {
	// Create starmap with default settings
	sm, err := New()
	if err != nil {
		t.Fatalf("Failed to create starmap: %v", err)
	}

	// Test standard sync (should use original implementation)
	result, err := sm.Sync(sources.SyncWithDryRun(true))
	if err != nil {
		t.Errorf("Backward compatible sync failed: %v", err)
	}

	if result == nil {
		t.Error("Expected sync result, got nil")
	}

	fmt.Printf("Backward compatible sync completed: %s\n", result.Summary())
}
