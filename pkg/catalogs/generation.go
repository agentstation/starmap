package catalogs

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
