package starmap

import (
	"testing"
	"time"

	"github.com/agentstation/starmap/internal/sources/registry"
	"github.com/agentstation/starmap/internal/sync"
	"github.com/agentstation/starmap/pkg/sources"

	// Import sources to trigger registration
	_ "github.com/agentstation/starmap/internal/sources"
)

func TestSourceAutoRegistration(t *testing.T) {
	// Test that all expected source types are auto-registered
	expectedSources := []sources.Type{
		sources.LocalCatalog,
		sources.ModelsDevGit,
		sources.ModelsDevHTTP,
		sources.ProviderAPI,
	}

	registeredSources := registry.RegisteredSourceTypes()

	for _, expected := range expectedSources {
		found := false
		for _, registered := range registeredSources {
			if registered == expected {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("Expected source type %s to be auto-registered, but it was not found", expected)
		}
	}

	// Test HasSource function
	for _, sourceType := range expectedSources {
		if !registry.HasSource(sourceType) {
			t.Errorf("Expected HasSource to return true for %s", sourceType)
		}
	}
}

func TestPipelineBuildBasicFunctionality(t *testing.T) {
	// Create a minimal context for testing
	sm, err := New(WithAutoUpdates(false))
	if err != nil {
		t.Fatalf("Failed to create starmap: %v", err)
	}

	catalog, err := sm.Catalog()
	if err != nil {
		t.Fatalf("Failed to get catalog: %v", err)
	}

	// Test building a pipeline with just catalog
	sourcePipeline, err := sync.Pipeline(catalog)
	if err != nil {
		t.Errorf("Failed to build pipeline: %v", err)
	}
	if sourcePipeline == nil {
		t.Error("Expected pipeline to be built, got nil")
	}

	// Test building with disabled local catalog (but other sources available)
	disabledOptions := sources.DefaultSyncOptions()
	disabledOptions.DisableLocalCatalog = true

	// This should still work because models.dev source is available by default
	// The test logic was incorrect - when local catalog is disabled, models.dev can still run
	pipeline2, err := sync.Pipeline(
		catalog,
		sync.WithSyncOptions(disabledOptions),
	)

	// Should not error - models.dev source should be available
	if err != nil {
		t.Errorf("Expected pipeline to build with models.dev source, got error: %v", err)
	}
	if pipeline2 == nil {
		t.Error("Expected pipeline to be built with models.dev source, got nil")
	}
}

// ===== Sync Options Tests =====

func TestSyncOptionsValidation(t *testing.T) {
	tests := []struct {
		name    string
		options *sources.SyncOptions
		wantErr bool
	}{
		{
			name:    "nil options",
			options: nil,
			wantErr: false,
		},
		{
			name: "valid options",
			options: &sources.SyncOptions{
				AutoApprove: true,
				Timeout:     30 * time.Second,
			},
			wantErr: false,
		},
		{
			name: "negative timeout",
			options: &sources.SyncOptions{
				Timeout: -5 * time.Second,
			},
			wantErr: true,
		},
		{
			name: "provenance file without tracking",
			options: &sources.SyncOptions{
				TrackProvenance: false,
				ProvenanceFile:  "/path/to/file",
			},
			wantErr: true,
		},
		{
			name: "valid provenance settings",
			options: &sources.SyncOptions{
				TrackProvenance: true,
				ProvenanceFile:  "/path/to/file",
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.options.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestSyncOptionsCopy(t *testing.T) {
	original := &sources.SyncOptions{
		AutoApprove: true,
		DryRun:      false,
		Timeout:     30 * time.Second,
		CustomFieldAuthorities: []sources.FieldAuthority{
			{FieldPath: "test", Source: sources.ProviderAPI, Priority: 100},
		},
	}

	copy := original.Copy()

	// Test that it's a proper copy
	if copy == nil {
		t.Fatal("Copy() returned nil")
	}

	if copy == original {
		t.Error("Copy() returned the same pointer")
	}

	// Test values are copied
	if copy.AutoApprove != original.AutoApprove {
		t.Error("AutoApprove not copied correctly")
	}

	if copy.Timeout != original.Timeout {
		t.Error("Timeout not copied correctly")
	}

	// Test deep copy of slice
	if len(copy.CustomFieldAuthorities) != len(original.CustomFieldAuthorities) {
		t.Error("CustomFieldAuthorities not copied correctly")
	}

	// Test that modifying copy doesn't affect original
	copy.AutoApprove = false
	if original.AutoApprove == false {
		t.Error("Modifying copy affected original")
	}
}

func TestStarmapSyncOptions(t *testing.T) {
	// Test setting and getting sync options
	sm, err := New(WithAutoUpdates(false))
	if err != nil {
		t.Fatalf("Failed to create starmap: %v", err)
	}

	// Test default state (no options)
	options := sm.GetSyncOptions()
	if options != nil {
		t.Error("Expected nil options by default")
	}

	// Test setting options
	customOptions := &sources.SyncOptions{
		AutoApprove: true,
		DryRun:      true,
		Timeout:     30 * time.Second,
	}

	sm.SetSyncOptions(customOptions)

	// Test getting options
	retrieved := sm.GetSyncOptions()
	if retrieved == nil {
		t.Fatal("Expected non-nil options after setting")
	}

	if !retrieved.AutoApprove {
		t.Error("AutoApprove setting not preserved")
	}

	if !retrieved.DryRun {
		t.Error("DryRun setting not preserved")
	}

	if retrieved.Timeout != 30*time.Second {
		t.Error("Timeout setting not preserved")
	}

	// Test that modifications to the original don't affect the stored options
	customOptions.DryRun = false
	retrievedAgain := sm.GetSyncOptions()
	if !retrievedAgain.DryRun {
		t.Error("Options should be isolated from external modifications")
	}
}

func TestStarmapWithSyncOptions(t *testing.T) {
	// Test creating starmap with sync options via WithSyncOptions
	options := &sources.SyncOptions{
		AutoApprove: true,
		DryRun:      false,
		Timeout:     45 * time.Second,
	}

	sm, err := New(
		WithAutoUpdates(false),
		WithSyncOptions(options),
	)
	if err != nil {
		t.Fatalf("Failed to create starmap with sync options: %v", err)
	}

	// Verify the options were applied
	retrieved := sm.GetSyncOptions()
	if retrieved == nil {
		t.Fatal("Expected options to be set from New()")
	}

	if !retrieved.AutoApprove {
		t.Error("AutoApprove setting not applied from New()")
	}

	if retrieved.Timeout != 45*time.Second {
		t.Error("Timeout setting not applied from New()")
	}
}
