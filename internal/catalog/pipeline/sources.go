package pipeline

import (
	"slices"

	"github.com/agentstation/starmap/internal/catalog/workspace"
	embeddedsrc "github.com/agentstation/starmap/internal/sources/embedded"
	"github.com/agentstation/starmap/internal/sources/local"
	"github.com/agentstation/starmap/internal/sources/modelsdev"
	"github.com/agentstation/starmap/internal/sources/providers"
	"github.com/agentstation/starmap/pkg/catalogs"
	"github.com/agentstation/starmap/pkg/sources"
	pkgsync "github.com/agentstation/starmap/pkg/sync"
)

func filterSources(
	options *pkgsync.Options,
	localCatalog *catalogs.Catalog,
	report catalogs.LoadReport,
	workspaceInput workspace.InputExpectation,
) []sources.Source {
	configuredSources := createSourcesWithConfig(options, localCatalog, report, workspaceInput)
	if options.Fresh {
		configuredSources = slices.DeleteFunc(configuredSources, func(src sources.Source) bool {
			return src.ID() == sources.LocalCatalogID
		})
	}

	if len(options.Sources) > 0 {
		filtered := make([]sources.Source, 0, len(options.Sources))
		for _, src := range configuredSources {
			if src.ID() == sources.EmbeddedCatalogID || slices.Contains(options.Sources, src.ID()) {
				filtered = append(filtered, src)
			}
		}
		return filtered
	}

	return configuredSources
}

func createSourcesWithConfig(
	options *pkgsync.Options,
	localCatalog *catalogs.Catalog,
	report catalogs.LoadReport,
	workspaceInput workspace.InputExpectation,
) []sources.Source {
	srcs := []sources.Source{
		providers.New(localCatalog.Providers()),
	}
	if workspaceInput.Exists {
		srcs = append(
			[]sources.Source{local.New(local.WithCatalogReport(localCatalog, report))},
			srcs...,
		)
	} else {
		srcs = append([]sources.Source{embeddedsrc.New(localCatalog)}, srcs...)
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
