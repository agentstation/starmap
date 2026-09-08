package starmap

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
