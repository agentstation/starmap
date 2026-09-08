package workspace

import (
	"os"
	"runtime"

	"golang.org/x/sys/windows"
)

// writableStagedDirectory permits assembly inside the private staging tree.
// Its temporary grant applies only to this directory. Assembly removes it before publication.
func writableStagedDirectory(file *os.File) (func() error, error) {
	original, err := windows.GetSecurityInfo(windows.Handle(file.Fd()), windows.SE_FILE_OBJECT, workspaceWindowsSecurity)
	if err != nil {
		return nil, err
	}
	if original == nil || !original.IsValid() {
		return nil, invalidWorkspaceDescriptor()
	}
	dacl, _, err := original.DACL()
	if err != nil {
		return nil, err
	}
	if dacl == nil {
		return func() error { return nil }, nil
	}
	user, err := windows.GetCurrentProcessToken().GetTokenUser()
	if err != nil {
		return nil, err
	}
	var pinned runtime.Pinner
	pinned.Pin(user.User.Sid)
	defer pinned.Unpin()
	grant := windows.EXPLICIT_ACCESS{
		AccessPermissions: windows.FILE_GENERIC_WRITE,
		AccessMode:        windows.GRANT_ACCESS,
		Inheritance:       windows.NO_INHERITANCE,
		Trustee:           windows.TRUSTEE{TrusteeForm: windows.TRUSTEE_IS_SID, TrusteeType: windows.TRUSTEE_IS_USER, TrusteeValue: windows.TrusteeValueFromSID(user.User.Sid)},
	}
	assemblyACL, err := windows.ACLFromEntries([]windows.EXPLICIT_ACCESS{grant}, dacl)
	if err != nil {
		return nil, err
	}
	assembly, err := original.ToAbsolute()
	if err != nil {
		return nil, err
	}
	if err := assembly.SetDACL(assemblyACL, true, false); err != nil {
		return nil, err
	}
	relative, err := assembly.ToSelfRelative()
	runtime.KeepAlive(assemblyACL)
	if err != nil {
		return nil, err
	}
	if err := assignNativeAccess(file, relative); err != nil {
		return nil, err
	}
	return func() error { return assignNativeAccess(file, original) }, nil
}
