package catalogs

// CatalogPermissionSchemaVersion identifies the permission semantics this reader enforces.
const CatalogPermissionSchemaVersion uint64 = 1

const maxPermissionIdentityBytes = 256

// CatalogAuthorityHead binds one authority publication to its catalog and required permission revision.
// The authority commits these fields together. Validation does not authenticate the authority.
type CatalogAuthorityHead struct {
	AuthorityID                string `json:"authority_id"`
	PolicyID                   string `json:"policy_id"`
	Sequence                   uint64 `json:"sequence"`
	GenerationID               string `json:"generation_id"`
	PayloadChecksum            string `json:"payload_checksum"`
	RequiredPermissionRevision string `json:"required_permission_revision"`
	PermissionSchemaVersion    uint64 `json:"permission_schema_version"`
}

// Validate checks publication identity and digest shape independently of catalog payload compatibility.
// Unknown positive permission versions remain readable so the consumer can record the required revision before refusing admission.
func (h CatalogAuthorityHead) Validate() error {
	for _, field := range []struct{ name, value string }{
		{"authority_id", h.AuthorityID}, {"policy_id", h.PolicyID}, {"generation_id", h.GenerationID},
	} {
		if len(field.value) > maxPermissionIdentityBytes || !validMembershipIdentifier(field.value) {
			return validationError("permission."+field.name, nil, "must be a bounded nonempty identity without surrounding whitespace or control characters")
		}
	}
	if h.Sequence == 0 {
		return validationError("permission.sequence", nil, "must be positive")
	}
	if h.PermissionSchemaVersion == 0 {
		return validationError("permission.permission_schema_version", nil, "must be positive")
	}
	if err := validateChecksum("permission.payload_checksum", h.PayloadChecksum); err != nil {
		return err
	}
	return validateChecksum("permission.required_permission_revision", h.RequiredPermissionRevision)
}

// SupportsPermissions reports whether this reader understands every mandatory permission semantic.
// A compatible catalog payload cannot override an unsupported permission version.
func (h CatalogAuthorityHead) SupportsPermissions() bool {
	return h.PermissionSchemaVersion == CatalogPermissionSchemaVersion
}

// ValidateSuccessor rejects a different authority, an older sequence, or changed content under the same sequence.
// The caller authorizes authority and policy changes in a new context.
func (h CatalogAuthorityHead) ValidateSuccessor(next CatalogAuthorityHead) error {
	if err := h.Validate(); err != nil {
		return err
	}
	if err := next.Validate(); err != nil {
		return err
	}
	if h.AuthorityID != next.AuthorityID || h.PolicyID != next.PolicyID {
		return validationError("permission.authority", nil, "requires an explicit authority or policy transition")
	}
	if next.Sequence < h.Sequence {
		return validationError("permission.sequence", nil, "cannot precede the highest accepted publication")
	}
	if next.Sequence == h.Sequence && next != h {
		return validationError("permission.sequence", nil, "already identifies different publication content")
	}
	return nil
}
