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

func filterSources(options *pkgsync.Options, localCatalog catalogs.Catalog) []sources.Source {
	configuredSources := createSourcesWithConfig(options, localCatalog)

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

func createSourcesWithConfig(options *pkgsync.Options, localCatalog catalogs.Catalog) []sources.Source {
	srcs := []sources.Source{
		local.New(local.WithCatalog(localCatalog)),
		providers.New(localCatalog.Providers()),
	}

	if options.SourcesDir != "" {
		return append(srcs,
			modelsdev.NewGitSource(modelsdev.WithSourcesDir(options.SourcesDir)),
			modelsdev.NewHTTPSource(modelsdev.WithHTTPSourcesDir(options.SourcesDir)),
		)
	}

	return append(srcs,
		modelsdev.NewGitSource(),
		modelsdev.NewHTTPSource(),
	)
}
