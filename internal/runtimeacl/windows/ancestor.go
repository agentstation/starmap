package windows

import "github.com/agentstation/starmap/pkg/errors"

const (
	windowsInstallerSID                   = "S-1-5-80-956008885-3418522649-1831038044-1853292631-2271478464"
	ancestorListDirectory          uint32 = 0x1
	ancestorAddSubdirectory        uint32 = 0x4
	ancestorReadEA                 uint32 = 0x8
	ancestorTraverse               uint32 = 0x20
	ancestorReadAttributes         uint32 = 0x80
	ancestorReadControl            uint32 = 0x20000
	ancestorSynchronize            uint32 = 0x100000
	ancestorGenericRead            uint32 = 0x80000000
	ancestorGenericExecute         uint32 = 0x20000000
	ancestorReadAndCreateDirectory        = ancestorListDirectory | ancestorAddSubdirectory | ancestorReadEA | ancestorTraverse | ancestorReadAttributes | ancestorReadControl | ancestorSynchronize | ancestorGenericRead | ancestorGenericExecute
	ancestorInheritOnly            uint8  = 0x08
	ancestorKnownFlags             uint8  = 0x1f
)

// ValidateAncestor checks ownership and mutation grants on a path ancestor.
// Shared reads, traversal, and subdirectory creation remain valid.
// Child ownership checks and protected private creation remain the caller's responsibility.
func ValidateAncestor(account, owner string, present, null bool, entries []Entry, field string) error {
	if account == "" || !trustedAncestorPrincipal(account, owner) {
		return &errors.ValidationError{Field: field, Message: "ancestor must belong to the process account or a trusted Windows host principal"}
	}
	if !present || null {
		return &errors.ValidationError{Field: field, Message: "ancestor requires a restrictive DACL. Review its native permissions before retrying"}
	}
	for _, entry := range entries {
		if entry.Flags & ^ancestorKnownFlags != 0 {
			return &errors.ValidationError{Field: field, Message: "ancestor DACL contains unsupported entry flags"}
		}
		switch entry.Kind {
		case windowsACLDeny:
		case windowsACLAllow:
			if entry.Flags&ancestorInheritOnly != 0 {
				continue
			}
			if entry.Rights & ^ancestorReadAndCreateDirectory != 0 && !trustedAncestorPrincipal(account, entry.Principal) {
				return &errors.ValidationError{Field: field, Message: "ancestor DACL permits another account to change directory entries or security. Review native ancestor grants before retrying"}
			}
		default:
			return &errors.ValidationError{Field: field, Message: "ancestor DACL contains an unsupported entry type"}
		}
	}
	return nil
}

func trustedAncestorPrincipal(account, principal string) bool {
	return principal == account || principal == windowsSystemSID || principal == windowsAdministratorsSID || principal == windowsInstallerSID
}
