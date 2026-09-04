package catalogs

import "github.com/agentstation/starmap/pkg/errors"

// Generation is an immutable manifest and its exact catalog payload bytes.
type Generation struct {
	Manifest GenerationManifest
	Payload  []byte
}

// Copy returns a generation that does not share mutable slices with g.
func (g Generation) Copy() Generation {
	return Generation{
		Manifest: g.Manifest.Copy(),
		Payload:  append([]byte(nil), g.Payload...),
	}
}

// Validate verifies the manifest and its binding to the payload.
func (g Generation) Validate() error {
	if err := g.Manifest.Validate(); err != nil {
		return err
	}
	if err := g.Manifest.Payload.Verify(g.Payload); err != nil {
		return err
	}
	return nil
}

// SemanticChecksum returns the facts-only identity of the catalog the payload
// carries. It excludes provenance, so a regenerated payload with the same facts
// keeps the same value. The publisher keys the immutable release tag and the
// channel catalog digest by this value. The exact payload checksum stays in
// the manifest.
func (g Generation) SemanticChecksum() (string, error) {
	catalog, err := DecodeCatalogPayload(g.Payload)
	if err != nil {
		return "", errors.WrapResource(
			"decode", "catalog generation semantics", g.Manifest.GenerationID, err)
	}
	checksum, err := CatalogSemanticChecksum(catalog)
	if err != nil {
		return "", errors.WrapResource(
			"encode", "catalog generation semantics", g.Manifest.GenerationID, err)
	}
	return checksum, nil
}
