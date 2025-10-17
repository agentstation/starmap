// Package reconciler provides catalog synchronization and reconciliation capabilities.
// It handles merging data from multiple sources, detecting changes, and applying
// updates while respecting data authorities and merge strategies.
package reconciler

import (
	"context"
	"maps"
	"time"

	"github.com/rs/zerolog"

	"github.com/agentstation/starmap/pkg/authority"
	"github.com/agentstation/starmap/pkg/catalogs"
	"github.com/agentstation/starmap/pkg/differ"
	"github.com/agentstation/starmap/pkg/enhancer"
	"github.com/agentstation/starmap/pkg/errors"
	"github.com/agentstation/starmap/pkg/logging"
	"github.com/agentstation/starmap/pkg/provenance"
	"github.com/agentstation/starmap/pkg/sources"
)

// Reconciler is the main interface for reconciling data from multiple sources.
type Reconciler interface {
	// Sources reconciles multiple catalogs from different sources
	// The primary source determines which models exist; other sources provide enrichment only
	Sources(ctx context.Context, primary sources.ID, srcs []sources.Source) (*Result, error)
}

// reconciler is the default implementation of Reconciler.
type reconciler struct {
	strategy    Strategy
	authorities authority.Authority
	provenance  provenance.Tracker
	tracking    bool
	enhancers   *enhancer.Pipeline
	baseline    catalogs.Catalog // Baseline catalog for comparison
}

// New creates a new Reconciler with options.
func New(opts ...Option) (Reconciler, error) {
	// Create options with defaults
	options, err := newOptions(opts...)
	if err != nil {
		return nil, err
	}

	// Create reconciler from options
	r := &reconciler{
		strategy:    options.strategy,
		authorities: options.authorities,
		provenance:  provenance.NewTracker(options.tracking),
		tracking:    options.tracking,
		enhancers:   enhancer.NewPipeline(options.enhancers...),
		baseline:    options.baseline,
	}
	return r, nil
}

// reconcileContext holds shared state for reconciliation.
type reconcileContext struct {
	collector *collector
	filter    *filter
	merger    Merger
	logger    *zerolog.Logger
	startTime time.Time
	baseline  catalogs.Catalog // Baseline for comparison
}

// modelResult holds reconciled models and provenance.
type modelResult struct {
	models     []*catalogs.Model
	provenance provenance.Map
	apiCount   int // Count from primary source
}

// Sources performs reconciliation with clean step-by-step flow.
func (r *reconciler) Sources(ctx context.Context, primary sources.ID, srcs []sources.Source) (*Result, error) {
	// Step 1: Initialize context and validate
	rctx, err := r.initialize(ctx, primary, srcs)
	if err != nil {
		return nil, err
	}

	// Step 2: Collect and merge providers
	providers, provenanceMap, err := r.reconcileProviders(rctx)
	if err != nil {
		return nil, err
	}

	// Step 3: Filter providers by primary source
	// Convert to values for filtering
	providerValues := make([]catalogs.Provider, len(providers))
	for i, p := range providers {
		if p != nil {
			providerValues[i] = *p
		}
	}
	filteredValues := rctx.filter.filterProviders(providerValues)
	// Convert back to pointers
	providers = make([]*catalogs.Provider, len(filteredValues))
	for i, p := range filteredValues {
		providers[i] = &p
	}
	rctx.logger.Info().
		Int("provider_count", len(providers)).
		Msg("Filtered providers by primary source")

	// Step 4: Reconcile models for each provider
	modelResults, err := r.reconcileAllModels(ctx, rctx, providers)
	if err != nil {
		return nil, err
	}

	// Step 5: Build catalog with providers and models
	catalog, err := r.catalog(providers, modelResults)
	if err != nil {
		return nil, err
	}

	// Step 6: Compute changeset if we have a base catalog
	changeset := r.changeset(rctx, catalog)

	// Step 7: Build and return result
	return r.result(rctx, catalog, changeset, provenanceMap, modelResults), nil
}

// initialize sets up reconciliation context.
func (r *reconciler) initialize(ctx context.Context, primary sources.ID, srcs []sources.Source) (*reconcileContext, error) {
	logger := logging.FromContext(ctx)

	// Create collector
	collector := newCollector(srcs, primary)

	// Validate and get primary catalog if specified
	var primaryCatalog catalogs.Catalog
	if primary != "" {
		primaryCatalog = collector.primaryCatalog()
		if primaryCatalog == nil {
			return nil, &errors.ValidationError{
				Field:   "primary",
				Value:   string(primary),
				Message: "specified primary source not found in sources list or has no catalog",
			}
		}
		logger.Debug().
			Str("primary_source", string(primary)).
			Msg("Using primary catalog for filtering")
	}

	// Create context
	return &reconcileContext{
		collector: collector,
		filter:    newFilter(primary, primaryCatalog),
		merger:    r.createMerger(),
		logger:    logger,
		startTime: time.Now(),
		baseline:  r.baseline,
	}, nil
}

// reconcileProviders merges providers from all sources.
func (r *reconciler) reconcileProviders(rctx *reconcileContext) ([]*catalogs.Provider, provenance.Map, error) {
	// Collect providers from all sources
	providerSources := rctx.collector.collectProviders()

	// Merge providers using configured strategy
	return rctx.merger.Providers(providerSources)
}

// reconcileAllModels processes models for all providers.
func (r *reconciler) reconcileAllModels(ctx context.Context, rctx *reconcileContext, providers []*catalogs.Provider) (map[catalogs.ProviderID]modelResult, error) {
	results := make(map[catalogs.ProviderID]modelResult)

	for _, provider := range providers {
		rctx.logger.Debug().
			Str("provider", string(provider.ID)).
			Msg("Reconciling models for provider")

		result, err := r.reconcileProviderModels(ctx, rctx, provider)
		if err != nil {
			// Log error but continue with other providers
			rctx.logger.Error().
				Err(err).
				Str("provider", string(provider.ID)).
				Msg("Failed to reconcile provider models")
			continue
		}

		results[provider.ID] = result
	}

	return results, nil
}

// reconcileProviderModels merges models for a single provider.
func (r *reconciler) reconcileProviderModels(ctx context.Context, rctx *reconcileContext, provider *catalogs.Provider) (modelResult, error) {
	// Collect models for this provider from all sources
	primaryCatalog := rctx.filter.primaryCatalog
	if primaryCatalog == nil {
		primaryCatalog = rctx.collector.primaryCatalog()
	}

	modelSources := rctx.collector.collectModelsForProvider(
		provider,
		primaryCatalog,
	)

	if len(modelSources) == 0 {
		return modelResult{}, nil
	}

	// Track API count from primary source
	apiCount := 0
	if rctx.collector.primary != "" {
		if models, exists := modelSources[rctx.collector.primary]; exists {
			apiCount = len(models)
		}
	}

	// Merge models using configured strategy
	models, prov, err := rctx.merger.Models(modelSources)
	if err != nil {
		return modelResult{}, err
	}

	// Apply enhancements if configured
	if r.enhancers != nil {
		enhanced, err := r.enhancers.Batch(ctx, models)
		if err != nil {
			rctx.logger.Warn().
				Err(err).
				Str("provider", string(provider.ID)).
				Msg("Enhancement failed but continuing")
		} else {
			models = enhanced
		}
	}

	return modelResult{
		models:     models,
		provenance: prov,
		apiCount:   apiCount,
	}, nil
}

// catalog creates the final catalog with providers and models.
func (r *reconciler) catalog(providers []*catalogs.Provider, modelResults map[catalogs.ProviderID]modelResult) (catalogs.Catalog, error) {
	var catalog catalogs.Catalog
	var err error

	// Start with baseline catalog if available to preserve existing providers
	if r.baseline != nil {
		catalog, err = r.baseline.Copy()
		if err != nil {
			return nil, errors.WrapResource("copy", "baseline catalog", "", err)
		}
	} else {
		catalog = catalogs.NewEmpty()
	}

	// Add/update providers with their reconciled models
	for i := range providers {
		provider := providers[i]

		// Add models if we have results for this provider
		if result, ok := modelResults[provider.ID]; ok && len(result.models) > 0 {
			provider.Models = make(map[string]*catalogs.Model)
			for _, model := range result.models {
				provider.Models[model.ID] = model
			}
		}

		// Set provider in catalog (this will overwrite existing provider)
		if err := catalog.Providers().Set(provider.ID, provider); err != nil {
			return nil, errors.WrapResource("set", "provider", string(provider.ID), err)
		}
	}

	return catalog, nil
}

// changeset calculates differences between baseline and new catalog.
func (r *reconciler) changeset(rctx *reconcileContext, catalog catalogs.Catalog) *differ.Changeset {
	// Use provided baseline if available, otherwise fall back to first source
	var baseCatalog catalogs.Catalog
	if rctx.baseline != nil {
		baseCatalog = rctx.baseline
		rctx.logger.Debug().Msg("Using provided baseline catalog for comparison")
	} else {
		// Fall back to first source catalog for backward compatibility
		baseCatalog = rctx.collector.baseCatalog()
		if baseCatalog == nil {
			return nil
		}
		rctx.logger.Debug().Msg("No baseline provided, using first source catalog")
	}

	// Collect all models from merged catalog
	var allMergedModels []*catalogs.Model
	for _, provider := range catalog.Providers().List() {
		if provider.Models != nil {
			for _, model := range provider.Models {
				allMergedModels = append(allMergedModels, model)
			}
		}
	}

	// Create differ and compute changes
	d := differ.New()

	// Convert base models to pointers for diff
	baseModelValues := baseCatalog.Models().List()
	baseModels := make([]*catalogs.Model, len(baseModelValues))
	for i, m := range baseModelValues {
		baseModels[i] = &m
	}

	modelChangeset := d.Models(
		baseModels,
		allMergedModels,
	)

	// Build changeset structure
	changeset := &differ.Changeset{
		Models: modelChangeset,
		Summary: differ.ChangesetSummary{
			ModelsAdded:   len(modelChangeset.Added),
			ModelsUpdated: len(modelChangeset.Updated),
			ModelsRemoved: len(modelChangeset.Removed),
			TotalChanges:  len(modelChangeset.Added) + len(modelChangeset.Updated) + len(modelChangeset.Removed),
		},
	}

	return changeset
}

// result creates the final result.
func (r *reconciler) result(rctx *reconcileContext, catalog catalogs.Catalog, changeset *differ.Changeset, providerProv provenance.Map, modelResults map[catalogs.ProviderID]modelResult) *Result {
	result := NewResult()

	// Set core data
	result.Catalog = catalog
	result.Changeset = changeset

	// Combine all provenance data
	maps.Copy(result.Provenance, providerProv)
	for _, mr := range modelResults {
		maps.Copy(result.Provenance, mr.provenance)
	}

	// Build tracking maps
	for providerID, mr := range modelResults {
		// Track API counts
		if mr.apiCount > 0 {
			result.ProviderAPICounts[providerID] = mr.apiCount
		}

		// Track model to provider mapping
		for _, model := range mr.models {
			result.ModelProviderMap[model.ID] = providerID
		}
	}

	// Set metadata
	result.Metadata.Sources = rctx.collector.sourceTypes()
	result.Metadata.Strategy = r.strategy

	// Calculate statistics
	result.Metadata.Stats = r.calcStats(catalog, modelResults)

	// Finalize result
	result.Finalize()

	return result
}

// createMerger creates a merger based on configuration.
func (r *reconciler) createMerger() Merger {
	if r.tracking && r.provenance != nil {
		return newMergerWithProvenance(r.authorities, r.strategy, r.provenance, r.baseline)
	}
	return newMerger(r.authorities, r.strategy, r.baseline)
}

// calcStats computes statistics from the catalog.
func (r *reconciler) calcStats(catalog catalogs.Catalog, modelResults map[catalogs.ProviderID]modelResult) ResultStatistics {
	var stats ResultStatistics

	// Count providers
	providers := catalog.Providers().List()
	stats.ProvidersProcessed = len(providers)

	// Count models
	for _, result := range modelResults {
		stats.ModelsProcessed += len(result.models)
	}

	return stats
}
