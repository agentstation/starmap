package starmap

import (
	bootstraploader "github.com/agentstation/starmap/internal/bootstrap"
	"github.com/agentstation/starmap/pkg/catalogs"
)

// EmbeddedBuilder returns a catalog builder loaded from the generation
// embedded in this module. Consumers use it to construct catalog fixtures
// without provisioning client storage. Callers that need the verified
// immutable generation with durable storage should construct a Client.
func EmbeddedBuilder() (*catalogs.Builder, error) {
	return bootstraploader.NewEmbeddedBuilder()
}
