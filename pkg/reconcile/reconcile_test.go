package reconcile_test

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/agentstation/starmap/pkg/catalogs"
	"github.com/agentstation/starmap/pkg/reconcile"
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
func createTestProvider(id, name string) catalogs.Provider {
	return catalogs.Provider{
		ID:   catalogs.ProviderID(id),
		Name: name,
	}
}

func TestReconcilerBasic(t *testing.T) {
	ctx := context.Background()
	
	// Create test catalogs
	catalog1, _ := catalogs.New()
	catalog1.Models().Add(&[]catalogs.Model{
		createTestModel("gpt-4", "GPT-4", 8192),
		createTestModel("gpt-3.5", "GPT-3.5", 4096),
	}[0])
	catalog1.Models().Add(&[]catalogs.Model{
		createTestModel("gpt-4", "GPT-4", 8192),
		createTestModel("gpt-3.5", "GPT-3.5", 4096),
	}[1])

	catalog2, _ := catalogs.New()
	catalog2.Models().Add(&[]catalogs.Model{
		createTestModel("gpt-4", "GPT-4 Updated", 32768), // Updated context
		createTestModel("claude-3", "Claude 3", 100000),  // New model
	}[0])
	catalog2.Models().Add(&[]catalogs.Model{
		createTestModel("gpt-4", "GPT-4 Updated", 32768),
		createTestModel("claude-3", "Claude 3", 100000),
	}[1])

	// Create reconciler
	r, err := reconcile.New()
	if err != nil {
		t.Fatalf("Failed to create reconciler: %v", err)
	}

	// Reconcile catalogs
	sources := map[reconcile.SourceName]catalogs.Catalog{
		"source1": catalog1,
		"source2": catalog2,
	}

	result, err := r.ReconcileCatalogs(ctx, "source1", sources)
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
	models := result.Catalog.Models()
	if models.Len() != 3 {
		t.Errorf("Expected 3 models, got %d", models.Len())
	}

	// Verify specific models
	gpt4, exists := models.Get("gpt-4")
	if !exists || gpt4 == nil {
		t.Error("Expected gpt-4 model to exist")
	}

	claude, exists := models.Get("claude-3")
	if !exists || claude == nil {
		t.Error("Expected claude-3 model to exist")
	}
}

func TestDifferChangeDetection(t *testing.T) {
	// Create old catalog
	oldCatalog, _ := catalogs.New()
	oldCatalog.Models().Add(&[]catalogs.Model{
		createTestModel("model-1", "Model 1", 1000),
		createTestModel("model-2", "Model 2", 2000),
	}[0])
	oldCatalog.Models().Add(&[]catalogs.Model{
		createTestModel("model-1", "Model 1", 1000),
		createTestModel("model-2", "Model 2", 2000),
	}[1])

	// Create new catalog with changes
	newCatalog, _ := catalogs.New()
	newCatalog.Models().Add(&[]catalogs.Model{
		createTestModel("model-1", "Model 1 Updated", 1500), // Updated
		createTestModel("model-3", "Model 3", 3000),         // Added
	}[0])
	newCatalog.Models().Add(&[]catalogs.Model{
		createTestModel("model-1", "Model 1 Updated", 1500),
		createTestModel("model-3", "Model 3", 3000),
	}[1])

	// Create differ
	differ := reconcile.NewDiffer()

	// Detect changes
	changeset := differ.DiffCatalogs(oldCatalog, newCatalog)

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
	authorities := reconcile.NewDefaultAuthorityProvider()
	
	// Create strategy
	strategy := reconcile.NewAuthorityBasedStrategy(authorities)

	// Test conflict resolution
	values := map[reconcile.SourceName]interface{}{
		reconcile.ProviderAPI:  "value1",
		reconcile.ModelsDevGit: "value2",
		reconcile.LocalCatalog: "value3",
	}

	value, source, reason := strategy.ResolveConflict("pricing.input", values)

	// Should prefer ModelsDevGit for pricing
	if source != reconcile.ModelsDevGit && source != reconcile.ModelsDevHTTP {
		t.Errorf("Expected ModelsDevGit or ModelsDevHTTP source for pricing.input, got %s", source)
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
	tracker := reconcile.NewProvenanceTracker(true)

	// Track some field changes
	tracker.Track(
		reconcile.ResourceTypeModel,
		"gpt-4",
		"pricing.input",
		reconcile.ProvenanceInfo{
			Source:    reconcile.ModelsDevHTTP,
			Field:     "pricing.input",
			Value:     0.01,
			Timestamp: time.Now(),
		},
	)

	// Retrieve provenance
	provenance := tracker.GetProvenance(reconcile.ResourceTypeModel, "gpt-4", "pricing.input")

	if len(provenance) == 0 {
		t.Error("Expected provenance to be tracked")
	}

	if provenance[0].Source != reconcile.ModelsDevHTTP {
		t.Errorf("Expected source to be ModelsDevHTTP, got %s", provenance[0].Source)
	}
}

func TestMergerWithProvenance(t *testing.T) {
	// Create merger with provenance tracking
	authorities := reconcile.NewDefaultAuthorityProvider()
	strategy := reconcile.NewAuthorityBasedStrategy(authorities)
	tracker := reconcile.NewProvenanceTracker(true)
	merger := reconcile.NewStrategicMerger(authorities, strategy)
	merger.WithProvenance(tracker)

	// Create test models from different sources
	sources := map[reconcile.SourceName][]catalogs.Model{
		reconcile.ProviderAPI: {
			createTestModel("model-1", "API Model", 1000),
		},
		reconcile.ModelsDevGit: {
			createTestModel("model-1", "ModelsDiv Model", 1500),
		},
	}

	// Merge models
	merged, provenance, err := merger.MergeModels(sources)
	if err != nil {
		t.Fatalf("MergeModels failed: %v", err)
	}

	if len(merged) != 1 {
		t.Errorf("Expected 1 merged model, got %d", len(merged))
	}

	if len(provenance) == 0 {
		t.Error("Expected provenance to be tracked")
	}
}

func TestChangesetFiltering(t *testing.T) {
	// Create a changeset with various changes
	changeset := &reconcile.Changeset{
		Models: &reconcile.ModelChangeset{
			Added: []catalogs.Model{
				createTestModel("new-1", "New Model 1", 1000),
			},
			Updated: []reconcile.ModelUpdate{
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
		Providers: &reconcile.ProviderChangeset{},
		Authors:   &reconcile.AuthorChangeset{},
	}

	// Calculate summary
	changeset.Summary = reconcile.ChangesetSummary{
		ModelsAdded:   1,
		ModelsUpdated: 1,
		ModelsRemoved: 1,
		TotalChanges:  3,
	}

	// Test filtering strategies
	tests := []struct {
		name     string
		strategy reconcile.ApplyStrategy
		wantAdded   int
		wantUpdated int
		wantRemoved int
	}{
		{
			name:        "ApplyAll",
			strategy:    reconcile.ApplyAll,
			wantAdded:   1,
			wantUpdated: 1,
			wantRemoved: 1,
		},
		{
			name:        "ApplyAdditive",
			strategy:    reconcile.ApplyAdditive,
			wantAdded:   1,
			wantUpdated: 1,
			wantRemoved: 0,
		},
		{
			name:        "ApplyUpdatesOnly",
			strategy:    reconcile.ApplyUpdatesOnly,
			wantAdded:   0,
			wantUpdated: 1,
			wantRemoved: 0,
		},
		{
			name:        "ApplyAdditionsOnly",
			strategy:    reconcile.ApplyAdditionsOnly,
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
	changeset := &reconcile.Changeset{
		Models:    &reconcile.ModelChangeset{},
		Providers: &reconcile.ProviderChangeset{},
		Authors:   &reconcile.AuthorChangeset{},
	}

	result := reconcile.NewResultBuilder().
		WithCatalog(catalog).
		WithChangeset(changeset).
		WithSources(reconcile.ProviderAPI, reconcile.ModelsDevGit).
		WithStrategy(reconcile.NewAuthorityBasedStrategy(reconcile.NewDefaultAuthorityProvider())).
		WithDryRun(true).
		Build()

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

func TestStrategyChain(t *testing.T) {
	// Create a chain of strategies
	authorities := reconcile.NewDefaultAuthorityProvider()
	chain := reconcile.NewStrategyChain(
		reconcile.NewAuthorityBasedStrategy(authorities),
		reconcile.NewUnionStrategy(),
	)

	// Test that it tries strategies in order
	values := map[reconcile.SourceName]interface{}{
		reconcile.ProviderAPI: "value1",
	}

	value, source, reason := chain.ResolveConflict("some.field", values)

	if value == nil {
		t.Error("Expected chain to resolve to a value")
	}

	if source == "" {
		t.Error("Expected chain to provide a source")
	}

	if reason == "" {
		t.Error("Expected chain to provide a reason")
	}
}

// Benchmark tests
func BenchmarkReconciliation(b *testing.B) {
	ctx := context.Background()
	
	// Create large catalogs
	catalog1, _ := catalogs.New()
	catalog2, _ := catalogs.New()
	
	// Add many models
	for i := 0; i < 1000; i++ {
		model := createTestModel(
			fmt.Sprintf("model-%d", i),
			fmt.Sprintf("Model %d", i),
			int64(i*1000),
		)
		catalog1.Models().Add(&model)
		
		if i%2 == 0 {
			// Half the models in second catalog with updates
			model.Name = fmt.Sprintf("Model %d Updated", i)
			catalog2.Models().Add(&model)
		}
	}
	
	// Add new models to second catalog
	for i := 1000; i < 1500; i++ {
		model := createTestModel(
			fmt.Sprintf("model-%d", i),
			fmt.Sprintf("Model %d", i),
			int64(i*1000),
		)
		catalog2.Models().Add(&model)
	}

	r, err := reconcile.New()
	if err != nil {
		b.Fatalf("Failed to create reconciler: %v", err)
	}
	sources := map[reconcile.SourceName]catalogs.Catalog{
		"source1": catalog1,
		"source2": catalog2,
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := r.ReconcileCatalogs(ctx, "source1", sources)
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
	for i := 0; i < 1000; i++ {
		model := createTestModel(
			fmt.Sprintf("model-%d", i),
			fmt.Sprintf("Model %d", i),
			int64(i*1000),
		)
		oldCatalog.Models().Add(&model)
		
		// Modify every other model
		if i%2 == 0 {
			model.Name = fmt.Sprintf("Model %d Updated", i)
		}
		newCatalog.Models().Add(&model)
	}

	differ := reconcile.NewDiffer()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = differ.DiffCatalogs(oldCatalog, newCatalog)
	}
}