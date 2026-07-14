package pipeline

import (
	"slices"

	"github.com/agentstation/starmap/internal/sources/local"
	"github.com/agentstation/starmap/internal/sources/modelsdev"
	"github.com/agentstation/starmap/internal/sources/providers"
	"github.com/agentstation/starmap/pkg/catalogs"
	"github.com/agentstation/starmap/pkg/sources"
	pkgsync "github.com/agentstation/starmap/pkg/sync"
)

func filterSources(options *pkgsync.Options, localCatalog *catalogs.Catalog) ([]sources.Source, error) {
	configuredSources, err := createSourcesWithConfig(options, localCatalog)
	if err != nil {
		return nil, err
	}
	if options.Fresh {
		configuredSources = slices.DeleteFunc(configuredSources, func(src sources.Source) bool {
			return src.ID() == sources.LocalCatalogID
		})
	}
	if options.ProviderID != nil {
		configuredSources = slices.DeleteFunc(configuredSources, func(source sources.Source) bool {
			providerSource, scoped := source.(interface{ ProviderID() catalogs.ProviderID })
			return scoped && providerSource.ProviderID() != *options.ProviderID
		})
	}

	if len(options.Sources) > 0 {
		filtered := make([]sources.Source, 0, len(options.Sources))
		for _, src := range configuredSources {
			if slices.Contains(options.Sources, src.ID()) {
				filtered = append(filtered, src)
			}
		}
		return filtered, nil
	}

	return configuredSources, nil
}

func createSourcesWithConfig(options *pkgsync.Options, localCatalog *catalogs.Catalog) ([]sources.Source, error) {
	providerSources, err := providers.NewConfigured(localCatalog.Providers())
	if err != nil {
		return nil, err
	}
	srcs := []sources.Source{local.New(local.WithCatalog(localCatalog))}
	srcs = append(srcs, providerSources...)

	useGit := slices.Contains(options.Sources, sources.ModelsDevGitID)
	useHTTP := len(options.Sources) == 0 || slices.Contains(options.Sources, sources.ModelsDevHTTPID)
	if useGit {
		gitOptions := []modelsdev.GitSourceOption{modelsdev.WithGitCommit(options.ModelsDevGitCommit)}
		if options.SourcesDir != "" {
			gitOptions = append(gitOptions, modelsdev.WithSourcesDir(options.SourcesDir))
		}
		srcs = append(srcs, modelsdev.NewGitSource(gitOptions...))
	}
	if useHTTP {
		if options.SourcesDir != "" {
			srcs = append(srcs, modelsdev.NewHTTPSource(modelsdev.WithHTTPSourcesDir(options.SourcesDir)))
		} else {
			srcs = append(srcs, modelsdev.NewHTTPSource())
		}
	}
	return srcs, nil
}
