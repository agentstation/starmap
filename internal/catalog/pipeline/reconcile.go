package pipeline

import (
	"context"

	"github.com/agentstation/starmap/pkg/authority"
	"github.com/agentstation/starmap/pkg/catalogs"
	pkgerrors "github.com/agentstation/starmap/pkg/errors"
	"github.com/agentstation/starmap/pkg/reconciler"
	"github.com/agentstation/starmap/pkg/sources"
)

func reconcile(ctx context.Context, baseline catalogs.Catalog, srcs []sources.Source) (*reconciler.Result, error) {
	opts := []reconciler.Option{
		reconciler.WithStrategy(reconciler.NewAuthorityStrategy(authority.New())),
	}

	if baseline != nil {
		opts = append(opts, reconciler.WithBaseline(baseline))
	}

	reconcile, err := reconciler.New(opts...)
	if err != nil {
		return nil, pkgerrors.WrapResource("create", "reconciler", "", err)
	}

	result, err := reconcile.Sources(ctx, sources.ProvidersID, srcs)
	if err != nil {
		return nil, &pkgerrors.SyncError{
			Provider: "all",
			Err:      err,
		}
	}

	return result, nil
}
