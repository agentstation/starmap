// Package reconcile provides catalog synchronization and reconciliation capabilities.
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
package reconcile

import (
	"context"
	"time"

	"github.com/agentstation/starmap/pkg/catalogs"
	"github.com/agentstation/starmap/pkg/errors"
	"github.com/agentstation/starmap/pkg/logging"
)

// SourceName represents the name/type of a data source
type SourceName string

// String returns the string representation of a source name
func (sn SourceName) String() string {
	return string(sn)
}

// Common source names
const (
	ProviderAPI   SourceName = "Provider APIs"
	ModelsDevGit  SourceName = "models.dev (git)"
	ModelsDevHTTP SourceName = "models.dev (http)"
	LocalCatalog  SourceName = "Local Catalog"
)

// ResourceType identifies the type of resource being merged
type ResourceType string

const (
	ResourceTypeModel    ResourceType = "model"
	ResourceTypeProvider ResourceType = "provider"
	ResourceTypeAuthor   ResourceType = "author"
)

// Reconciler is the main interface for reconciling data from multiple sources
type Reconciler interface {
	// ReconcileCatalogs merges multiple catalogs from different sources
	// The primary source determines which models exist; other sources provide enrichment only
	ReconcileCatalogs(ctx context.Context, primary SourceName, sources map[SourceName]catalogs.Catalog) (*Result, error)

	// ReconcileModels merges models from multiple sources
	ReconcileModels(sources map[SourceName][]catalogs.Model) ([]catalogs.Model, ProvenanceMap, error)

	// ReconcileProviders merges providers from multiple sources
	ReconcileProviders(sources map[SourceName][]catalogs.Provider) ([]catalogs.Provider, ProvenanceMap, error)
}

// reconciler is the default implementation of Reconciler
type reconciler struct {
	strategy    Strategy
	authorities AuthorityProvider
	provenance  ProvenanceTracker
	tracking    bool
	enhancers   *EnhancerPipeline
}

// Option configures a Reconciler
type Option func(*reconciler) error

// New creates a new Reconciler with options
func New(opts ...Option) (Reconciler, error) {
	r := &reconciler{
		strategy:    NewAuthorityBasedStrategy(NewDefaultAuthorityProvider()),
		authorities: NewDefaultAuthorityProvider(),
		provenance:  NewProvenanceTracker(false),
		tracking:    false,
		enhancers:   NewEnhancerPipeline(),
	}
	
	for _, opt := range opts {
		if err := opt(r); err != nil {
			return nil, err
		}
	}
	
	return r, nil
}

// ReconcileCatalogs merges multiple catalogs from different sources
func (r *reconciler) ReconcileCatalogs(ctx context.Context, primary SourceName, sources map[SourceName]catalogs.Catalog) (*Result, error) {
	startTime := time.Now()
	
	// Create result builder
	resultBuilder := NewResultBuilder().
		WithSources(getSourceNames(sources)...).
		WithStrategy(r.strategy)

	// Create merged catalog
	mergedCatalog, err := catalogs.New()
	if err != nil {
		return nil, errors.WrapResource("create", "merged catalog", "", err)
	}

	// First, extract and reconcile providers
	providerSources := make(map[SourceName][]catalogs.Provider)
	for sourceName, catalog := range sources {
		providers := catalog.Providers().List()
		providerList := make([]catalogs.Provider, 0, len(providers))
		for _, p := range providers {
			providerList = append(providerList, *p)
		}
		providerSources[sourceName] = providerList
	}

	// Reconcile providers first
	mergedProviders, providerProvenance, err := r.ReconcileProviders(providerSources)
	if err != nil {
		return nil, &errors.SyncError{
			Provider: "reconciler",
			Err:      err,
		}
	}

	// Now reconcile models per provider to maintain provider-specific attributes
	allModelProvenance := make(ProvenanceMap)
	
	// Process each provider's models separately
	logger := logging.FromContext(ctx)
	for i, provider := range mergedProviders {
		// Collect models for this provider from all sources
		providerModels := make(map[SourceName][]catalogs.Model)
		
		logger.Debug().
			Str("provider", string(provider.ID)).
			Int("source_count", len(sources)).
			Msg("Processing provider for model collection")
		
		for sourceName, catalog := range sources {
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
				allModels := catalog.Models().List()
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
					if !shouldInclude && primary != "" {
						// Check if the primary source serves this model for this provider
						if primaryCatalog, hasPrimary := sources[primary]; hasPrimary {
							primaryProvider, exists := primaryCatalog.Providers().Get(provider.ID)
							if !exists {
								// Check aliases in primary source
								for _, alias := range provider.Aliases {
									if aliasProvider, aliasExists := primaryCatalog.Providers().Get(alias); aliasExists {
										primaryProvider = aliasProvider
										break
									}
								}
							}
							if primaryProvider != nil && primaryProvider.Models != nil {
								if _, hasModel := primaryProvider.Models[model.ID]; hasModel {
									modelsForProvider = append(modelsForProvider, *model)
									shouldInclude = true
								}
							}
						}
					}
				}
			}
			
			if len(modelsForProvider) > 0 {
				providerModels[sourceName] = modelsForProvider
			}
		}
		
		// Reconcile models for this provider
		if len(providerModels) > 0 {
			reconciledModels, modelProvenance, err := r.ReconcileModels(providerModels)
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
				// Also add to the catalog's model collection
				if err := mergedCatalog.Models().Add(&model); err != nil {
					resultBuilder.WithError(errors.WrapResource("set", "model", model.ID, err))
				}
			}
			
			// Merge model provenance
			for k, v := range modelProvenance {
				allModelProvenance[k] = v
			}
		}
	}

	// Add merged providers with their models to catalog
	for _, provider := range mergedProviders {
		if err := mergedCatalog.Providers().Add(&provider); err != nil {
			resultBuilder.WithError(errors.WrapResource("set", "provider", string(provider.ID), err))
		}
	}

	// Combine provenance
	allProvenance := make(ProvenanceMap)
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
		TotalTimeMs:       time.Since(startTime).Milliseconds(),
	}

	// Create a changeset if we have a base catalog to compare against
	var changeset *Changeset
	if len(sources) > 0 {
		// Use the first source as the base for comparison
		// In practice, you might want to be more sophisticated about this
		var baseCatalog catalogs.Catalog
		for _, catalog := range sources {
			baseCatalog = catalog
			break
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
		differ := NewDiffer()
		modelChangeset := differ.DiffModels(
			catalogModelsToSlice(baseCatalog),
			allMergedModels,
		)
		
		// Build proper changeset structure
		changeset = &Changeset{
			Models: modelChangeset,
			Summary: ChangesetSummary{
				ModelsAdded:   len(modelChangeset.Added),
				ModelsUpdated: len(modelChangeset.Updated),
				ModelsRemoved: len(modelChangeset.Removed),
			},
		}
	}

	// Build and return result
	return resultBuilder.
		WithCatalog(mergedCatalog).
		WithProvenance(allProvenance).
		WithStatistics(stats).
		WithChangeset(changeset).
		Build(), nil
}

// ReconcileModels merges models from multiple sources
func (r *reconciler) ReconcileModels(sources map[SourceName][]catalogs.Model) ([]catalogs.Model, ProvenanceMap, error) {
	return r.ReconcileModelsWithContext(context.Background(), sources)
}

// ReconcileModelsWithContext merges models from multiple sources with context support
func (r *reconciler) ReconcileModelsWithContext(ctx context.Context, sources map[SourceName][]catalogs.Model) ([]catalogs.Model, ProvenanceMap, error) {
	// Use the merger to combine models
	merger := NewStrategicMerger(r.authorities, r.strategy)
	if r.tracking && r.provenance != nil {
		merger.WithProvenance(r.provenance)
	}
	
	// Merge models from sources
	mergedModels, provenance, err := merger.MergeModels(sources)
	if err != nil {
		return nil, nil, err
	}
	
	// Apply enhancers if configured
	if r.enhancers != nil && len(r.enhancers.enhancers) > 0 {
		enhanced, err := r.enhancers.EnhanceBatch(ctx, mergedModels)
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

// ReconcileProviders merges providers from multiple sources
func (r *reconciler) ReconcileProviders(sources map[SourceName][]catalogs.Provider) ([]catalogs.Provider, ProvenanceMap, error) {
	// Use the merger to combine providers
	merger := NewStrategicMerger(r.authorities, r.strategy)
	if r.tracking && r.provenance != nil {
		merger.WithProvenance(r.provenance)
	}
	
	return merger.MergeProviders(sources)
}

// getSourceNames extracts source names from a map
func getSourceNames(sources map[SourceName]catalogs.Catalog) []SourceName {
	names := make([]SourceName, 0, len(sources))
	for name := range sources {
		names = append(names, name)
	}
	return names
}

// catalogModelsToSlice converts catalog models to a slice
func catalogModelsToSlice(catalog catalogs.Reader) []catalogs.Model {
	models := catalog.Models().List()
	result := make([]catalogs.Model, 0, len(models))
	for _, m := range models {
		result = append(result, *m)
	}
	return result
}

// Option Functions
// ================

// WithStrategy sets the merge strategy
func WithStrategy(strategy Strategy) Option {
	return func(r *reconciler) error {
		if strategy == nil {
			return &errors.ValidationError{
				Field:   "strategy",
				Message: "cannot be nil",
			}
		}
		r.strategy = strategy
		return nil
	}
}

// WithAuthorities sets the field authorities
func WithAuthorities(authorities AuthorityProvider) Option {
	return func(r *reconciler) error {
		if authorities == nil {
			return &errors.ValidationError{
				Field:   "authorities",
				Message: "cannot be nil",
			}
		}
		r.authorities = authorities
		// If using authority-based strategy, update it
		if authStrategy, ok := r.strategy.(*AuthorityBasedStrategy); ok {
			authStrategy.authorities = authorities
		}
		return nil
	}
}

// WithProvenance enables field-level tracking
func WithProvenance(enabled bool) Option {
	return func(r *reconciler) error {
		r.tracking = enabled
		if enabled && r.provenance == nil {
			r.provenance = NewProvenanceTracker(true)
		}
		return nil
	}
}

// WithEnhancers adds model enhancers to the pipeline
func WithEnhancers(enhancers ...Enhancer) Option {
	return func(r *reconciler) error {
		r.enhancers = NewEnhancerPipeline(enhancers...)
		if r.tracking && r.provenance != nil {
			r.enhancers.WithProvenance(r.provenance)
		}
		return nil
	}
}