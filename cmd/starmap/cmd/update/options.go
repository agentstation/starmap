// Package update provides the update command implementation.
package update

import (
	"github.com/agentstation/starmap/pkg/catalogs"
	"github.com/agentstation/starmap/pkg/sync"
)

// BuildUpdateOptions creates a slice of update options based on the provided flags.
func BuildUpdateOptions(provider, output string, dryRun, force, autoApprove, cleanup, reformat bool, sourcesDir string) []sync.Option {
	var opts []sync.Option

	if provider != "" {
		opts = append(opts, sync.WithProvider(catalogs.ProviderID(provider)))
	}
	if dryRun {
		opts = append(opts, sync.WithDryRun(true))
	}
	if autoApprove {
		opts = append(opts, sync.WithAutoApprove(true))
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

	return opts
}
