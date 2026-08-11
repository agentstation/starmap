package pipeline

import (
	"slices"

	embeddedsrc "github.com/agentstation/starmap/internal/sources/embedded"
	"github.com/agentstation/starmap/internal/sources/local"
	"github.com/agentstation/starmap/internal/sources/modelsdev"
	"github.com/agentstation/starmap/internal/sources/providers"
	"github.com/agentstation/starmap/pkg/sources"
	pkgsync "github.com/agentstation/starmap/pkg/sync"
)

type providerSourceComposition struct {
	clientFactory      sources.ProviderClientFactory
	credentialResolver sources.ProviderCredentialResolver
}

func filterSources(
	options *pkgsync.Options,
	inputs catalogInputs,
	composition providerSourceComposition,
) []sources.Source {
	configuredSources := createSourcesWithConfig(
		options,
		inputs,
		composition,
	)
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
	inputs catalogInputs,
	composition providerSourceComposition,
) []sources.Source {
	configuredProviders := inputs.providerConfig.Providers()
	srcs := []sources.Source{
		embeddedsrc.New(inputs.embedded),
		providers.New(
			configuredProviders,
			providers.WithClientFactory(composition.clientFactory),
			providers.WithCredentialResolver(composition.credentialResolver),
		),
	}
	if inputs.workspaceInput.Exists {
		srcs = append(
			[]sources.Source{local.New(local.WithCatalogReport(inputs.workspace, inputs.workspaceReport))},
			srcs...,
		)
	}

	useGit := slices.Contains(options.Sources, sources.ModelsDevGitID)
	useHTTP := len(options.Sources) == 0 || slices.Contains(options.Sources, sources.ModelsDevHTTPID)
	if useGit {
		gitOptions := []modelsdev.GitSourceOption{
			modelsdev.WithGitCommit(options.ModelsDevGitCommit),
			modelsdev.WithGitProviders(configuredProviders),
		}
		if options.SourcesDir != "" {
			gitOptions = append(gitOptions, modelsdev.WithSourcesDir(options.SourcesDir))
		}
		srcs = append(srcs, modelsdev.NewGitSource(gitOptions...))
	}
	if useHTTP {
		httpOptions := []modelsdev.HTTPSourceOption{
			modelsdev.WithHTTPProviders(configuredProviders),
		}
		if options.SourcesDir != "" {
			httpOptions = append(httpOptions, modelsdev.WithHTTPSourcesDir(options.SourcesDir))
		}
		srcs = append(srcs, modelsdev.NewHTTPSource(httpOptions...))
	}
	return srcs
}
