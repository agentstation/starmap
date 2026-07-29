package starmap

import (
	"context"

	"github.com/agentstation/starmap/pkg/catalogs"
	"github.com/agentstation/starmap/pkg/catalogstore"
	"github.com/agentstation/starmap/pkg/errors"
)

// Candidate is a complete immutable catalog prepared off to the side for one
// atomic publication. Source observations are immutable generation evidence;
// they do not alter catalog facts.
type Candidate struct {
	catalog            *catalogs.Catalog
	sourceObservations []catalogs.SourceObservationLink
}

// NewCandidate validates and returns a publication candidate. An empty
// observation list is permitted for custom acquisition; Client.Update records
// a deterministic custom-update observation in that case.
func NewCandidate(
	catalog *catalogs.Catalog,
	observations ...catalogs.SourceObservationLink,
) (*Candidate, error) {
	if catalog == nil {
		return nil, &errors.ValidationError{
			Field:   "candidate.catalog",
			Message: "is required",
		}
	}
	for _, observation := range observations {
		if err := observation.Validate(); err != nil {
			return nil, errors.WrapResource(
				"validate",
				"candidate source observation",
				observation.ObservationID,
				err,
			)
		}
	}
	return &Candidate{
		catalog:            catalog,
		sourceObservations: append([]catalogs.SourceObservationLink(nil), observations...),
	}, nil
}

// UpdateFunc builds and validates a complete candidate while Client.Update
// holds the client's mutation transaction. Returning nil performs no
// publication. The current catalog is immutable and safe to retain.
type UpdateFunc func(context.Context, *catalogs.Catalog) (*Candidate, error)

// Publication identifies the durable generation produced by a successful
// update. Published is false when the update function intentionally returns no
// candidate or an identical retained generation is activated again.
type Publication struct {
	Published       bool
	GenerationID    string
	PayloadChecksum string
	SyncRunID       string
}

// Update serializes candidate construction, generation-store CAS, and atomic
// in-memory publication. Acquisition and scheduling remain explicit caller
// composition above Client.
func (c *Client) Update(ctx context.Context, update UpdateFunc) (Publication, error) {
	if c == nil {
		return Publication{}, &errors.ValidationError{
			Field:   "starmap.client",
			Message: "is required",
		}
	}
	if update == nil {
		return Publication{}, &errors.ValidationError{
			Field:   "starmap.update",
			Message: "update function is required",
		}
	}
	if err := c.requireWritableCatalogStore(); err != nil {
		return Publication{}, err
	}
	if ctx == nil {
		ctx = context.Background()
	}
	release, err := c.updates.acquire(ctx)
	if err != nil {
		return Publication{}, err
	}
	defer release()

	candidate, err := update(ctx, c.Catalog())
	if err != nil {
		return Publication{}, err
	}
	if candidate == nil {
		return Publication{}, nil
	}
	return c.commitAndPublish(ctx, candidate.catalog, candidate.sourceObservations)
}

// Activate validates, durably commits, and atomically activates an immutable
// generation obtained by an explicit trusted distribution adapter.
func (c *Client) Activate(ctx context.Context, generation catalogstore.Generation) (Publication, error) {
	if c == nil {
		return Publication{}, &errors.ValidationError{
			Field:   "starmap.client",
			Message: "is required",
		}
	}
	if err := c.requireWritableCatalogStore(); err != nil {
		return Publication{}, err
	}
	if ctx == nil {
		ctx = context.Background()
	}
	if generation.Manifest.SchemaVersion != catalogs.CurrentCatalogSchemaVersion ||
		!generation.Manifest.ConsumerCompatibility.SupportsSchema(
			catalogs.CurrentCatalogSchemaVersion,
		) {
		return Publication{}, &errors.ValidationError{
			Field:   "catalog_generation.schema_version",
			Value:   generation.Manifest.SchemaVersion,
			Message: "is not compatible with this Starmap catalog schema",
		}
	}
	release, err := c.updates.acquire(ctx)
	if err != nil {
		return Publication{}, err
	}
	defer release()

	published, err := catalogstore.DecodeCatalogPayload(generation.Payload)
	if err != nil {
		return Publication{}, errors.WrapResource(
			"decode",
			"catalog generation",
			generation.Manifest.GenerationID,
			err,
		)
	}
	return c.commitReceivedGeneration(ctx, published, generation)
}

func (c *Client) swapCatalogGeneration(
	published *catalogs.Catalog,
	generationID string,
) (*catalogs.Catalog, uint64) {
	c.mu.Lock()
	oldCatalog := c.catalog
	c.catalog = published
	c.usingEmbeddedBootstrap = false
	c.generationSequence++
	sequence := c.generationSequence
	if generationID != "" {
		c.generationID = generationID
	}
	c.mu.Unlock()
	return oldCatalog, sequence
}
