// Package windows owns the private runtime DACL grant policy.
package windows

import "github.com/agentstation/starmap/pkg/errors"

const (
	windowsSystemSID         = "S-1-5-18"
	windowsAdministratorsSID = "S-1-5-32-544"
	windowsACLAllow          = 0
	windowsACLDeny           = 1
)

// Entry describes one basic native allow or deny entry.
type Entry struct {
	Kind      uint8
	Flags     uint8
	Principal string
	Rights    uint32
}

// Validate restricts grants without computing ordered effective access.
// SYSTEM and Administrators retain their privileged host-management access.
func Validate(account, owner string, present, null bool, entries []Entry, field string) error {
	if account == "" || owner != account {
		return &errors.ValidationError{Field: field, Message: "must belong to the process account. Review the service identity and file owner before retrying"}
	}
	if !present || null {
		return &errors.ValidationError{Field: field, Message: "requires a restrictive DACL. An absent or null DACL grants unrestricted access"}
	}
	for _, entry := range entries {
		switch entry.Kind {
		case windowsACLAllow:
			if entry.Rights != 0 && entry.Principal != account && entry.Principal != windowsSystemSID && entry.Principal != windowsAdministratorsSID {
				return &errors.ValidationError{Field: field, Message: "DACL grants access beyond the process account and privileged host administrators. Review native ACL entries before retrying"}
			}
		case windowsACLDeny:
		default:
			return &errors.ValidationError{Field: field, Message: "DACL contains an unsupported entry type"}
		}
	}
	return nil
}
