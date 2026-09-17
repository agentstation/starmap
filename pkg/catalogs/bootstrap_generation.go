package catalogs

import (
	"github.com/agentstation/starmap/pkg/errors"
)

// BootstrapGenerationManifestFilename contains the complete manifest of a promoted generation.
const BootstrapGenerationManifestFilename = "generation-manifest.json"

// DecodeBootstrapGeneration verifies retained generation evidence against the bootstrap identity and payload.
func DecodeBootstrapGeneration(bootstrap BootstrapManifest, payload, data []byte) (Generation, error) {
	if err := bootstrap.Validate(); err != nil {
		return Generation{}, err
	}
	committed, err := ParseGenerationManifestJSON(data)
	if err != nil {
		return Generation{}, err
	}
	if committed.GenerationID != bootstrap.GenerationID || !committed.GeneratedAt.Equal(bootstrap.GeneratedAt) ||
		committed.SchemaVersion != bootstrap.SchemaVersion || committed.Payload != bootstrap.Payload {
		return Generation{}, &errors.ValidationError{
			Field: "bootstrap_manifest.committed_generation", Message: "does not match the embedded bootstrap identity",
		}
	}
	generation := Generation{Manifest: committed, Payload: payload}
	catalog, err := DecodeCatalogGeneration(generation)
	if err != nil {
		return Generation{}, err
	}
	semantic, err := CatalogSemanticChecksum(catalog)
	if err != nil {
		return Generation{}, err
	}
	if semantic != bootstrap.SemanticChecksum {
		return Generation{}, &errors.ValidationError{
			Field: "bootstrap_manifest.semantic_checksum", Message: "does not match the committed catalog facts",
		}
	}
	return generation, nil
}
