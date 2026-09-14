package starmap

import (
	"context"

	"github.com/agentstation/starmap/pkg/catalogs"
	"github.com/agentstation/starmap/pkg/errors"
)

// Reload reads the caller's current catalog store and activates its validated generation without a store write.
// Publication guards apply because the operation can change this client's visible catalog.
// An optional check receives a separate generation copy before activation. It must not mutate this client.
// A failed read, check, or validation preserves the current snapshot. Acquisition and scheduling remain explicit.
func (c *Client) Reload(ctx context.Context, check func(catalogs.Generation) error) (Publication, error) {
	if err := c.requireWritableCatalogStore(); err != nil {
		return Publication{}, err
	}
	if ctx == nil {
		ctx = context.Background()
	}
	if err := c.authorizePublication(ctx); err != nil {
		return Publication{}, err
	}
	release, err := c.updates.acquire(ctx)
	if err != nil {
		return Publication{}, err
	}
	defer release()
	if err := c.authorizePublication(ctx); err != nil {
		return Publication{}, err
	}

	generation, err := c.options.catalogStore.Current(ctx)
	if err != nil {
		return Publication{}, errors.WrapResource("reload", "catalog generation", "current", err)
	}
	if !catalogs.SupportsCatalogSchema(generation.Manifest.SchemaVersion) || !generation.Manifest.ConsumerCompatibility.SupportsSchema(generation.Manifest.SchemaVersion) {
		return Publication{}, &errors.ValidationError{Field: "catalog_generation.schema_version", Message: "is not compatible with this Starmap catalog schema"}
	}
	published, err := catalogs.DecodeCatalogGeneration(generation)
	if err != nil {
		return Publication{}, err
	}
	if check != nil {
		if err := check(generation.Copy()); err != nil {
			return Publication{}, err
		}
	}
	current := c.CurrentCatalogState()
	if current.AuthorityHead != (catalogs.CatalogAuthorityHead{}) {
		if err := current.AuthorityHead.ValidateSuccessor(generation.Manifest.AuthorityHead); err != nil {
			return Publication{}, err
		}
	}
	if err := current.Catalog.CanonicalAliases().ValidateSuccessor(published.CanonicalAliases()); err != nil {
		return Publication{}, err
	}
	if err := ctx.Err(); err != nil {
		return Publication{}, err
	}
	result := Publication{GenerationID: generation.Manifest.GenerationID, PayloadChecksum: generation.Manifest.Payload.Checksum, SyncRunID: generation.Manifest.SyncRunID}
	if current.GenerationID == result.GenerationID {
		if current.PayloadChecksum != result.PayloadChecksum || current.AuthorityHead != generation.Manifest.AuthorityHead || !current.GeneratedAt.Equal(generation.Manifest.GeneratedAt) {
			return Publication{}, &errors.ConflictError{Resource: "catalog generation", Message: "selected identity already names different active content"}
		}
		c.bindCommittedGenerationIdentity(generation)
		return result, nil
	}
	if current.PayloadChecksum == result.PayloadChecksum {
		c.publishCommittedGenerationIdentity(generation)
	} else {
		c.publishCommittedGeneration(published, generation)
	}
	result.Published = true
	return result, nil
}
