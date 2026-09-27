package starmap

import (
	"github.com/agentstation/starmap/pkg/catalogs"
	"github.com/agentstation/starmap/pkg/errors"
)

// EmbeddedCatalogState returns the verified baseline compiled into this module.
// It remains independent of stored generations, workspace input, and later updates.
// The immutable state requires no storage reads or payload decoding.
func (c *Client) EmbeddedCatalogState() CatalogState {
	if c == nil {
		return CatalogState{}
	}
	return CatalogState{
		Catalog:         c.embeddedCatalog,
		GenerationID:    c.embeddedBootstrap.GenerationID,
		PayloadChecksum: c.embeddedBootstrap.Payload.Checksum,
		GeneratedAt:     c.embeddedBootstrap.GeneratedAt,
		Sequence:        1,
	}
}

// decodeStoredCatalog reuses verified embedded facts when validated stored bytes
// have the same digest. The caller validates the stored manifest and bytes first.
// Stored schema and membership evidence still require their own checks.
func decodeStoredCatalog(stored catalogs.Generation, embedded *catalogs.Catalog, baseline catalogs.BootstrapManifest) (*catalogs.Catalog, error) {
	if stored.Manifest.Payload.Checksum != baseline.Payload.Checksum {
		return catalogs.DecodeCatalogGeneration(stored)
	}
	if stored.Manifest.SchemaVersion != baseline.SchemaVersion {
		return nil, &errors.ValidationError{Field: "catalog_generation.schema_version", Message: "manifest and payload schemas must match"}
	}
	if err := catalogs.ValidateMembershipEvidence(embedded.MembershipScopes(), stored.Manifest.SourceObservations); err != nil {
		return nil, err
	}
	return embedded, nil
}
