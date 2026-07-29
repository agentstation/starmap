package acquisition

import (
	"context"

	"github.com/agentstation/starmap"
	"github.com/agentstation/starmap/internal/catalog/workspace"
	"github.com/agentstation/starmap/internal/sources/modelsdev"
	"github.com/agentstation/starmap/pkg/catalogs"
	"github.com/agentstation/starmap/pkg/logging"
	pkgsync "github.com/agentstation/starmap/pkg/sync"
)

func projectCommittedCatalog(
	ctx context.Context,
	catalog *catalogs.Catalog,
	path string,
	publication starmap.Publication,
	input workspace.InputExpectation,
) *pkgsync.ProjectionResult {
	result := &pkgsync.ProjectionResult{
		Path:         path,
		Status:       pkgsync.ProjectionStatusPendingRepair,
		GenerationID: publication.GenerationID,
	}
	if catalog == nil {
		result.IssueCode = pkgsync.ProjectionIssueWorkspaceFailed
		return result
	}
	receipt, err := projectCatalogWorkspace(
		ctx,
		catalog,
		path,
		workspace.Identity{
			GenerationID:    publication.GenerationID,
			PayloadChecksum: publication.PayloadChecksum,
		},
		input,
	)
	result.WorkspaceChecksum = receipt.WorkspaceChecksum
	if err != nil {
		result.IssueCode = pkgsync.ProjectionIssueWorkspaceFailed
		logging.Warn().
			Err(err).
			Str("generation_id", publication.GenerationID).
			Str("catalog_path", path).
			Msg("Catalog generation committed; YAML workspace projection is pending repair")
		return result
	}
	result.Status = pkgsync.ProjectionStatusApplied
	return result
}

func projectCatalogWorkspace(
	ctx context.Context,
	catalog *catalogs.Catalog,
	outputPath string,
	identity workspace.Identity,
	input workspace.InputExpectation,
) (workspace.Receipt, error) {
	receipt, err := workspace.ProjectExpected(ctx, outputPath, catalog, identity, input)
	if err != nil {
		return receipt, err
	}

	providers := catalog.Providers().List()
	providerPointers := make([]*catalogs.Provider, len(providers))
	for index := range providers {
		providerPointers[index] = &providers[index]
	}
	if len(providerPointers) > 0 {
		if err := modelsdev.CopyProviderLogos(outputPath, providerPointers); err != nil {
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
