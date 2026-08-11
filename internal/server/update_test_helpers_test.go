package server

import (
	"context"

	"github.com/agentstation/starmap"
	"github.com/agentstation/starmap/pkg/catalogs"
)

func serverCatalogUpdate(
	mutate func(*catalogs.Builder) error,
) starmap.UpdateFunc {
	return func(
		_ context.Context,
		current *catalogs.Catalog,
	) (*starmap.Candidate, error) {
		builder, err := catalogs.NewBuilderFrom(current)
		if err != nil {
			return nil, err
		}
		if err := mutate(builder); err != nil {
			return nil, err
		}
		catalog, err := builder.Build()
		if err != nil {
			return nil, err
		}
		return starmap.NewCandidate(catalog, starmap.CandidateEvidence{})
	}
}
