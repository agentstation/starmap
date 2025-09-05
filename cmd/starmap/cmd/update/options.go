// Package update provides the update command implementation.
package update

import (
	"github.com/agentstation/starmap"
	"github.com/agentstation/starmap/pkg/catalogs"
)

// BuildSyncOptions creates a slice of sync options based on the provided flags.
func BuildSyncOptions(provider, output string, dryRun, force, autoApprove, cleanup, reformat bool) []starmap.SyncOption {
	var opts []starmap.SyncOption

	if provider != "" {
		opts = append(opts, starmap.WithProvider(catalogs.ProviderID(provider)))
	}
	if dryRun {
		opts = append(opts, starmap.WithDryRun(true))
	}
	if autoApprove {
		opts = append(opts, starmap.WithAutoApprove(true))
	}
	if output != "" {
		opts = append(opts, starmap.WithOutputPath(output))
	}
	// Use typed options for source-specific behavior
	if force {
		opts = append(opts, starmap.WithFresh(true))
	}
	if cleanup {
		opts = append(opts, starmap.WithCleanModelsDevRepo(true))
	}
	if reformat {
		opts = append(opts, starmap.WithReformat(true))
	}

	return opts
}
