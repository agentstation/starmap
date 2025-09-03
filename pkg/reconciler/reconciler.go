// package reconciler provides catalog synchronization and reconciliation capabilities.
// It handles merging data from multiple sources, detecting changes, and applying
// updates while respecting data authorities and merge strategies.
//
// The reconciler coordinates fetching data from various sources, computing differences,
// and merging changes into a target catalog. It supports dry-run operations,
// changeset generation, and intelligent conflict resolution.
//
// Example usage:
//
//	// Create a reconciler
//	r := NewReconciler(targetCatalog, sources)
//
//	// Perform reconciliation
//	changeset, err := r.Reconcile(ctx, options)
//	if err != nil {
//	    log.Fatal(err)
//	}
//
//	// Review changes
//	for _, change := range changeset.Changes {
//	    fmt.Printf("Change: %s %s\n", change.Type, change.ModelID)
//	}
//
//	// Apply changes if not dry-run
//	if !options.DryRun {
//	    err = r.Apply(changeset)
//	}
package reconciler

import (
	"context"
	"time"

	"github.com/agentstation/starmap/pkg/authority"
	"github.com/agentstation/starmap/pkg/catalogs"
	"github.com/agentstation/starmap/pkg/differ"
	"github.com/agentstation/starmap/pkg/enhancer"
	"github.com/agentstation/starmap/pkg/errors"
	"github.com/agentstation/starmap/pkg/logging"
	"github.com/agentstation/starmap/pkg/provenance"
	"github.com/agentstation/starmap/pkg/sources"
)

// Reconciler is the main interface for reconciling data from multiple sources
type Reconciler interface {
	// Sources reconciles multiple catalogs from different sources
	// The primary source determines which models exist; other sources provide enrichment only
	Sources(ctx context.Context, primary sources.Type, srcs []sources.Source) (*Result, error)
}

// reconciler is the default implementation of Reconciler
type reconciler struct {
	strategy    Strategy
	authorities authority.Authority
	provenance  provenance.Tracker
	tracking    bool
	enhancers   *enhancer.Pipeline
}

// New creates a new Reconciler with options
func New(opts ...Option) (Reconciler, error) {
	// Create options with defaults
	options := NewOptions(opts...)

	// Create reconciler from options
	r := &reconciler{
		strategy:    options.strategy,
		authorities: options.authorities,
		provenance:  provenance.NewTracker(options.tracking),
		tracking:    options.tracking,
		enhancers:   enhancer.NewPipeline(options.enhancers...),
	}
	return r, nil
}

// Sources reconciles multiple catalogs from different sources
func (r *reconciler) Sources(ctx context.Context, primary sources.Type, srcs []sources.Source) (*Result, error) {

	// Start timer & get logger
	startTime := time.Now()
	logger := logging.FromContext(ctx)

	// Create result builder
	sourceNames := make([]sources.Type, 0, len(srcs))
	for _, src := range srcs {
		sourceNames = append(sourceNames, src.Type())
	}
	resultBuilder := NewResultBuilder().WithSources(sourceNames...).WithStrategy(r.strategy)

	// Cache primary catalog early if specified
	var primaryCatalog catalogs.Catalog
	if primary != "" {
		for _, src := range srcs {
			if src.Type() == primary {
				primaryCatalog = src.Catalog()
				break
			}
		}
		// Validate primary source was found
		if primaryCatalog == nil {
			return nil, &errors.ValidationError{
				Field:   "primary",
				Value:   string(primary),
				Message: "specified primary source not found in sources list or has no catalog",
			}
		}
		logger.Debug().
			Str("primary_source", string(primary)).
			Msg("Cached primary catalog for reconciliation")
	}

	// Create merged catalog
	mergedCatalog, err := catalogs.New()
	if err != nil {
		return nil, errors.WrapResource("create", "merged catalog", "", err)
	}

	// Initialize tracking maps
	providerAPICounts := make(map[catalogs.ProviderID]int)
	modelProviderMap := make(map[string]catalogs.ProviderID)

	// First, extract and reconcile providers
	providerSources := make(map[sources.Type][]catalogs.Provider)
	for _, src := range srcs {
		catalog := src.Catalog()
		if catalog == nil {
			continue
		}
		providers := catalog.Providers().List()
		providerList := make([]catalogs.Provider, 0, len(providers))
		for _, p := range providers {
			providerList = append(providerList, *p)
		}
		providerSources[src.Type()] = providerList
	}

	// Reconcile providers first
	mergedProviders, providerProvenance, err := r.mergeProviders(providerSources)
	if err != nil {
		return nil, &errors.SyncError{
			Provider: "reconciler",
			Err:      err,
		}
	}

	// Filter providers by primary source if specified
	if primary != "" && primaryCatalog != nil {
		originalCount := len(mergedProviders)
		filteredProviders := make([]catalogs.Provider, 0)

		for _, provider := range mergedProviders {
			// Check if provider exists in primary source
			primaryProvider := r.getPrimaryProvider(primaryCatalog, provider.ID, provider.Aliases)
			if primaryProvider != nil {
				filteredProviders = append(filteredProviders, provider)
			}
		}

		logger.Info().
			Int("original_count", originalCount).
			Int("filtered_count", len(filteredProviders)).
			Str("primary_source", string(primary)).
			Msg("Filtered providers by primary source")

		mergedProviders = filteredProviders
	}

	// Now reconcile models per provider to maintain provider-specific attributes
	allModelProvenance := make(provenance.Map)
	for i, provider := range mergedProviders {
		// Collect models for this provider from all sources, grouped by source type
		// We need this grouping for the merger to apply authority/priority rules
		providerModels := make(map[sources.Type][]catalogs.Model)

		logger.Debug().
			Str("provider", string(provider.ID)).
			Int("source_count", len(srcs)).
			Msg("Processing provider for model collection")

		for _, src := range srcs {
			// Get catalog from this source
			catalog := src.Catalog()
			if catalog == nil {
				continue
			}
			sourceName := src.Type()

			// Get provider from this source
			sourceProvider, exists := catalog.Providers().Get(provider.ID)

			// Debug logging
			if exists || sourceProvider != nil {
				modelCount := 0
				if sourceProvider.Models != nil {
					modelCount = len(sourceProvider.Models)
				}
				logger.Debug().
					Str("source", string(sourceName)).
					Str("provider", string(provider.ID)).
					Int("models_in_provider", modelCount).
					Bool("exists", exists).
					Msg("Checking provider models from source")
			} else {
				logger.Debug().
					Str("source", string(sourceName)).
					Str("provider", string(provider.ID)).
					Bool("exists", exists).
					Msg("Provider not found in source")
			}

			if !exists {
				// Check if any alias matches
				for _, alias := range provider.Aliases {
					if aliasProvider, aliasExists := catalog.Providers().Get(alias); aliasExists {
						sourceProvider = aliasProvider
						break
					}
				}
			}

			// Collect models associated with this provider (or its aliases)
			var modelsForProvider []catalogs.Model

			// If we found the provider, use its models
			if sourceProvider != nil && sourceProvider.Models != nil {
				for _, model := range sourceProvider.Models {
					modelsForProvider = append(modelsForProvider, model)
				}
			}

			// For non-primary sources (enrichment sources), only add models that the primary source serves
			// This ensures enrichment sources don't add models that don't exist in the provider API
			if primary != "" && sourceName != primary {
				// Get all models from this source for potential enrichment
				allModels := catalog.GetAllModels()
				logger.Debug().
					Str("source", string(sourceName)).
					Str("provider", string(provider.ID)).
					Int("potential_models", len(allModels)).
					Msg("Filtering non-primary source models by primary authority")

				// But only add models that this provider actually serves according to the primary source
				for _, model := range allModels {
					// Check if we already have this model from the provider
					shouldInclude := false
					for _, existingModel := range modelsForProvider {
						if existingModel.ID == model.ID {
							shouldInclude = true
							break
						}
					}
					// If provider doesn't have this model yet, check if the primary source has it
					if !shouldInclude && primary != "" && primaryCatalog != nil {
						// Use cached primary catalog and helper function
						primaryProvider := r.getPrimaryProvider(primaryCatalog, provider.ID, provider.Aliases)
						if r.primaryHasModel(primaryProvider, model.ID) {
							modelsForProvider = append(modelsForProvider, model)
							shouldInclude = true
						}
					}
				}
			}

			if len(modelsForProvider) > 0 {
				providerModels[sourceName] = modelsForProvider

				// Track API counts for primary source immediately
				if sourceName == primary && providerAPICounts[provider.ID] == 0 {
					providerAPICounts[provider.ID] = len(modelsForProvider)
					logger.Debug().
						Str("provider", string(provider.ID)).
						Int("api_model_count", len(modelsForProvider)).
						Msg("Tracked primary source model count")
				}
			}
		}

		// Reconcile models for this provider
		if len(providerModels) > 0 {
			reconciledModels, modelProvenance, err := r.mergeModels(providerModels)
			if err != nil {
				resultBuilder.WithError(&errors.SyncError{
					Provider: string(provider.ID),
					Err:      err,
				})
				continue
			}

			// Add reconciled models to the provider
			if mergedProviders[i].Models == nil {
				mergedProviders[i].Models = make(map[string]catalogs.Model)
			}
			for _, model := range reconciledModels {
				mergedProviders[i].Models[model.ID] = model
				// Track which provider owns this model
				modelProviderMap[model.ID] = provider.ID
			}

			// Merge model provenance
			for k, v := range modelProvenance {
				allModelProvenance[k] = v
			}
		}
	}

	// Add merged providers with their models to catalog
	for _, provider := range mergedProviders {
		// Use Set instead of Add to ensure upsert semantics
		if err := mergedCatalog.Providers().Set(provider.ID, &provider); err != nil {
			resultBuilder.WithError(errors.WrapResource("set", "provider", string(provider.ID), err))
		}
	}

	// Combine provenance
	allProvenance := make(provenance.Map)
	for k, v := range allModelProvenance {
		allProvenance[k] = v
	}
	for k, v := range providerProvenance {
		allProvenance[k] = v
	}

	// Count total models processed
	totalModels := 0
	for _, provider := range mergedProviders {
		if provider.Models != nil {
			totalModels += len(provider.Models)
		}
	}

	// Build statistics
	stats := ResultStatistics{
		ModelsProcessed:    totalModels,
		ProvidersProcessed: len(mergedProviders),
		TotalTimeMs:        time.Since(startTime).Milliseconds(),
	}

	// Create a changeset if we have a base catalog to compare against
	var changeset *differ.Changeset
	if len(srcs) > 0 {
		// Use the first source as the base for comparison
		// In practice, you might want to be more sophisticated about this
		var baseCatalog catalogs.Catalog
		for _, src := range srcs {
			if src.Catalog() != nil {
				baseCatalog = src.Catalog()
				break
			}
		}

		// Collect all merged models for diff
		var allMergedModels []catalogs.Model
		for _, provider := range mergedProviders {
			if provider.Models != nil {
				for _, model := range provider.Models {
					allMergedModels = append(allMergedModels, model)
				}
			}
		}

		// Create differ to detect changes
		diff := differ.New()
		modelChangeset := diff.Models(
			catalogModelsToSlice(baseCatalog),
			allMergedModels,
		)

		// Build proper changeset structure
		changeset = &differ.Changeset{
			Models: modelChangeset,
			Summary: differ.ChangesetSummary{
				ModelsAdded:   len(modelChangeset.Added),
				ModelsUpdated: len(modelChangeset.Updated),
				ModelsRemoved: len(modelChangeset.Removed),
			},
		}
	}

	// Build and return result
	result := resultBuilder.
		WithCatalog(mergedCatalog).
		WithProvenance(allProvenance).
		WithStatistics(stats).
		WithChangeset(changeset).
		Build()

	// Add the tracking maps
	result.ProviderAPICounts = providerAPICounts
	result.ModelProviderMap = modelProviderMap

	return result, nil
}

// getPrimaryProvider looks up a provider in the primary catalog by ID or aliases
func (r *reconciler) getPrimaryProvider(
	primaryCatalog catalogs.Catalog,
	providerID catalogs.ProviderID,
	aliases []catalogs.ProviderID,
) *catalogs.Provider {
	if primaryCatalog == nil {
		return nil
	}

	// Check main ID
	if provider, exists := primaryCatalog.Providers().Get(providerID); exists {
		return provider
	}

	// Check aliases
	for _, alias := range aliases {
		if provider, exists := primaryCatalog.Providers().Get(alias); exists {
			return provider
		}
	}

	return nil
}

// primaryHasModel checks if the primary provider serves a specific model
func (r *reconciler) primaryHasModel(
	primaryProvider *catalogs.Provider,
	modelID string,
) bool {
	if primaryProvider == nil || primaryProvider.Models == nil {
		return false
	}
	_, exists := primaryProvider.Models[modelID]
	return exists
}

// mergeModels merges models from multiple sources
func (r *reconciler) mergeModels(sources map[sources.Type][]catalogs.Model) ([]catalogs.Model, provenance.Map, error) {
	return r.mergeModelsWithContext(context.Background(), sources)
}

// mergeModelsWithContext merges models from multiple sources with context support
func (r *reconciler) mergeModelsWithContext(ctx context.Context, sources map[sources.Type][]catalogs.Model) ([]catalogs.Model, provenance.Map, error) {
	// Use the merger to combine models
	var merger Merger
	if r.tracking && r.provenance != nil {
		merger = newMergerWithProvenance(r.authorities, r.strategy, r.provenance)
	} else {
		merger = newMerger(r.authorities, r.strategy)
	}

	// Merge models from sources
	mergedModels, provenance, err := merger.Models(sources)
	if err != nil {
		return nil, nil, err
	}

	// Apply enhancers if configured
	if r.enhancers != nil {
		enhanced, err := r.enhancers.Batch(ctx, mergedModels)
		if err != nil {
			// Log but don't fail - enhancement is optional
			logging.Warn().
				Err(err).
				Msg("Enhancement failed but continuing")
		} else {
			mergedModels = enhanced
		}
	}

	return mergedModels, provenance, nil
}

// mergeProviders merges providers from multiple sources
func (r *reconciler) mergeProviders(srcs map[sources.Type][]catalogs.Provider) ([]catalogs.Provider, provenance.Map, error) {
	// Use the merger to combine providers
	var merger Merger
	if r.tracking && r.provenance != nil {
		merger = newMergerWithProvenance(r.authorities, r.strategy, r.provenance)
	} else {
		merger = newMerger(r.authorities, r.strategy)
	}

	return merger.Providers(srcs)
}

// catalogModelsToSlice converts catalog models to a slice
func catalogModelsToSlice(catalog catalogs.Reader) []catalogs.Model {
	return catalog.GetAllModels()
}
