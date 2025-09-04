package reconciler_test

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/agentstation/starmap/pkg/authority"
	"github.com/agentstation/starmap/pkg/catalogs"
	"github.com/agentstation/starmap/pkg/differ"
	"github.com/agentstation/starmap/pkg/provenance"
	"github.com/agentstation/starmap/pkg/reconciler"
	"github.com/agentstation/starmap/pkg/sources"
	"github.com/agentstation/utc"
)

// Helper function to create test models
func createTestModel(id, name string, contextWindow int64) catalogs.Model {
	return catalogs.Model{
		ID:   id,
		Name: name,
		Limits: &catalogs.ModelLimits{
			ContextWindow: contextWindow,
		},
		Metadata: &catalogs.ModelMetadata{
			ReleaseDate: utc.Now(),
		},
		CreatedAt: utc.Now(),
		UpdatedAt: utc.Now(),
	}
}

// Helper function to create test provider

// Helper function to add models to a catalog through a provider
func addTestModels(cat catalogs.Catalog, providerID string, models []catalogs.Model) error {
	provider, err := cat.Provider(catalogs.ProviderID(providerID))
	if err != nil {
		// Create provider if it doesn't exist
		provider = catalogs.Provider{
			ID:     catalogs.ProviderID(providerID),
			Name:   providerID,
			Models: make(map[string]catalogs.Model),
		}
	}
	if provider.Models == nil {
		provider.Models = make(map[string]catalogs.Model)
	}
	for _, model := range models {
		provider.Models[model.ID] = model
	}
	return cat.SetProvider(provider)
}

func TestReconcilerBasic(t *testing.T) {
	ctx := context.Background()

	// Create test catalogs
	catalog1, _ := catalogs.New()
	if err := addTestModels(catalog1, "test-provider", []catalogs.Model{
		createTestModel("gpt-4", "GPT-4", 8192),
		createTestModel("gpt-3.5", "GPT-3.5", 4096),
	}); err != nil {
		t.Fatalf("Failed to add models to catalog1: %v", err)
	}

	catalog2, _ := catalogs.New()
	if err := addTestModels(catalog2, "test-provider", []catalogs.Model{
		createTestModel("gpt-4", "GPT-4 Updated", 32768), // Updated context
		createTestModel("claude-3", "Claude 3", 100000),  // New model
	}); err != nil {
		t.Fatalf("Failed to add models to catalog2: %v", err)
	}

	// Create reconciler
	reconcile, err := reconciler.New()
	if err != nil {
		t.Fatalf("Failed to create reconciler: %v", err)
	}

	// Reconcile catalogs
	srcMap := map[sources.Type]catalogs.Catalog{
		"source1": catalog1,
		"source2": catalog2,
	}
	srcs := reconciler.ConvertCatalogsMapToSources(srcMap)

	result, err := reconcile.Sources(ctx, "source1", srcs)
	if err != nil {
		t.Fatalf("ReconcileCatalogs failed: %v", err)
	}

	// Verify result
	if result == nil {
		t.Fatal("Expected non-nil result")
	}

	if result.Catalog == nil {
		t.Fatal("Expected non-nil catalog in result")
	}

	// Check models were merged
	models := result.Catalog.GetAllModels()
	if len(models) != 3 {
		t.Errorf("Expected 3 models, got %d", len(models))
	}

	// Verify specific models
	gpt4, err := result.Catalog.FindModel("gpt-4")
	if err != nil {
		t.Error("Expected gpt-4 model to exist")
	}
	if gpt4.ID != "gpt-4" {
		t.Error("gpt-4 model not properly loaded")
	}

	claude, err := result.Catalog.FindModel("claude-3")
	if err != nil {
		t.Error("Expected claude-3 model to exist")
	}
	if claude.ID != "claude-3" {
		t.Error("claude-3 model not properly loaded")
	}
}

func TestDifferChangeDetection(t *testing.T) {
	// Create old catalog
	oldCatalog, _ := catalogs.New()
	if err := addTestModels(oldCatalog, "test-provider", []catalogs.Model{
		createTestModel("model-1", "Model 1", 1000),
		createTestModel("model-2", "Model 2", 2000),
	}); err != nil {
		t.Fatalf("Failed to add models to old catalog: %v", err)
	}

	// Create new catalog with changes
	newCatalog, _ := catalogs.New()
	if err := addTestModels(newCatalog, "test-provider", []catalogs.Model{
		createTestModel("model-1", "Model 1 Updated", 1500), // Updated
		createTestModel("model-3", "Model 3", 3000),         // Added
	}); err != nil {
		t.Fatalf("Failed to add models to new catalog: %v", err)
	}

	// Create differ
	diff := differ.New()

	// Detect changes
	changeset := diff.Catalogs(oldCatalog, newCatalog)

	// Verify changeset
	if changeset == nil {
		t.Fatal("Expected non-nil changeset")
	}

	if !changeset.HasChanges() {
		t.Error("Expected changes to be detected")
	}

	// Check specific changes
	if len(changeset.Models.Added) != 1 {
		t.Errorf("Expected 1 added model, got %d", len(changeset.Models.Added))
	}

	if len(changeset.Models.Removed) != 1 {
		t.Errorf("Expected 1 removed model, got %d", len(changeset.Models.Removed))
	}

	if len(changeset.Models.Updated) != 1 {
		t.Errorf("Expected 1 updated model, got %d", len(changeset.Models.Updated))
	}
}

func TestAuthorityBasedStrategy(t *testing.T) {
	// Create test authorities
	authorities := authority.New()

	// Create strategy
	strategy := reconciler.NewAuthorityStrategy(authorities)

	// Test conflict resolution
	values := map[sources.Type]any{
		sources.ProviderAPI:  "value1",
		sources.ModelsDevGit: "value2",
		sources.LocalCatalog: "value3",
	}

	value, source, reason := strategy.ResolveConflict("Pricing", values)

	// Should prefer ModelsDevGit or ModelsDevHTTP for pricing
	if source != sources.ModelsDevGit && source != sources.ModelsDevHTTP {
		t.Errorf("Expected ModelsDevGit or ModelsDevHTTP source for Pricing, got %s", source)
	}

	if value == nil {
		t.Error("Expected non-nil value")
	}

	if reason == "" {
		t.Error("Expected reason to be provided")
	}
}

func TestProvenanceTracking(t *testing.T) {
	// Create provenance tracker
	tracker := provenance.NewTracker(true)

	// Track some field changes
	tracker.Track(
		sources.ResourceTypeModel,
		"gpt-4",
		"pricing.input",
		provenance.Provenance{
			Source:    sources.ModelsDevHTTP,
			Field:     "pricing.input",
			Value:     0.01,
			Timestamp: time.Now(),
		},
	)

	// Retrieve provenance
	provenance := tracker.FindByField(sources.ResourceTypeModel, "gpt-4", "pricing.input")

	if len(provenance) == 0 {
		t.Error("Expected provenance to be tracked")
	}

	if provenance[0].Source != sources.ModelsDevHTTP {
		t.Errorf("Expected source to be ModelsDevHTTP, got %s", provenance[0].Source)
	}
}

func TestChangesetFiltering(t *testing.T) {
	// Create a changeset with various changes
	changeset := &differ.Changeset{
		Models: &differ.ModelChangeset{
			Added: []catalogs.Model{
				createTestModel("new-1", "New Model 1", 1000),
			},
			Updated: []differ.ModelUpdate{
				{
					ID:       "updated-1",
					Existing: createTestModel("updated-1", "Old", 500),
					New:      createTestModel("updated-1", "New", 1000),
				},
			},
			Removed: []catalogs.Model{
				createTestModel("removed-1", "Removed Model", 2000),
			},
		},
		Providers: &differ.ProviderChangeset{},
		Authors:   &differ.AuthorChangeset{},
	}

	// Calculate summary
	changeset.Summary = differ.ChangesetSummary{
		ModelsAdded:   1,
		ModelsUpdated: 1,
		ModelsRemoved: 1,
		TotalChanges:  3,
	}

	// Test filtering strategies
	tests := []struct {
		name        string
		strategy    differ.ApplyStrategy
		wantAdded   int
		wantUpdated int
		wantRemoved int
	}{
		{
			name:        "ApplyAll",
			strategy:    differ.ApplyAll,
			wantAdded:   1,
			wantUpdated: 1,
			wantRemoved: 1,
		},
		{
			name:        "ApplyAdditive",
			strategy:    differ.ApplyAdditive,
			wantAdded:   1,
			wantUpdated: 1,
			wantRemoved: 0,
		},
		{
			name:        "ApplyUpdatesOnly",
			strategy:    differ.ApplyUpdatesOnly,
			wantAdded:   0,
			wantUpdated: 1,
			wantRemoved: 0,
		},
		{
			name:        "ApplyAdditionsOnly",
			strategy:    differ.ApplyAdditionsOnly,
			wantAdded:   1,
			wantUpdated: 0,
			wantRemoved: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			filtered := changeset.Filter(tt.strategy)

			if len(filtered.Models.Added) != tt.wantAdded {
				t.Errorf("Expected %d added, got %d", tt.wantAdded, len(filtered.Models.Added))
			}

			if len(filtered.Models.Updated) != tt.wantUpdated {
				t.Errorf("Expected %d updated, got %d", tt.wantUpdated, len(filtered.Models.Updated))
			}

			if len(filtered.Models.Removed) != tt.wantRemoved {
				t.Errorf("Expected %d removed, got %d", tt.wantRemoved, len(filtered.Models.Removed))
			}
		})
	}
}

func TestResultBuilder(t *testing.T) {
	// Create a result using the builder
	catalog, _ := catalogs.New()
	changeset := &differ.Changeset{
		Models:    &differ.ModelChangeset{},
		Providers: &differ.ProviderChangeset{},
		Authors:   &differ.AuthorChangeset{},
	}

	result := reconciler.NewResult()
	result.Catalog = catalog
	result.Changeset = changeset
	result.Metadata.Sources = []sources.Type{sources.ProviderAPI, sources.ModelsDevGit}
	result.Metadata.Strategy = reconciler.NewAuthorityStrategy(authority.New())
	result.Metadata.DryRun = true
	result.Finalize()

	// Verify result
	if result == nil {
		t.Fatal("Expected non-nil result")
	}

	if !result.IsSuccess() {
		t.Error("Expected result to be successful")
	}

	if !result.Metadata.DryRun {
		t.Error("Expected dry run to be true")
	}

	if len(result.Metadata.Sources) != 2 {
		t.Errorf("Expected 2 sources, got %d", len(result.Metadata.Sources))
	}
}

// Benchmark tests
func BenchmarkReconciliation(b *testing.B) {
	ctx := context.Background()

	// Create large catalogs
	catalog1, _ := catalogs.New()
	catalog2, _ := catalogs.New()

	// Add many models to catalog1
	models1 := make([]catalogs.Model, 0, 1000)
	for i := 0; i < 1000; i++ {
		models1 = append(models1, createTestModel(
			fmt.Sprintf("model-%d", i),
			fmt.Sprintf("Model %d", i),
			int64(i*1000),
		))
	}
	if err := addTestModels(catalog1, "test-provider", models1); err != nil {
		b.Fatalf("Failed to add models to catalog1: %v", err)
	}

	// Add half the models to catalog2 with updates, plus new models
	models2 := make([]catalogs.Model, 0, 1000)
	for i := 0; i < 1000; i++ {
		if i%2 == 0 {
			// Half the models in second catalog with updates
			models2 = append(models2, createTestModel(
				fmt.Sprintf("model-%d", i),
				fmt.Sprintf("Model %d Updated", i),
				int64(i*1000),
			))
		}
	}

	// Add new models to second catalog
	for i := 1000; i < 1500; i++ {
		models2 = append(models2, createTestModel(
			fmt.Sprintf("model-%d", i),
			fmt.Sprintf("Model %d", i),
			int64(i*1000),
		))
	}
	if err := addTestModels(catalog2, "test-provider", models2); err != nil {
		b.Fatalf("Failed to add models to catalog2: %v", err)
	}

	reconcile, err := reconciler.New()
	if err != nil {
		b.Fatalf("Failed to create reconciler: %v", err)
	}
	srcMap := map[sources.Type]catalogs.Catalog{
		"source1": catalog1,
		"source2": catalog2,
	}
	srcs := reconciler.ConvertCatalogsMapToSources(srcMap)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := reconcile.Sources(ctx, "source1", srcs)
		if err != nil {
			b.Fatalf("ReconcileCatalogs failed: %v", err)
		}
	}
}

func BenchmarkDiffing(b *testing.B) {
	// Create large catalogs
	oldCatalog, _ := catalogs.New()
	newCatalog, _ := catalogs.New()

	// Add many models
	oldModels := make([]catalogs.Model, 0, 1000)
	newModels := make([]catalogs.Model, 0, 1000)
	for i := 0; i < 1000; i++ {
		model := createTestModel(
			fmt.Sprintf("model-%d", i),
			fmt.Sprintf("Model %d", i),
			int64(i*1000),
		)
		oldModels = append(oldModels, model)

		// Modify every other model
		if i%2 == 0 {
			model.Name = fmt.Sprintf("Model %d Updated", i)
		}
		newModels = append(newModels, model)
	}
	if err := addTestModels(oldCatalog, "test-provider", oldModels); err != nil {
		b.Fatalf("Failed to add models to old catalog: %v", err)
	}
	if err := addTestModels(newCatalog, "test-provider", newModels); err != nil {
		b.Fatalf("Failed to add models to new catalog: %v", err)
	}

	diff := differ.New()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = diff.Catalogs(oldCatalog, newCatalog)
	}
}
