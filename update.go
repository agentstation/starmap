package starmap

import (
	"context"

	"github.com/agentstation/starmap/pkg/authority"
	"github.com/agentstation/starmap/pkg/catalogs"
	"github.com/agentstation/starmap/pkg/errors"
	"github.com/agentstation/starmap/pkg/reconciler"
	"github.com/agentstation/starmap/pkg/sources"
)

// update reconciles the catalog with an optional baseline for comparison.
func update(ctx context.Context, baseline catalogs.Catalog, srcs []sources.Source) (*reconciler.Result, error) {
	// create reconciler options
	opts := []reconciler.Option{
		reconciler.WithStrategy(reconciler.NewAuthorityStrategy(authority.New())),
	}

	// Add baseline if provided
	if baseline != nil {
		opts = append(opts, reconciler.WithBaseline(baseline))
	}

	// create a new reconciler
	reconcile, err := reconciler.New(opts...)
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
