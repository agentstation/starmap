package reconciler

import (
	"context"

	"github.com/agentstation/starmap/pkg/catalogs"
	"github.com/agentstation/starmap/pkg/catalogs/authority"
	pkgerrors "github.com/agentstation/starmap/pkg/errors"
	"github.com/agentstation/starmap/pkg/sources"
)

// ReconcileObservations applies source selection, baseline enrichment, and canonical field authority.
// The caller supplies verified observations and owns acquisition and publication.
func ReconcileObservations(ctx context.Context, baseline *catalogs.Catalog, srcs []sources.Observation, extra ...Option) (*Result, error) {
	primary := reconciliationPrimary(srcs)
	collectionSource := sources.ID("")
	if isModelsDevSource(primary) && baseline != nil && baseline.Providers().Len() > 0 {
		collectionSource = primary
	}
	srcs = reconciliationSources(baseline, srcs, primary)
	primary = reconciliationPrimaryAfterEnrichment(baseline, srcs, primary)

	opts := []Option{
		WithAuthorities(authority.New()),
		func(options *options) error {
			options.baselineProviderSource = collectionSource
			return nil
		},
	}

	if baseline != nil {
		opts = append(opts, WithBaseline(baseline))
	}

	reconcile, err := New(append(opts, extra...)...)
	if err != nil {
		return nil, pkgerrors.WrapResource("create", "reconciler", "", err)
	}

	result, err := reconcile.Sources(ctx, primary, srcs)
	if err != nil {
		return nil, &pkgerrors.SyncError{
			Provider: "all",
			Err:      err,
		}
	}

	return result, nil
}

func reconciliationSources(baseline *catalogs.Catalog, srcs []sources.Observation, primary sources.ID) []sources.Observation {
	if !needsBaselineEnrichment(baseline, srcs, primary) {
		return srcs
	}
	enriched := make([]sources.Observation, 0, len(srcs)+1)
	enriched = append(enriched, sources.Observation{SourceID: sources.LocalCatalogID, Catalog: baseline})
	return append(enriched, srcs...)
}

func isModelsDevSource(source sources.ID) bool {
	return source == sources.ModelsDevHTTPID || source == sources.ModelsDevGitID
}

func reconciliationPrimaryAfterEnrichment(baseline *catalogs.Catalog, srcs []sources.Observation, primary sources.ID) sources.ID {
	if primary != sources.ModelsDevHTTPID && primary != sources.ModelsDevGitID {
		return primary
	}
	if baseline == nil || baseline.Providers().Len() == 0 {
		return primary
	}
	if hasSource(srcs, sources.LocalCatalogID) {
		return sources.LocalCatalogID
	}
	return primary
}

type providerSetter interface {
	SetProvider(catalogs.Provider) error
}

func setBaselineProviders(filtered providerSetter, sourceCatalog, baseline catalogs.Reader, permit func(catalogs.ProviderID) bool) error {
	if sourceCatalog == nil || baseline == nil {
		return nil
	}

	for _, baselineProvider := range baseline.Providers().List() {
		sourceProvider, ok := resolveProviderForBaseline(sourceCatalog, baselineProvider)
		if !ok || (permit != nil && !permit(sourceProvider.ID)) {
			continue
		}
		sourceProvider.ID = baselineProvider.ID
		if err := filtered.SetProvider(sourceProvider); err != nil {
			return pkgerrors.WrapResource(
				"filter",
				"provider",
				baselineProvider.ID.String(),
				err,
			)
		}
	}
	return nil
}

func resolveProviderForBaseline(sourceCatalog catalogs.Reader, baselineProvider catalogs.Provider) (catalogs.Provider, bool) {
	if provider, err := sourceCatalog.Provider(baselineProvider.ID); err == nil {
		return provider, true
	}
	for _, alias := range baselineProvider.Aliases {
		if provider, err := sourceCatalog.Provider(alias); err == nil {
			return provider, true
		}
	}
	return catalogs.Provider{}, false
}

func needsBaselineEnrichment(baseline *catalogs.Catalog, srcs []sources.Observation, primary sources.ID) bool {
	if primary != sources.ModelsDevHTTPID && primary != sources.ModelsDevGitID {
		return false
	}
	if baseline == nil || baseline.Providers().Len() == 0 {
		return false
	}
	return !hasSource(srcs, sources.LocalCatalogID)
}

func reconciliationPrimary(srcs []sources.Observation) sources.ID {
	for _, preferred := range []sources.ID{
		sources.ProvidersID,
		sources.ModelsDevHTTPID,
		sources.ModelsDevGitID,
	} {
		for _, src := range srcs {
			if src.SourceID == preferred {
				return preferred
			}
		}
	}
	return ""
}

func hasSource(srcs []sources.Observation, id sources.ID) bool {
	for _, src := range srcs {
		if src.SourceID == id {
			return true
		}
	}
	return false
}
