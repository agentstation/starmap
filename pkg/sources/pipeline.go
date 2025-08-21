package sources

import (
	"context"
	"fmt"
	"log"
	"sort"
	"time"

	"github.com/agentstation/starmap/pkg/catalogs"
)

// Pipeline manages an ordered list of sources and executes them to merge data
type Pipeline struct {
	sources      []Source
	fieldMerger  *FieldMerger
	validateFunc func(catalogs.Model) error
	logger       Logger
}

// Logger interface for pipeline logging
type Logger interface {
	Printf(format string, v ...interface{})
}

// DefaultLogger is a simple logger implementation
type DefaultLogger struct{}

func (dl DefaultLogger) Printf(format string, v ...interface{}) {
	log.Printf(format, v...)
}

// PipelineResult contains the results of executing the source pipeline
type PipelineResult struct {
	ProviderID catalogs.ProviderID
	Models     []catalogs.Model
	Provider   *catalogs.Provider
	Provenance map[string]Provenance

	// Source statistics
	SourceStats map[Type]SourceStats

	// Execution metadata
	ExecutedAt time.Time
	Duration   time.Duration
}

// SourceStats contains statistics about a source's contribution
type SourceStats struct {
	Available      bool
	ModelsReturned int
	ProviderData   bool
	Errors         []error
}

// NewSourcePipeline creates a new source pipeline with the given sources
func NewPipeline(sources ...Source) *Pipeline {
	// Sort sources by priority (highest first)
	sort.Slice(sources, func(i, j int) bool {
		return sources[i].Priority() > sources[j].Priority()
	})

	return &Pipeline{
		sources:     sources,
		fieldMerger: NewFieldMerger().WithProvenance(true),
		logger:      DefaultLogger{},
	}
}

// WithValidation adds a validation function for merged models
func (sp *Pipeline) WithValidation(fn func(catalogs.Model) error) *Pipeline {
	sp.validateFunc = fn
	return sp
}

// WithLogger sets a custom logger for the pipeline
func (sp *Pipeline) WithLogger(logger Logger) *Pipeline {
	sp.logger = logger
	return sp
}

// WithCustomAuthorities sets custom field authorities on the field merger
func (sp *Pipeline) WithCustomAuthorities(modelAuth, providerAuth []FieldAuthority) *Pipeline {
	sp.fieldMerger = NewFieldMerger().WithAuthorities(modelAuth, providerAuth).WithProvenance(true)
	return sp
}

// Execute runs the pipeline for a specific provider
func (sp *Pipeline) Execute(ctx context.Context, providerID catalogs.ProviderID) (*PipelineResult, error) {
	startTime := time.Now()

	sp.logger.Printf("Executing pipeline for provider %s with %d sources", providerID, len(sp.sources))

	result := &PipelineResult{
		ProviderID:  providerID,
		SourceStats: make(map[Type]SourceStats),
		ExecutedAt:  startTime,
	}

	// Collect data from all sources
	sourceModels := make(map[Type][]catalogs.Model)
	sourceProviders := make(map[Type]*catalogs.Provider)

	// Execute each source
	for _, source := range sp.sources {
		stats := SourceStats{
			Available: source.IsAvailable(),
		}

		if !stats.Available {
			sp.logger.Printf("Source %s (%s) is not available", source.Name(), source.Type())
			result.SourceStats[source.Type()] = stats
			continue
		}

		// Fetch models from this source
		models, err := source.FetchModels(ctx, providerID)
		if err != nil {
			sp.logger.Printf("Error fetching models from source %s: %v", source.Name(), err)
			stats.Errors = append(stats.Errors, err)
		} else if models != nil {
			sourceModels[source.Type()] = models
			stats.ModelsReturned = len(models)
			sp.logger.Printf("Source %s returned %d models", source.Name(), len(models))
		}

		// Fetch provider info from this source
		provider, err := source.FetchProvider(ctx, providerID)
		if err != nil {
			sp.logger.Printf("Error fetching provider from source %s: %v", source.Name(), err)
			stats.Errors = append(stats.Errors, err)
		} else if provider != nil {
			sourceProviders[source.Type()] = provider
			stats.ProviderData = true
			sp.logger.Printf("Source %s returned provider data", source.Name())
		}

		result.SourceStats[source.Type()] = stats
	}

	// Merge models field-by-field
	if len(sourceModels) > 0 {
		mergedModels, modelProvenance := sp.fieldMerger.MergeModels(sourceModels)
		result.Models = mergedModels
		result.Provenance = modelProvenance

		sp.logger.Printf("Merged %d models from %d sources", len(mergedModels), len(sourceModels))

		// Validate merged models if validation function is set
		if sp.validateFunc != nil {
			validModels := make([]catalogs.Model, 0, len(result.Models))
			for _, model := range result.Models {
				if err := sp.validateFunc(model); err != nil {
					sp.logger.Printf("Validation failed for model %s: %v", model.ID, err)
				} else {
					validModels = append(validModels, model)
				}
			}
			result.Models = validModels
		}
	}

	// Merge provider info
	if len(sourceProviders) > 0 {
		mergedProvider, providerProvenance := sp.fieldMerger.MergeProvider(sourceProviders, providerID)
		result.Provider = mergedProvider

		// Merge provider provenance into main provenance map
		if result.Provenance == nil {
			result.Provenance = make(map[string]Provenance)
		}
		for field, prov := range providerProvenance {
			result.Provenance[fmt.Sprintf("provider.%s", field)] = prov
		}

		sp.logger.Printf("Merged provider data from %d sources", len(sourceProviders))
	}

	result.Duration = time.Since(startTime)
	sp.logger.Printf("Pipeline execution completed in %v", result.Duration)

	return result, nil
}

// GetSources returns a copy of the sources in the pipeline
func (sp *Pipeline) GetSources() []Source {
	sources := make([]Source, len(sp.sources))
	copy(sources, sp.sources)
	return sources
}

// AddSource adds a source to the pipeline and re-sorts by priority
func (sp *Pipeline) AddSource(source Source) {
	sp.sources = append(sp.sources, source)

	// Re-sort by priority
	sort.Slice(sp.sources, func(i, j int) bool {
		return sp.sources[i].Priority() > sp.sources[j].Priority()
	})
}

// RemoveSource removes a source from the pipeline by type
func (sp *Pipeline) RemoveSource(sourceType Type) bool {
	for i, source := range sp.sources {
		if source.Type() == sourceType {
			sp.sources = append(sp.sources[:i], sp.sources[i+1:]...)
			return true
		}
	}
	return false
}

// HasSource returns true if the pipeline has a source of the given type
func (sp *Pipeline) HasSource(sourceType Type) bool {
	for _, source := range sp.sources {
		if source.Type() == sourceType {
			return true
		}
	}
	return false
}

// GetSourceByType returns the source of the given type, if it exists
func (sp *Pipeline) GetSourceByType(sourceType Type) Source {
	for _, source := range sp.sources {
		if source.Type() == sourceType {
			return source
		}
	}
	return nil
}

// Summary returns a human-readable summary of the pipeline result
func (pr *PipelineResult) Summary() string {
	available := 0
	total := len(pr.SourceStats)
	totalModels := len(pr.Models)

	for _, stats := range pr.SourceStats {
		if stats.Available {
			available++
		}
	}

	hasProvider := ""
	if pr.Provider != nil {
		hasProvider = ", provider data merged"
	}

	return fmt.Sprintf("%d models from %d/%d sources%s (took %v)",
		totalModels, available, total, hasProvider, pr.Duration)
}
