package starmap

import (
	"context"

	"github.com/agentstation/starmap/internal/catalog/pipeline"
	"github.com/agentstation/starmap/internal/catalog/workspace"
	"github.com/agentstation/starmap/internal/providers/clients"
	"github.com/agentstation/starmap/pkg/catalogs"
	"github.com/agentstation/starmap/pkg/sources"
	pkgsync "github.com/agentstation/starmap/pkg/sync"
)

// Sync is a test-only bridge for historical root integration tests while the
// production composition lives in package acquisition. New tests must exercise
// acquisition.Syncer directly.
func (c *Client) Sync(
	ctx context.Context,
	opts ...pkgsync.Option,
) (*pkgsync.Result, error) {
	factory := func(provider *catalogs.Provider) (sources.ProviderClient, error) {
		return clients.NewProvider(provider)
	}
	runner := pipeline.NewAcquisition(factory)
	parsed := pkgsync.Defaults().Apply(opts...)
	effective := append([]pkgsync.Option(nil), opts...)
	if parsed.CatalogPath == "" && c.WorkspacePath() != "" {
		effective = append(effective, pkgsync.WithCatalogPath(c.WorkspacePath()))
		parsed = pkgsync.Defaults().Apply(effective...)
	}
	if parsed.DryRun {
		prepared, err := runner.Prepare(ctx, c.Catalog(), effective...)
		if err != nil {
			return nil, err
		}
		return prepared.Result, nil
	}

	var prepared *pipeline.Prepared
	publication, err := c.Update(ctx, func(
		updateCtx context.Context,
		current *catalogs.Catalog,
	) (*Candidate, error) {
		var prepareErr error
		prepared, prepareErr = runner.Prepare(updateCtx, current, effective...)
		if prepareErr != nil || !prepared.Publish {
			return nil, prepareErr
		}
		catalog, buildErr := prepared.Catalog.Build()
		if buildErr != nil {
			return nil, buildErr
		}
		links := make([]catalogs.SourceObservationLink, 0, len(prepared.Observations))
		for _, observation := range prepared.Observations {
			links = append(links, observation.Link())
		}
		return NewCandidate(catalog, links...)
	})
	if err != nil {
		return nil, err
	}
	result := prepared.Result
	if publication.Published {
		result.GenerationID = publication.GenerationID
		result.SyncRunID = publication.SyncRunID
		if prepared.Options.CatalogPath != "" {
			result.Projection = projectRollbackCatalog(
				ctx,
				c.Catalog(),
				prepared.Options.CatalogPath,
				workspace.Identity{
					GenerationID:    publication.GenerationID,
					PayloadChecksum: publication.PayloadChecksum,
				},
				prepared.WorkspaceInput,
			)
		}
	}
	return result, nil
}
