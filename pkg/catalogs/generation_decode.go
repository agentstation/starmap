package catalogs

import "github.com/agentstation/starmap/pkg/errors"

// DecodeCatalogGeneration verifies exact bytes and matching manifest and payload schemas.
// Only a complete immutable catalog may cross a generation activation boundary.
func DecodeCatalogGeneration(generation Generation) (*Catalog, error) {
	if err := generation.Validate(); err != nil {
		return nil, err
	}
	catalog, err := DecodeCatalogPayload(generation.Payload)
	if err != nil {
		return nil, err
	}
	if catalog.payloadSchemaVersion != generation.Manifest.SchemaVersion {
		return nil, &errors.ValidationError{Field: "catalog_generation.schema_version", Message: "manifest and payload schemas must match"}
	}
	if err := ValidateMembershipEvidence(catalog.MembershipScopes(), generation.Manifest.SourceObservations); err != nil {
		return nil, err
	}
	return catalog, nil
}
