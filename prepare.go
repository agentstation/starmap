package starmap

import (
	"context"

	"github.com/agentstation/starmap/pkg/catalogs"
	"github.com/agentstation/starmap/pkg/errors"
)

// PrepareGeneration encodes a candidate with its final ordinary manifest and evidence.
// It writes no storage and changes no active catalog. The caller owns the returned bytes.
// A transaction can bind these exact bytes to its journal before Activate commits them.
// Preparation reserves no predecessor, so activation still requires the store's atomic compare-and-swap.
func (c *Client) PrepareGeneration(ctx context.Context, candidate *Candidate) (catalogs.Generation, error) {
	if c == nil || candidate == nil || candidate.catalog == nil || ctx == nil {
		return catalogs.Generation{}, &errors.ValidationError{Field: "catalog_generation.preparation", Message: "a client, candidate, and context are required"}
	}
	if err := ctx.Err(); err != nil {
		return catalogs.Generation{}, err
	}
	generation, err := c.newGeneration(candidate.catalog, candidate.evidence, candidate.generationID)
	if err != nil {
		return catalogs.Generation{}, err
	}
	if err := ctx.Err(); err != nil {
		return catalogs.Generation{}, err
	}
	return generation, nil
}
