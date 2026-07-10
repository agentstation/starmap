// Package update provides the update command implementation.
package update

import (
	"strings"

	"github.com/agentstation/starmap/pkg/catalogs"
	pkgerrors "github.com/agentstation/starmap/pkg/errors"
	"github.com/agentstation/starmap/pkg/sources"
	"github.com/agentstation/starmap/pkg/sync"
)

// BuildUpdateOptions creates a slice of update options based on the provided flags.
func BuildUpdateOptions(provider, source, output string, dryRun, force, cleanup, reformat bool, sourcesDir, modelsDevGitCommit string, autoInstallDeps, skipDepPrompts, requireAllSources bool) ([]sync.Option, error) {
	var opts []sync.Option

	if provider != "" {
		opts = append(opts, sync.WithProvider(catalogs.ProviderID(provider)))
	}
	if source != "" {
		sourceIDs, err := sourceSelection(source)
		if err != nil {
			return nil, err
		}
		if len(sourceIDs) > 0 {
			opts = append(opts, sync.WithSources(sourceIDs...))
		}
	}
	if dryRun {
		opts = append(opts, sync.WithDryRun(true))
	}
	if output != "" {
		opts = append(opts, sync.WithOutputPath(output))
	}
	// Use typed options for source-specific behavior
	if force {
		opts = append(opts, sync.WithFresh(true))
	}
	if cleanup {
		opts = append(opts, sync.WithCleanModelsDevRepo(true))
	}
	if reformat {
		opts = append(opts, sync.WithReformat(true))
	}
	if sourcesDir != "" {
		opts = append(opts, sync.WithSourcesDir(sourcesDir))
	}
	if modelsDevGitCommit != "" {
		opts = append(opts, sync.WithModelsDevGitCommit(modelsDevGitCommit))
	}
	// Dependency options
	if autoInstallDeps {
		opts = append(opts, sync.WithAutoInstallDeps(true))
	}
	if skipDepPrompts {
		opts = append(opts, sync.WithSkipDepPrompts(true))
	}
	if requireAllSources {
		opts = append(opts, sync.WithRequireAllSources(true))
	}
	if !autoInstallDeps && !skipDepPrompts {
		opts = append(opts, sync.WithDependencyDecisionHandler(promptForMissingDependency))
	}

	return opts, nil
}

func sourceSelection(source string) ([]sources.ID, error) {
	switch strings.ToLower(strings.TrimSpace(source)) {
	case "", "all":
		return nil, nil
	case "provider-api", "provider", "providers", "api":
		return []sources.ID{sources.ProvidersID}, nil
	case "models.dev", "modelsdev", "models-dev", "models_dev", "models_dev_http", "models.dev-http":
		return []sources.ID{sources.ModelsDevHTTPID}, nil
	case "models.dev-git", "modelsdev-git", "models-dev-git", "models_dev_git":
		return []sources.ID{sources.ModelsDevGitID}, nil
	default:
		return nil, &pkgerrors.ValidationError{
			Field:   "source",
			Value:   source,
			Message: "must be one of: all, provider-api, models.dev, models.dev-git",
		}
	}
}
