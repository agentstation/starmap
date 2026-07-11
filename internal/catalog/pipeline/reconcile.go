package pipeline

import (
	"context"

	"github.com/agentstation/starmap/pkg/authority"
	"github.com/agentstation/starmap/pkg/catalogs"
	pkgerrors "github.com/agentstation/starmap/pkg/errors"
	"github.com/agentstation/starmap/pkg/reconciler"
	"github.com/agentstation/starmap/pkg/sources"
)

func reconcile(ctx context.Context, baseline *catalogs.Catalog, srcs []sources.Observation) (*reconciler.Result, error) {
	primary := reconciliationPrimary(srcs)
	var err error
	srcs, err = reconciliationSources(baseline, srcs, primary)
	if err != nil {
		return nil, &pkgerrors.SyncError{
			Provider: "all",
			Err:      err,
		}
	}
	primary = reconciliationPrimaryAfterEnrichment(baseline, srcs, primary)

	opts := []reconciler.Option{
		reconciler.WithStrategy(reconciler.NewAuthorityStrategy(authority.New())),
	}

	if baseline != nil {
		opts = append(opts, reconciler.WithBaseline(baseline))
	}

	reconcile, err := reconciler.New(opts...)
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

func reconciliationSources(baseline *catalogs.Catalog, srcs []sources.Observation, primary sources.ID) ([]sources.Observation, error) {
	var err error
	srcs, err = restrictModelsDevPrimaryToBaseline(baseline, srcs, primary)
	if err != nil {
		return nil, err
	}
	if !needsBaselineEnrichment(baseline, srcs, primary) {
		return srcs, nil
	}

	enriched := make([]sources.Observation, 0, len(srcs)+1)
	enriched = append(enriched, sources.Observation{SourceID: sources.LocalCatalogID, Catalog: baseline})
	enriched = append(enriched, srcs...)
	return enriched, nil
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

func restrictModelsDevPrimaryToBaseline(baseline *catalogs.Catalog, srcs []sources.Observation, primary sources.ID) ([]sources.Observation, error) {
	if primary != sources.ModelsDevHTTPID && primary != sources.ModelsDevGitID {
		return srcs, nil
	}
	if baseline == nil || baseline.Providers().Len() == 0 {
		return srcs, nil
	}

	restricted := make([]sources.Observation, 0, len(srcs))
	for _, src := range srcs {
		if src.SourceID != primary {
			restricted = append(restricted, src)
			continue
		}
		filtered, err := filterCatalogToBaselineProviders(src.Catalog, baseline)
		if err != nil {
			return nil, err
		}
		src.Catalog = filtered
		restricted = append(restricted, src)
	}
	return restricted, nil
}

func filterCatalogToBaselineProviders(sourceCatalog, baseline *catalogs.Catalog) (*catalogs.Catalog, error) {
	filtered, err := filterCatalogToBaselineProvidersInto(catalogs.NewEmpty(), sourceCatalog, baseline)
	if err != nil {
		return nil, err
	}
	return filtered.Build()
}

func filterCatalogToBaselineProvidersInto(filtered *catalogs.Builder, sourceCatalog, baseline catalogs.Reader) (*catalogs.Builder, error) {
	if err := setBaselineProviders(filtered, sourceCatalog, baseline); err != nil {
		return nil, err
	}
	return filtered, nil
}

type providerSetter interface {
	SetProvider(catalogs.Provider) error
}

func setBaselineProviders(filtered providerSetter, sourceCatalog, baseline catalogs.Reader) error {
	if sourceCatalog == nil || baseline == nil {
		return nil
	}

	for _, baselineProvider := range baseline.Providers().List() {
		sourceProvider, ok := resolveProviderForBaseline(sourceCatalog, baselineProvider)
		if !ok {
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
		sources.LocalCatalogID,
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
