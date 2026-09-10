package catalogs

// validateAuthorityBinding keeps permission identity and catalog identity in one committed manifest.
func (m GenerationManifest) validateAuthorityBinding() error {
	switch m.ManifestVersion {
	case CurrentGenerationManifestVersion:
		if m.AuthorityHead != (CatalogAuthorityHead{}) {
			return validationError("authority_head", nil, "requires the authority manifest version")
		}
	case AuthorityGenerationManifestVersion:
		if err := m.AuthorityHead.Validate(); err != nil {
			return err
		}
		if m.AuthorityHead.GenerationID != m.GenerationID {
			return validationError("authority_head.generation_id", nil, "must match the committed catalog generation")
		}
		if m.AuthorityHead.PayloadChecksum != m.Payload.Checksum {
			return validationError("authority_head.payload_checksum", nil, "must match the committed catalog payload")
		}
	default:
		return validationError("manifest_version", m.ManifestVersion, "must name a supported catalog manifest version")
	}
	return nil
}
