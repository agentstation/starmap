package starmap

import (
	"context"

	"github.com/agentstation/starmap/pkg/authority"
	"github.com/agentstation/starmap/pkg/errors"
	"github.com/agentstation/starmap/pkg/reconciler"
	"github.com/agentstation/starmap/pkg/sources"
)

// update reconciles the catalog from all sources
func update(ctx context.Context, srcs []sources.Source) (*reconciler.Result, error) {

	// create a new reconciler
	strategy := reconciler.NewAuthorityStrategy(authority.New())
	reconcile, err := reconciler.New(reconciler.WithStrategy(strategy))
	if err != nil {
		return nil, errors.WrapResource("create", "reconciler", "", err)
	}

	// reconcile the sources catalogs into a single result
	result, err := reconcile.Sources(ctx, sources.ProviderAPI, srcs)
	if err != nil {
		return nil, &errors.SyncError{
			Provider: "all",
			Err:      err,
		}
	}

	return result, nil
}
