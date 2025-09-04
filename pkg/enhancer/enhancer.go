package enhancer

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/agentstation/starmap/pkg/catalogs"
	"github.com/agentstation/starmap/pkg/errors"
	"github.com/agentstation/starmap/pkg/logging"
	"github.com/agentstation/starmap/pkg/provenance"
	"github.com/agentstation/starmap/pkg/sources"
)

// Enhancer defines the interface for model enrichment
type Enhancer interface {
	// Name returns the enhancer name
	Name() string

	// Enhance enhances a single model with additional data
	Enhance(ctx context.Context, model catalogs.Model) (catalogs.Model, error)

	// EnhanceBatch enhances multiple models efficiently
	EnhanceBatch(ctx context.Context, models []catalogs.Model) ([]catalogs.Model, error)

	// CanEnhance checks if this enhancer can enhance a specific model
	CanEnhance(model catalogs.Model) bool

	// Priority returns the priority of this enhancer (higher = applied first)
	Priority() int
}

// Pipeline manages a chain of enhancers
type Pipeline struct {
	enhancers []Enhancer
	tracker   provenance.Tracker
}

// NewPipeline creates a new enhancer pipeline
func NewPipeline(enhancers ...Enhancer) *Pipeline {
	// Sort enhancers by priority (highest first)
	sortedEnhancers := make([]Enhancer, len(enhancers))
	copy(sortedEnhancers, enhancers)

	for i := 0; i < len(sortedEnhancers)-1; i++ {
		for j := i + 1; j < len(sortedEnhancers); j++ {
			if sortedEnhancers[j].Priority() > sortedEnhancers[i].Priority() {
				sortedEnhancers[i], sortedEnhancers[j] = sortedEnhancers[j], sortedEnhancers[i]
			}
		}
	}

	return &Pipeline{
		enhancers: sortedEnhancers,
	}
}

// WithProvenance enables provenance tracking for enhancements
func (p *Pipeline) WithProvenance(tracker provenance.Tracker) *Pipeline {
	p.tracker = tracker
	return p
}

// Enhance applies all enhancers to a single model
func (p *Pipeline) Enhance(ctx context.Context, model catalogs.Model) (catalogs.Model, error) {
	enhanced := model

	for _, enhancer := range p.enhancers {
		if !enhancer.CanEnhance(enhanced) {
			continue
		}

		result, err := enhancer.Enhance(ctx, enhanced)
		if err != nil {
			// Log error but continue with other enhancers
			logging.Warn().
				Err(err).
				Str("enhancer", enhancer.Name()).
				Str("model_id", model.ID).
				Msg("Enhancer failed for model")
			continue
		}

		// Track provenance if enabled
		if p.tracker != nil {
			p.track(enhanced, result, enhancer)
		}

		enhanced = result
	}

	return enhanced, nil
}

// Batch applies all enhancers to multiple models
func (p *Pipeline) Batch(ctx context.Context, models []catalogs.Model) ([]catalogs.Model, error) {
	enhanced := make([]catalogs.Model, len(models))

	for _, enhancer := range p.enhancers {
		// Filter models that can be enhanced
		toEnhance := []catalogs.Model{}
		indices := []int{}

		for i, model := range models {
			if enhancer.CanEnhance(model) {
				toEnhance = append(toEnhance, model)
				indices = append(indices, i)
			}
		}

		if len(toEnhance) == 0 {
			continue
		}

		// Batch enhance
		results, err := enhancer.EnhanceBatch(ctx, toEnhance)
		if err != nil {
			// Fallback to individual enhancement
			for i, model := range toEnhance {
				result, err := enhancer.Enhance(ctx, model)
				if err != nil {
					logging.Warn().
						Err(err).
						Str("enhancer", enhancer.Name()).
						Str("model_id", model.ID).
						Msg("Enhancer failed for model in batch")
					results[i] = model // Keep original on failure
				} else {
					results[i] = result
				}
			}
		}

		// Update enhanced models
		for i, idx := range indices {
			if p.tracker != nil {
				p.track(models[idx], results[i], enhancer)
			}
			models[idx] = results[i]
		}
	}

	copy(enhanced, models)
	return enhanced, nil
}

// track tracks provenance for model enhancements
func (p *Pipeline) track(original, enhanced catalogs.Model, enhancer Enhancer) {
	// Track which fields were enhanced
	// This is a simplified version - could be made more sophisticated
	if original.Pricing == nil && enhanced.Pricing != nil {
		p.tracker.Track(
			sources.ResourceTypeModel,
			enhanced.ID,
			"pricing",
			provenance.Provenance{
				Source:    sources.Type(enhancer.Name()),
				Field:     "pricing",
				Value:     enhanced.Pricing,
				Timestamp: utcNow(),
			},
		)
	}

	if original.Limits == nil && enhanced.Limits != nil {
		p.tracker.Track(
			sources.ResourceTypeModel,
			enhanced.ID,
			"limits",
			provenance.Provenance{
				Source:    sources.Type(enhancer.Name()),
				Field:     "limits",
				Value:     enhanced.Limits,
				Timestamp: utcNow(),
			},
		)
	}

	if original.Metadata == nil && enhanced.Metadata != nil {
		p.tracker.Track(
			sources.ResourceTypeModel,
			enhanced.ID,
			"metadata",
			provenance.Provenance{
				Source:    sources.Type(enhancer.Name()),
				Field:     "metadata",
				Value:     enhanced.Metadata,
				Timestamp: utcNow(),
			},
		)
	}
}

// ModelsDevEnhancer enhances models with models.dev data
type ModelsDevEnhancer struct {
	priority int
	data     map[string]any // Simplified for example
}

// NewModelsDevEnhancer creates a new models.dev enhancer
func NewModelsDevEnhancer(priority int) *ModelsDevEnhancer {
	return &ModelsDevEnhancer{
		priority: priority,
		data:     make(map[string]any),
	}
}

// Name returns the enhancer name
func (e *ModelsDevEnhancer) Name() string {
	return "models.dev"
}

// Priority returns the priority
func (e *ModelsDevEnhancer) Priority() int {
	return e.priority
}

// CanEnhance checks if this enhancer can enhance a model
func (e *ModelsDevEnhancer) CanEnhance(model catalogs.Model) bool {
	// Check if we have data for this model
	_, exists := e.data[model.ID]
	return exists
}

// Enhance enhances a single model
func (e *ModelsDevEnhancer) Enhance(ctx context.Context, model catalogs.Model) (catalogs.Model, error) {
	// This would integrate with the actual models.dev data
	// For now, it's a placeholder implementation
	enhanced := model

	// Example: enhance pricing if missing
	if enhanced.Pricing == nil {
		if data, exists := e.data[model.ID]; exists {
			// Extract pricing from models.dev data
			if pricing, ok := data.(map[string]any)["pricing"]; ok {
				// Convert and apply pricing
				_ = pricing // Placeholder
			}
		}
	}

	return enhanced, nil
}

// EnhanceBatch enhances multiple models
func (e *ModelsDevEnhancer) EnhanceBatch(ctx context.Context, models []catalogs.Model) ([]catalogs.Model, error) {
	enhanced := make([]catalogs.Model, len(models))
	for i, model := range models {
		result, err := e.Enhance(ctx, model)
		if err != nil {
			return nil, errors.WrapResource("enhance", "model", model.ID, err)
		}
		enhanced[i] = result
	}
	return enhanced, nil
}

// MetadataEnhancer adds metadata from various sources
type MetadataEnhancer struct {
	priority int
}

// NewMetadataEnhancer creates a new metadata enhancer
func NewMetadataEnhancer(priority int) *MetadataEnhancer {
	return &MetadataEnhancer{
		priority: priority,
	}
}

// Name returns the enhancer name
func (e *MetadataEnhancer) Name() string {
	return "metadata"
}

// Priority returns the priority
func (e *MetadataEnhancer) Priority() int {
	return e.priority
}

// CanEnhance checks if this enhancer can enhance a model
func (e *MetadataEnhancer) CanEnhance(model catalogs.Model) bool {
	// Can enhance if metadata is missing or incomplete
	return model.Metadata == nil || model.Metadata.ReleaseDate.IsZero()
}

// Enhance enhances a single model
func (e *MetadataEnhancer) Enhance(ctx context.Context, model catalogs.Model) (catalogs.Model, error) {
	enhanced := model

	if enhanced.Metadata == nil {
		enhanced.Metadata = &catalogs.ModelMetadata{}
	}

	// Add metadata based on model patterns
	// This is a simplified example
	if enhanced.Metadata.ReleaseDate.IsZero() {
		// Could look up release dates from a database
	}

	return enhanced, nil
}

// EnhanceBatch enhances multiple models
func (e *MetadataEnhancer) EnhanceBatch(ctx context.Context, models []catalogs.Model) ([]catalogs.Model, error) {
	// Could fetch metadata for all models in one query
	enhanced := make([]catalogs.Model, len(models))
	for i, model := range models {
		result, err := e.Enhance(ctx, model)
		if err != nil {
			return nil, errors.WrapResource("enhance", "model", model.ID, err)
		}
		enhanced[i] = result
	}
	return enhanced, nil
}

// ChainEnhancer allows chaining multiple enhancers with custom logic
type ChainEnhancer struct {
	enhancers []Enhancer
	priority  int
}

// NewChainEnhancer creates a new chain enhancer
func NewChainEnhancer(priority int, enhancers ...Enhancer) *ChainEnhancer {
	return &ChainEnhancer{
		enhancers: enhancers,
		priority:  priority,
	}
}

// Name returns the enhancer name
func (e *ChainEnhancer) Name() string {
	names := []string{}
	for _, enhancer := range e.enhancers {
		names = append(names, enhancer.Name())
	}
	return fmt.Sprintf("chain(%s)", strings.Join(names, ","))
}

// Priority returns the priority
func (e *ChainEnhancer) Priority() int {
	return e.priority
}

// CanEnhance checks if any enhancer in the chain can enhance
func (e *ChainEnhancer) CanEnhance(model catalogs.Model) bool {
	for _, enhancer := range e.enhancers {
		if enhancer.CanEnhance(model) {
			return true
		}
	}
	return false
}

// Enhance applies all enhancers in sequence
func (e *ChainEnhancer) Enhance(ctx context.Context, model catalogs.Model) (catalogs.Model, error) {
	enhanced := model
	for _, enhancer := range e.enhancers {
		if enhancer.CanEnhance(enhanced) {
			result, err := enhancer.Enhance(ctx, enhanced)
			if err != nil {
				return enhanced, &errors.SyncError{
					Provider: enhancer.Name(),
					Err:      err,
				}
			}
			enhanced = result
		}
	}
	return enhanced, nil
}

// Batch enhances multiple models
func (e *ChainEnhancer) Batch(ctx context.Context, models []catalogs.Model) ([]catalogs.Model, error) {
	enhanced := make([]catalogs.Model, len(models))
	copy(enhanced, models)

	for _, enhancer := range e.enhancers {
		result, err := enhancer.EnhanceBatch(ctx, enhanced)
		if err != nil {
			return enhanced, &errors.SyncError{
				Provider: enhancer.Name(),
				Err:      err,
			}
		}
		enhanced = result
	}

	return enhanced, nil
}

// Helper function to get current UTC time
func utcNow() time.Time {
	return time.Now().UTC()
}
