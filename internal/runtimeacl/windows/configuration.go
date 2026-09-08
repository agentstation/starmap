package windows

import "github.com/agentstation/starmap/pkg/errors"

// File read rights exclude the directory-specific permission to create children.
const configurationReadRights = ancestorListDirectory | ancestorReadEA | ancestorReadAttributes | ancestorReadControl | ancestorSynchronize | ancestorGenericRead

// ValidateConfiguration permits shared reads but restricts owners and mutation grants.
// A successful native file read must separately establish service read access.
func ValidateConfiguration(account string, descriptor Descriptor, field string) error {
	if account == "" || !trustedAncestorPrincipal(account, descriptor.Owner) {
		return &errors.ValidationError{Field: field, Message: "configuration must belong to the process account or a trusted Windows host principal"}
	}
	if !descriptor.Present || descriptor.Null {
		return &errors.ValidationError{Field: field, Message: "configuration requires a restrictive DACL"}
	}
	for _, entry := range descriptor.Entries {
		if entry.Flags & ^ancestorKnownFlags != 0 {
			return &errors.ValidationError{Field: field, Message: "configuration DACL contains unsupported entry flags"}
		}
		switch entry.Kind {
		case windowsACLDeny:
		case windowsACLAllow:
			if entry.Rights & ^configurationReadRights != 0 && !trustedAncestorPrincipal(account, entry.Principal) {
				return &errors.ValidationError{Field: field, Message: "configuration DACL permits untrusted writes or security changes"}
			}
		default:
			return &errors.ValidationError{Field: field, Message: "configuration DACL contains an unsupported entry type"}
		}
	}
	return nil
}
