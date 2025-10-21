//nolint:revive // Package types provides common type definitions
package types

import "slices"

// SourceID identifies a data source in the synchronization pipeline.
// It uniquely identifies where catalog data originates from (providers, models.dev, local files, etc.).
type SourceID string

// String returns the string representation of a source ID.
func (id SourceID) String() string {
	return string(id)
}

// Common source identifiers used throughout the system.
const (
	// ProvidersID identifies the provider API source (OpenAI, Anthropic, etc.).
	ProvidersID SourceID = "providers"

	// ModelsDevGitID identifies the models.dev Git repository source.
	ModelsDevGitID SourceID = "models_dev_git"

	// ModelsDevHTTPID identifies the models.dev HTTP API source.
	ModelsDevHTTPID SourceID = "models_dev_http"

	// LocalCatalogID identifies the local filesystem catalog source.
	LocalCatalogID SourceID = "local_catalog"
)

// SourceIDs returns all available source identifiers.
// This provides a convenient way to iterate over all defined source IDs.
func SourceIDs() []SourceID {
	return []SourceID{
		ProvidersID,
		ModelsDevGitID,
		ModelsDevHTTPID,
		LocalCatalogID,
	}
}

// IsValid returns true if the SourceID is one of the defined constants.
// Uses SourceIDs() to ensure consistency with the authoritative list.
func (id SourceID) IsValid() bool {
	return slices.Contains(SourceIDs(), id)
}
