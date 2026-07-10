// Package reconciler provides catalog synchronization and reconciliation capabilities.
// It handles merging data from multiple sources, detecting changes, and applying
// updates while respecting data authorities and merge strategies.
package reconciler

import (
	"context"
	"maps"
	"strings"
	"time"

	"github.com/rs/zerolog"

	"github.com/agentstation/starmap/internal/attribution"
	"github.com/agentstation/starmap/pkg/authority"
	"github.com/agentstation/starmap/pkg/catalogs"
	"github.com/agentstation/starmap/pkg/differ"
	"github.com/agentstation/starmap/pkg/enhancer"
	"github.com/agentstation/starmap/pkg/errors"
	"github.com/agentstation/starmap/pkg/logging"
	"github.com/agentstation/starmap/pkg/provenance"
	"github.com/agentstation/starmap/pkg/sources"
)

// Reconciler combines data from multiple sources into a canonical catalog.
// It is concrete because this package has one reconciliation engine; extension
// points are accepted through the narrow Strategy, Authority, Source, and
// Enhancer interfaces.
type Reconciler struct {
	strategy    Strategy
	authorities authority.Authority
	provenance  provenance.Tracker
	tracking    bool
	enhancers   *enhancer.Pipeline
	baseline    *catalogs.Catalog // Baseline catalog for comparison
}

// New creates a new Reconciler with options.
func New(opts ...Option) (*Reconciler, error) {
	// Create options with defaults
	options, err := newOptions(opts...)
	if err != nil {
		return nil, err
	}

	// Create reconciler from options
	r := &Reconciler{
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
	merger    *merger
	logger    *zerolog.Logger
	startTime time.Time
	baseline  *catalogs.Catalog // Baseline for comparison
}

// modelResult holds reconciled models and provenance.
type modelResult struct {
	models     []*catalogs.Model
	provenance provenance.Map
	apiCount   int // Count from primary source
}

// Sources performs reconciliation with clean step-by-step flow.
func (r *Reconciler) Sources(ctx context.Context, primary sources.ID, srcs []sources.Observation) (*Result, error) {
	// Step 1: Initialize context and validate
	rctx, err := r.initialize(ctx, primary, srcs)
	if err != nil {
		return nil, err
	}

	// Step 2: Collect and merge providers
	providers, err := r.reconcileProviders(rctx)
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

	// Step 5.5: Apply author attributions using the fresh catalog
	// This populates author.Models based on attribution config from authors.yaml
	// Attribution now uses the fresh catalog with API-fetched models instead of baseline
	if err := attribution.Apply(catalog); err != nil {
		rctx.logger.Warn().Err(err).Msg("Failed to apply author attributions")
		// Non-fatal - continue with reconciliation
	}

	// Step 6: Compute changeset if we have a base catalog
	changeset := r.changeset(rctx, catalog)

	// Step 7: Build and return result
	return r.result(rctx, catalog, changeset, modelResults), nil
}

// initialize sets up reconciliation context.
func (r *Reconciler) initialize(ctx context.Context, primary sources.ID, srcs []sources.Observation) (*reconcileContext, error) {
	logger := logging.FromContext(ctx)

	// Create collector
	collector := newCollector(srcs, primary)

	// Validate and get primary catalog if specified
	var primaryCatalog *catalogs.Catalog
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

	merger := r.createMerger()
	merger.setObservations(srcs)

	// Create context
	return &reconcileContext{
		collector: collector,
		filter:    newFilter(primary, primaryCatalog),
		merger:    merger,
		logger:    logger,
		startTime: time.Now(),
		baseline:  r.baseline,
	}, nil
}

// reconcileProviders merges providers from all sources.
func (r *Reconciler) reconcileProviders(rctx *reconcileContext) ([]*catalogs.Provider, error) {
	// Collect providers from all sources
	providerSources := rctx.collector.collectProviders()

	// Merge providers using configured strategy
	return rctx.merger.Providers(providerSources)
}

// reconcileAllModels processes models for all providers.
func (r *Reconciler) reconcileAllModels(ctx context.Context, rctx *reconcileContext, providers []*catalogs.Provider) (map[catalogs.ProviderID]modelResult, error) {
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
func (r *Reconciler) reconcileProviderModels(ctx context.Context, rctx *reconcileContext, provider *catalogs.Provider) (modelResult, error) {
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
	models, prov, err := rctx.merger.ModelsForProvider(provider.ID, modelSources)
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
func (r *Reconciler) catalog(providers []*catalogs.Provider, modelResults map[catalogs.ProviderID]modelResult) (*catalogs.Builder, error) {
	var catalog *catalogs.Builder
	var err error

	// Start with baseline catalog if available to preserve existing providers
	if r.baseline != nil {
		catalog, err = catalogs.NewBuilderFrom(r.baseline)
		if err != nil {
			return nil, errors.WrapResource("copy", "baseline catalog", "", err)
		}
		// Clear old provenance data - we'll regenerate it from the current reconciliation
		catalog.ClearProvenance()
	} else {
		catalog = catalogs.NewEmpty()
	}

	// Add/update providers with their reconciled models
	for i := range providers {
		provider := providers[i]

		// Add models if we have results for this provider
		if result, ok := modelResults[provider.ID]; ok && len(result.models) > 0 {
			provider.Models = r.providerModels(result.models)
		}

		// Set provider in catalog (this will overwrite existing provider)
		if err := catalog.SetProvider(*provider); err != nil {
			return nil, errors.WrapResource("set", "provider", string(provider.ID), err)
		}
	}

	return catalog, nil
}

func (r *Reconciler) providerModels(models []*catalogs.Model) map[string]*catalogs.Model {
	providerModels := make(map[string]*catalogs.Model)
	for _, model := range models {
		if model == nil {
			continue
		}
		providerModels[model.ID] = model
	}
	return providerModels
}

// changeset calculates differences between baseline and new catalog.
func (r *Reconciler) changeset(rctx *reconcileContext, catalog *catalogs.Builder) *differ.Changeset {
	// Use provided baseline if available, otherwise fall back to first source
	var baseCatalog *catalogs.Catalog
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

	return differ.New().Catalogs(baseCatalog, catalog)
}

// result creates the final result.
func (r *Reconciler) result(rctx *reconcileContext, catalog *catalogs.Builder, changeset *differ.Changeset, modelResults map[catalogs.ProviderID]modelResult) *Result {
	result := NewResult()

	// Set core data
	result.Catalog = catalog
	result.Changeset = changeset

	// Combine all provenance data (models only). Scope merge-local provenance
	// keys by provider so shared model IDs from different providers cannot
	// overwrite one another in the flat result map.
	for providerID, mr := range modelResults {
		maps.Copy(result.Provenance, providerScopedProvenance(providerID, mr.provenance))
	}

	// Copy tracker provenance into catalog for persistence
	// This ensures the catalog contains the correctly-formatted provenance data
	if r.provenance != nil {
		catalog.MergeProvenance(r.provenance.Map())
	}

	// Build tracking maps from final catalog (not just current sync results)
	// This ensures all models in the catalog have provider mappings, including
	// those from the baseline that weren't re-fetched in this sync
	for _, provider := range catalog.Providers().List() {
		if provider.Models != nil {
			for modelID := range provider.Models {
				result.ModelProviderMap[modelID] = provider.ID
			}
		}
	}

	// Track API counts from modelResults
	for providerID, mr := range modelResults {
		if mr.apiCount > 0 {
			result.ProviderAPICounts[providerID] = mr.apiCount
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

func providerScopedProvenance(providerID catalogs.ProviderID, source provenance.Map) provenance.Map {
	scoped := make(provenance.Map, len(source))
	prefix := "models."
	scopedPrefix := "models." + string(providerID) + "."
	for key, entries := range source {
		if rest, ok := strings.CutPrefix(key, prefix); ok && providerID != "" {
			scoped[scopedPrefix+rest] = entries
			continue
		}
		scoped[key] = entries
	}
	return scoped
}

// createMerger creates a merger based on configuration.
func (r *Reconciler) createMerger() *merger {
	if r.tracking && r.provenance != nil {
		return newMergerWithProvenance(r.authorities, r.strategy, r.provenance, r.baseline)
	}
	return newMerger(r.authorities, r.strategy, r.baseline)
}

// calcStats computes statistics from the catalog.
func (r *Reconciler) calcStats(catalog catalogs.Reader, modelResults map[catalogs.ProviderID]modelResult) ResultStatistics {
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
