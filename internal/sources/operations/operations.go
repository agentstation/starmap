package operations

import (
	"github.com/agentstation/starmap/internal/sources/registry"
	"github.com/agentstation/starmap/pkg/catalogs"
	"github.com/agentstation/starmap/pkg/sources"
)

// PostSyncOperations handles operations after sync completes
type PostSyncOperations interface {
	CopyProviderLogos(providerIDs []catalogs.ProviderID) error
	Cleanup() error
}

// GetPostSyncOperations returns post-sync operations for a source type
func GetPostSyncOperations(sourceType sources.Type) PostSyncOperations {
	source, ok := registry.GetSource(sourceType)
	if !ok {
		return nil
	}

	if ops, ok := source.(PostSyncOperations); ok {
		return ops
	}
	return nil
}
