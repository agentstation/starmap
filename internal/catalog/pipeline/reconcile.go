package pipeline

import (
	"context"

	"github.com/agentstation/starmap/internal/catalog/reconciler"
	"github.com/agentstation/starmap/pkg/catalogs"
	"github.com/agentstation/starmap/pkg/sources"
)

func reconcile(ctx context.Context, baseline *catalogs.Catalog, observations []sources.Observation) (*reconciler.Result, error) {
	return reconciler.ReconcileObservations(ctx, baseline, observations)
}
