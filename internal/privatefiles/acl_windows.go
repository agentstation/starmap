package privatefiles

import (
	"io/fs"
	"os"

	"golang.org/x/sys/windows"

	aclpolicy "github.com/agentstation/starmap/internal/runtimeacl/windows"
	"github.com/agentstation/starmap/pkg/errors"
)

func windowsProcessSID() (string, error) {
	user, err := windows.GetCurrentProcessToken().GetTokenUser()
	if err != nil {
		return "", err
	}
	if user.User.Sid == nil || !user.User.Sid.IsValid() {
		return "", &errors.ConfigError{Component: "runtime.owner", Message: "process account SID is unavailable"}
	}
	return user.User.Sid.String(), nil
}

// ValidateACL checks native grants on the expected file without changing its ACL.
func ValidateACL(root *os.Root, name string, expected fs.FileInfo, field string) error {
	file, err := root.Open(name)
	if err != nil {
		return errors.WrapResource("inspect", "runtime ACL", field, err)
	}
	defer func() { _ = file.Close() }()
	actual, err := file.Stat()
	if err != nil {
		return err
	}
	current, err := root.Lstat(name)
	if err != nil || !os.SameFile(expected, actual) || !os.SameFile(expected, current) || current.Mode()&os.ModeSymlink != 0 {
		return &errors.ConflictError{Resource: field, Message: "file changed during ACL inspection"}
	}
	account, err := windowsProcessSID()
	if err != nil {
		return err
	}
	sd, err := windows.GetSecurityInfo(windows.Handle(file.Fd()), windows.SE_FILE_OBJECT, windows.OWNER_SECURITY_INFORMATION|windows.DACL_SECURITY_INFORMATION)
	if err != nil {
		return errors.WrapResource("read", "runtime security descriptor", field, err)
	}
	return validateWindowsSecurityDescriptor(sd, account, field)
}

func validateWindowsSecurityDescriptor(sd *windows.SECURITY_DESCRIPTOR, account, field string) error {
	descriptor, err := decodeWindowsSecurityDescriptor(sd, field)
	if err != nil {
		return err
	}
	return aclpolicy.Validate(account, descriptor.Owner, descriptor.Present, descriptor.Null, descriptor.Entries, field)
}

func decodeWindowsSecurityDescriptor(sd *windows.SECURITY_DESCRIPTOR, field string) (aclpolicy.Descriptor, error) {
	descriptor, err := aclpolicy.DecodeDescriptor(sd, field)
	if err == nil && !descriptor.Present {
		return descriptor, errors.WrapResource("read", "runtime DACL", field, windows.ERROR_OBJECT_NOT_FOUND)
	}
	return descriptor, err
}
