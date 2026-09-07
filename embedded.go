package starmap

import (
	bootstraploader "github.com/agentstation/starmap/internal/bootstrap"
	"github.com/agentstation/starmap/pkg/catalogs"
)

// EmbeddedBuilder returns a catalog builder loaded from the generation
// embedded in this module. Consumers use it to construct catalog fixtures
// without provisioning client storage. Use EmbeddedGeneration for a verified
// manifest and payload without a client or application storage.
func EmbeddedBuilder() (*catalogs.Builder, error) {
	return bootstraploader.NewEmbeddedBuilder()
}

// EmbeddedGeneration returns the verified generation compiled into this module.
// The caller owns its manifest and payload. This function reads no application
// configuration and creates no files, network connections, or runtime workers.
func EmbeddedGeneration() (catalogs.Generation, error) {
	return bootstraploader.Generation()
}
