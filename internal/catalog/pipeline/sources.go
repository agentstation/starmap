package pipeline

import (
	"slices"

	"github.com/agentstation/starmap/internal/providers/azurefoundry"
	"github.com/agentstation/starmap/internal/providers/bedrock"
	"github.com/agentstation/starmap/internal/providers/oci"
	"github.com/agentstation/starmap/internal/sources/local"
	"github.com/agentstation/starmap/internal/sources/modelsdev"
	"github.com/agentstation/starmap/internal/sources/providers"
	"github.com/agentstation/starmap/pkg/catalogs"
	"github.com/agentstation/starmap/pkg/sources"
	pkgsync "github.com/agentstation/starmap/pkg/sync"
)

func filterSources(options *pkgsync.Options, localCatalog *catalogs.Catalog) []sources.Source {
	configuredSources := createSourcesWithConfig(options, localCatalog)
	if options.Fresh {
		configuredSources = slices.DeleteFunc(configuredSources, func(src sources.Source) bool {
			return src.ID() == sources.LocalCatalogID
		})
	}

	if len(options.Sources) > 0 {
		filtered := make([]sources.Source, 0, len(options.Sources))
		for _, src := range configuredSources {
			if slices.Contains(options.Sources, src.ID()) {
				filtered = append(filtered, src)
			}
		}
		return filtered
	}

	return configuredSources
}

func createSourcesWithConfig(options *pkgsync.Options, localCatalog *catalogs.Catalog) []sources.Source {
	srcs := []sources.Source{
		local.New(local.WithCatalog(localCatalog)),
		providers.New(localCatalog.Providers()),
		bedrock.NewCommercialSource(),
		azurefoundry.NewCommercialSource(),
		oci.NewCommercialSource(),
	}

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
	return srcs
}
