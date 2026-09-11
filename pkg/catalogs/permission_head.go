package catalogs

// CatalogPermissionSchemaVersion identifies the permission semantics this reader enforces.
const CatalogPermissionSchemaVersion uint64 = 1

const maxPermissionIdentityBytes = 256

var catalogAuthorityHeadJSONFields = []string{
	"authority_id", "policy_id", "sequence", "generation_id", "payload_checksum",
	"required_permission_revision", "permission_schema_version",
}

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
	if err := ValidateCatalogAuthorityIdentity(h.AuthorityID, h.PolicyID); err != nil {
		return err
	}
	if err := validatePermissionIdentity("generation_id", h.GenerationID); err != nil {
		return err
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

// ValidateCatalogAuthorityIdentity checks the bounded authority and policy names used by publishers and subscribers.
func ValidateCatalogAuthorityIdentity(authorityID, policyID string) error {
	if err := validatePermissionIdentity("authority_id", authorityID); err != nil {
		return err
	}
	return validatePermissionIdentity("policy_id", policyID)
}

func validatePermissionIdentity(field, value string) error {
	if len(value) > maxPermissionIdentityBytes || !validMembershipIdentifier(value) {
		return validationError("permission."+field, nil, "must be a bounded nonempty identity without surrounding whitespace or control characters")
	}
	return nil
}
