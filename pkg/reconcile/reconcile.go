package reconcile

import (
	"context"
	"fmt"
	"time"

	"github.com/agentstation/starmap/pkg/catalogs"
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
	ReconcileCatalogs(ctx context.Context, sources map[SourceName]catalogs.Catalog) (*Result, error)

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
func (r *reconciler) ReconcileCatalogs(ctx context.Context, sources map[SourceName]catalogs.Catalog) (*Result, error) {
	startTime := time.Now()
	
	// Create result builder
	resultBuilder := NewResultBuilder().
		WithSources(getSourceNames(sources)...).
		WithStrategy(r.strategy)

	// Create merged catalog
	mergedCatalog, err := catalogs.New()
	if err != nil {
		return nil, fmt.Errorf("creating merged catalog: %w", err)
	}

	// Extract models from all sources
	modelSources := make(map[SourceName][]catalogs.Model)
	for sourceName, catalog := range sources {
		models := catalog.Models().List()
		modelList := make([]catalogs.Model, 0, len(models))
		for _, m := range models {
			modelList = append(modelList, *m)
		}
		modelSources[sourceName] = modelList
	}

	// Reconcile models
	mergedModels, modelProvenance, err := r.ReconcileModels(modelSources)
	if err != nil {
		return nil, fmt.Errorf("reconciling models: %w", err)
	}

	// Add merged models to catalog
	for _, model := range mergedModels {
		if err := mergedCatalog.Models().Add(&model); err != nil {
			resultBuilder.WithError(fmt.Errorf("setting model %s: %w", model.ID, err))
		}
	}

	// Extract providers from all sources
	providerSources := make(map[SourceName][]catalogs.Provider)
	for sourceName, catalog := range sources {
		providers := catalog.Providers().List()
		providerList := make([]catalogs.Provider, 0, len(providers))
		for _, p := range providers {
			providerList = append(providerList, *p)
		}
		providerSources[sourceName] = providerList
	}

	// Reconcile providers
	mergedProviders, providerProvenance, err := r.ReconcileProviders(providerSources)
	if err != nil {
		return nil, fmt.Errorf("reconciling providers: %w", err)
	}

	// Add merged providers to catalog
	for _, provider := range mergedProviders {
		if err := mergedCatalog.Providers().Add(&provider); err != nil {
			resultBuilder.WithError(fmt.Errorf("setting provider %s: %w", provider.ID, err))
		}
	}

	// Combine provenance
	allProvenance := make(ProvenanceMap)
	for k, v := range modelProvenance {
		allProvenance[k] = v
	}
	for k, v := range providerProvenance {
		allProvenance[k] = v
	}

	// Build statistics
	stats := ResultStatistics{
		ModelsProcessed:    len(mergedModels),
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
		
		// Create differ to detect changes
		differ := NewDiffer()
		modelChangeset := differ.DiffModels(
			catalogModelsToSlice(baseCatalog),
			mergedModels,
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
		ctx := context.Background()
		enhanced, err := r.enhancers.EnhanceBatch(ctx, mergedModels)
		if err != nil {
			// Log but don't fail - enhancement is optional
			fmt.Printf("Warning: enhancement failed: %v\n", err)
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
func catalogModelsToSlice(catalog catalogs.Catalog) []catalogs.Model {
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
			return fmt.Errorf("strategy cannot be nil")
		}
		r.strategy = strategy
		return nil
	}
}

// WithAuthorities sets the field authorities
func WithAuthorities(authorities AuthorityProvider) Option {
	return func(r *reconciler) error {
		if authorities == nil {
			return fmt.Errorf("authorities cannot be nil")
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