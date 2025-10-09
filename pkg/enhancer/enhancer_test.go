package enhancer

import (
	"context"
	"fmt"
	"testing"

	"github.com/agentstation/utc"

	"github.com/agentstation/starmap/pkg/catalogs"
	"github.com/agentstation/starmap/pkg/errors"
)

// TestEnhancer is a test implementation of the Enhancer interface.
type TestEnhancer struct {
	name       string
	priority   int
	enhance    func(*catalogs.Model) (*catalogs.Model, error)
	canEnhance func(*catalogs.Model) bool
}

func (e *TestEnhancer) Name() string  { return e.name }
func (e *TestEnhancer) Priority() int { return e.priority }
func (e *TestEnhancer) CanEnhance(model *catalogs.Model) bool {
	if e.canEnhance != nil {
		return e.canEnhance(model)
	}
	return true
}
func (e *TestEnhancer) Enhance(ctx context.Context, model *catalogs.Model) (*catalogs.Model, error) {
	if e.enhance != nil {
		return e.enhance(model)
	}
	return model, nil
}
func (e *TestEnhancer) EnhanceBatch(ctx context.Context, models []*catalogs.Model) ([]*catalogs.Model, error) {
	result := make([]*catalogs.Model, len(models))
	for i, model := range models {
		enhanced, err := e.Enhance(ctx, model)
		if err != nil {
			return nil, err
		}
		result[i] = enhanced
	}
	return result, nil
}

func TestEnhancerPipeline(t *testing.T) {
	ctx := context.Background()

	// Create test enhancers
	pricingEnhancer := &TestEnhancer{
		name:     "pricing",
		priority: 100,
		enhance: func(model *catalogs.Model) (*catalogs.Model, error) {
			if model.Pricing == nil {
				model.Pricing = &catalogs.ModelPricing{
					Tokens: &catalogs.ModelTokenPricing{
						Input: &catalogs.ModelTokenCost{
							Per1M: 10.0,
						},
						Output: &catalogs.ModelTokenCost{
							Per1M: 20.0,
						},
					},
				}
			}
			return model, nil
		},
	}

	limitsEnhancer := &TestEnhancer{
		name:     "limits",
		priority: 90,
		enhance: func(model *catalogs.Model) (*catalogs.Model, error) {
			if model.Limits == nil {
				model.Limits = &catalogs.ModelLimits{
					ContextWindow: 128000,
					OutputTokens:  4096,
				}
			}
			return model, nil
		},
	}

	metadataEnhancer := &TestEnhancer{
		name:     "metadata",
		priority: 80,
		enhance: func(model *catalogs.Model) (*catalogs.Model, error) {
			if model.Metadata == nil {
				model.Metadata = &catalogs.ModelMetadata{
					ReleaseDate: utc.Now(),
				}
			}
			return model, nil
		},
	}

	// Create pipeline - should be sorted by priority
	pipeline := NewPipeline(
		metadataEnhancer, // priority 80
		pricingEnhancer,  // priority 100
		limitsEnhancer,   // priority 90
	)

	// Test single model enhancement
	model := &catalogs.Model{
		ID:   "test-model",
		Name: "Test Model",
	}

	enhanced, err := pipeline.Enhance(ctx, model)
	if err != nil {
		t.Fatalf("Enhance failed: %v", err)
	}

	// Verify all enhancements were applied
	if enhanced.Pricing == nil {
		t.Error("Expected pricing to be added")
	}
	if enhanced.Limits == nil {
		t.Error("Expected limits to be added")
	}
	if enhanced.Metadata == nil {
		t.Error("Expected metadata to be added")
	}
}

func TestEnhancerPipelineWithErrors(t *testing.T) {
	ctx := context.Background()

	// Create an enhancer that fails
	failingEnhancer := &TestEnhancer{
		name:     "failing",
		priority: 100,
		enhance: func(model *catalogs.Model) (*catalogs.Model, error) {
			return model, &errors.SyncError{
				Provider: "failing",
				Err:      errors.New("enhancement failed"),
			}
		},
	}

	// Create an enhancer that succeeds
	successEnhancer := &TestEnhancer{
		name:     "success",
		priority: 90,
		enhance: func(model *catalogs.Model) (*catalogs.Model, error) {
			model.Description = "Enhanced"
			return model, nil
		},
	}

	pipeline := NewPipeline(failingEnhancer, successEnhancer)

	model := &catalogs.Model{
		ID:   "test-model",
		Name: "Test Model",
	}

	// Should continue despite failure
	enhanced, err := pipeline.Enhance(ctx, model)
	if err != nil {
		t.Fatalf("Pipeline should not fail on individual enhancer errors: %v", err)
	}

	// The successful enhancer should still run
	if enhanced.Description != "Enhanced" {
		t.Error("Expected successful enhancer to run despite earlier failure")
	}
}

func TestEnhancerBatch(t *testing.T) {
	ctx := context.Background()

	enhancer := &TestEnhancer{
		name:     "batch",
		priority: 100,
		enhance: func(model *catalogs.Model) (*catalogs.Model, error) {
			model.Description = "Batch Enhanced"
			return model, nil
		},
	}

	pipeline := NewPipeline(enhancer)

	models := []*catalogs.Model{
		{ID: "model1", Name: "Model 1"},
		{ID: "model2", Name: "Model 2"},
		{ID: "model3", Name: "Model 3"},
	}

	enhanced, err := pipeline.Batch(ctx, models)
	if err != nil {
		t.Fatalf("EnhanceBatch failed: %v", err)
	}

	if len(enhanced) != 3 {
		t.Errorf("Expected 3 models, got %d", len(enhanced))
	}

	for _, model := range enhanced {
		if model.Description != "Batch Enhanced" {
			t.Errorf("Expected model %s to be enhanced", model.ID)
		}
	}
}

func TestEnhancerCanEnhance(t *testing.T) {
	ctx := context.Background()

	// Create an enhancer that only enhances specific models
	selectiveEnhancer := &TestEnhancer{
		name:     "selective",
		priority: 100,
		enhance: func(model *catalogs.Model) (*catalogs.Model, error) {
			model.Description = "Selectively Enhanced"
			return model, nil
		},
		canEnhance: func(model *catalogs.Model) bool {
			return model.ID == "enhance-me"
		},
	}

	pipeline := NewPipeline(selectiveEnhancer)

	models := []*catalogs.Model{
		{ID: "enhance-me", Name: "Should be enhanced"},
		{ID: "skip-me", Name: "Should not be enhanced"},
	}

	enhanced, err := pipeline.Batch(ctx, models)
	if err != nil {
		t.Fatalf("EnhanceBatch failed: %v", err)
	}

	if enhanced[0].Description != "Selectively Enhanced" {
		t.Error("Expected first model to be enhanced")
	}

	if enhanced[1].Description == "Selectively Enhanced" {
		t.Error("Expected second model NOT to be enhanced")
	}
}

func TestChainEnhancer(t *testing.T) {
	ctx := context.Background()

	// Create enhancers that will be chained
	enhancer1 := &TestEnhancer{
		name:     "first",
		priority: 100,
		enhance: func(model *catalogs.Model) (*catalogs.Model, error) {
			model.Description = "First"
			return model, nil
		},
	}

	enhancer2 := &TestEnhancer{
		name:     "second",
		priority: 90,
		enhance: func(model *catalogs.Model) (*catalogs.Model, error) {
			model.Description += " -> Second"
			return model, nil
		},
	}

	// Create chain
	chain := NewChainEnhancer(100, enhancer1, enhancer2)

	model := &catalogs.Model{
		ID:   "test",
		Name: "Test",
	}

	enhanced, err := chain.Enhance(ctx, model)
	if err != nil {
		t.Fatalf("Chain enhance failed: %v", err)
	}

	if enhanced.Description != "First -> Second" {
		t.Errorf("Expected chained enhancements, got %q", enhanced.Description)
	}
}

func BenchmarkEnhancerPipeline(b *testing.B) {
	ctx := context.Background()

	// Create test enhancers
	enhancers := []Enhancer{}
	for i := 0; i < 5; i++ {
		enhancer := &TestEnhancer{
			name:     fmt.Sprintf("enhancer-%d", i),
			priority: 100 - i*10,
			enhance: func(model *catalogs.Model) (*catalogs.Model, error) {
				// Simulate some work
				if model.Metadata == nil {
					model.Metadata = &catalogs.ModelMetadata{}
				}
				return model, nil
			},
		}
		enhancers = append(enhancers, enhancer)
	}

	pipeline := NewPipeline(enhancers...)

	// Create test models
	models := make([]*catalogs.Model, 100)
	for i := 0; i < 100; i++ {
		models[i] = &catalogs.Model{
			ID:   fmt.Sprintf("model-%d", i),
			Name: fmt.Sprintf("Model %d", i),
		}
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := pipeline.Batch(ctx, models)
		if err != nil {
			b.Fatalf("EnhanceBatch failed: %v", err)
		}
	}
}
