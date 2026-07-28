package starmap

import (
	"context"
	"strings"

	"github.com/agentstation/starmap/internal/catalog/workspace"
	"github.com/agentstation/starmap/pkg/catalogs"
	"github.com/agentstation/starmap/pkg/catalogstore"
	"github.com/agentstation/starmap/pkg/errors"
	pkgsync "github.com/agentstation/starmap/pkg/sync"
)

// RollbackResult describes activation of a retained immutable generation.
type RollbackResult struct {
	// FromGenerationID is the generation active before rollback.
	FromGenerationID string
	// GenerationID is the retained generation selected by rollback.
	GenerationID string
	// PayloadChecksum identifies the exact immutable generation payload.
	PayloadChecksum string
	// Sequence is the in-process publication sequence after rollback.
	Sequence uint64
	// Projection reports exact workspace restoration or pending repair.
	Projection *pkgsync.ProjectionResult
}

// Rollback atomically makes a retained generation current and projects its
// exact catalog semantics and provenance into the configured human workspace.
// Repeating a rollback to the current durable generation is idempotent.
func (c *Client) Rollback(ctx context.Context, generationID string) (*RollbackResult, error) {
	if c == nil {
		return nil, &errors.ValidationError{Field: "starmap.client", Message: "is required"}
	}
	generationID = strings.TrimSpace(generationID)
	if generationID == "" {
		return nil, &errors.ValidationError{Field: "generation_id", Message: "is required"}
	}
	if ctx == nil {
		ctx = context.Background()
	}
	if err := c.requireWritableCatalogStore(); err != nil {
		return nil, err
	}
	catalogPath := c.options.catalogPath
	if err := validateCatalogLayout(c.options.catalogStore, catalogPath); err != nil {
		return nil, err
	}

	release, err := c.updates.acquire(ctx)
	if err != nil {
		return nil, err
	}
	defer release()

	target, err := c.Generation(ctx, generationID)
	if err != nil {
		return nil, err
	}
	if err := target.Validate(); err != nil {
		return nil, errors.WrapResource("validate", "rollback generation", generationID, err)
	}
	if target.Manifest.GenerationID != generationID {
		return nil, &errors.ValidationError{
			Field:   "rollback.generation_id",
			Value:   target.Manifest.GenerationID,
			Message: "does not match the requested retained generation",
		}
	}
	published, err := catalogstore.DecodeCatalogPayload(target.Payload)
	if err != nil {
		return nil, errors.WrapResource("decode", "rollback generation", generationID, err)
	}

	input, err := observeBoundWorkspaceInput(catalogPath)
	if err != nil {
		return nil, err
	}

	c.mu.RLock()
	expectedGenerationID := c.generationID
	fromGenerationID := expectedGenerationID
	if fromGenerationID == "" && c.usingEmbeddedBootstrap {
		fromGenerationID = c.embeddedBootstrap.GenerationID
	}
	sequence := c.generationSequence
	c.mu.RUnlock()

	result := &RollbackResult{
		FromGenerationID: fromGenerationID,
		GenerationID:     target.Manifest.GenerationID,
		PayloadChecksum:  target.Manifest.Payload.Checksum,
		Sequence:         sequence,
	}
	if expectedGenerationID == "" && fromGenerationID == target.Manifest.GenerationID {
		return result, nil
	}

	if err := c.options.catalogStore.Commit(ctx, target, expectedGenerationID); err != nil {
		return nil, errors.WrapResource("rollback", "catalog generation", generationID, err)
	}
	if expectedGenerationID != target.Manifest.GenerationID {
		c.publishCommittedGeneration(published, target)
		result.Sequence = c.CurrentCatalogState().Sequence
	}
	if catalogPath != "" {
		result.Projection = projectCommittedCatalog(
			ctx,
			published,
			catalogPath,
			workspace.Identity{
				GenerationID:    target.Manifest.GenerationID,
				PayloadChecksum: target.Manifest.Payload.Checksum,
			},
			input,
		)
	}
	return result, nil
}

func observeBoundWorkspaceInput(path string) (workspace.InputExpectation, error) {
	input, err := workspace.ObserveInput(path)
	if err != nil || !input.Exists {
		return input, err
	}
	builder, err := catalogs.NewFromPath(path)
	if err != nil {
		return workspace.InputExpectation{}, errors.WrapResource("load", "rollback workspace", path, err)
	}
	if err := builder.LoadReport().Err(); err != nil {
		return workspace.InputExpectation{}, errors.WrapResource("load", "rollback workspace model", path, err)
	}
	catalog, err := builder.Build()
	if err != nil {
		return workspace.InputExpectation{}, errors.WrapResource("publish", "rollback workspace input", path, err)
	}
	return workspace.BindInputCatalog(input, catalog)
}
