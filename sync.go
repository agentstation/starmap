package starmap

import (
	"context"

	"github.com/agentstation/starmap/internal/catalog/pipeline"
	"github.com/agentstation/starmap/internal/catalog/workspace"
	"github.com/agentstation/starmap/internal/sources/modelsdev"
	"github.com/agentstation/starmap/pkg/catalogs"
	"github.com/agentstation/starmap/pkg/differ"
	"github.com/agentstation/starmap/pkg/logging"
	"github.com/agentstation/starmap/pkg/sources"
	"github.com/agentstation/starmap/pkg/sync"
)

// Sync synchronizes the catalog with provider APIs using staged source execution.
func (c *Client) Sync(ctx context.Context, opts ...sync.Option) (*sync.Result, error) {
	options := sync.Defaults().Apply(opts...)
	catalogPath := options.CatalogPath
	if c.options != nil && catalogPath == "" && c.options.catalogPath != "" {
		catalogPath = c.options.catalogPath
	}
	if c.options != nil {
		if err := validateCatalogLayout(c.options.catalogStore, catalogPath); err != nil {
			return nil, err
		}
	}
	if !options.DryRun {
		if err := c.requireWritableCatalogStore(); err != nil {
			return nil, err
		}
	}

	release, err := c.updates.acquire(ctx)
	if err != nil {
		return nil, err
	}
	defer release()

	effective := append([]sync.Option(nil), opts...)
	if options.CatalogPath == "" && c.options.catalogPath != "" {
		effective = append(effective, sync.WithCatalogPath(c.options.catalogPath))
	}
	return pipeline.New(pipelineStore{client: c}).Sync(ctx, effective...)
}

// ============================================================================
// Helper Methods for Sync
// ============================================================================

// save commits and publishes the catalog, then best-effort projects the
// committed generation into an optional human YAML workspace.
func (c *Client) save(
	ctx context.Context,
	result *catalogs.Builder,
	options *sync.Options,
	changeset *differ.Changeset,
	observations []sources.Observation,
	workspaceInput workspace.InputExpectation,
) (pipeline.Publication, error) {
	published, err := snapshotBuilder(result)
	if err != nil {
		return pipeline.Publication{}, err
	}

	publication, err := c.commitAndPublish(ctx, published, observations)
	if err != nil {
		return pipeline.Publication{}, err
	}

	if options.CatalogPath != "" {
		publication.Projection = &sync.ProjectionResult{
			Path:         options.CatalogPath,
			Status:       sync.ProjectionStatusPendingRepair,
			GenerationID: publication.GenerationID,
		}
		receipt, projectionErr := projectCatalogWorkspace(
			ctx,
			published,
			options.CatalogPath,
			workspace.Identity{
				GenerationID:    publication.GenerationID,
				PayloadChecksum: publication.PayloadChecksum,
			},
			workspaceInput,
		)
		publication.Projection.WorkspaceChecksum = receipt.WorkspaceChecksum
		if projectionErr != nil {
			publication.Projection.IssueCode = sync.ProjectionIssueWorkspaceFailed
			logging.Warn().
				Err(projectionErr).
				Str("generation_id", publication.GenerationID).
				Str("catalog_path", options.CatalogPath).
				Msg("Catalog generation committed; YAML workspace projection is pending repair")
		} else {
			publication.Projection.Status = sync.ProjectionStatusApplied
		}
	}

	logging.Info().
		Int("changes_applied", changeset.Summary.TotalChanges).
		Msg("Sync completed successfully")

	return publication, nil
}

func projectCatalogWorkspace(
	ctx context.Context,
	catalog *catalogs.Catalog,
	outputPath string,
	identity workspace.Identity,
	input workspace.InputExpectation,
) (workspace.Receipt, error) {
	providers := catalog.Providers().List()
	for _, provider := range providers {
		logging.Debug().
			Str("provider", string(provider.ID)).
			Int("models", len(provider.Models)).
			Msg("Provider model count before workspace projection")
	}

	receipt, err := workspace.ProjectExpected(ctx, outputPath, catalog, identity, input)
	if err != nil {
		return receipt, err
	}

	providerPtrs := make([]*catalogs.Provider, len(providers))
	for i := range providers {
		providerPtrs[i] = &providers[i]
	}
	if len(providerPtrs) > 0 {
		if err := modelsdev.CopyProviderLogos(outputPath, providerPtrs); err != nil {
			logging.Warn().Err(err).Msg("Could not copy provider logos")
		}
	}

	authors := catalog.Authors().List()
	if len(authors) > 0 {
		if err := modelsdev.CopyAuthorLogos(outputPath, authors, catalog.Providers()); err != nil {
			logging.Warn().Err(err).Msg("Could not copy author logos")
		}
	}
	return receipt, nil
}

type pipelineStore struct {
	client *Client
}

func (s pipelineStore) Catalog() (*catalogs.Catalog, error) {
	return s.client.Catalog(), nil
}

func (s pipelineStore) Apply(
	ctx context.Context,
	catalog *catalogs.Builder,
	options *sync.Options,
	changeset *differ.Changeset,
	observations []sources.Observation,
	workspaceInput workspace.InputExpectation,
) (pipeline.Publication, error) {
	return s.client.save(ctx, catalog, options, changeset, observations, workspaceInput)
}
