package productpaths

import (
	"io/fs"
	"os"
	"strings"

	"golang.org/x/sys/windows"

	aclpolicy "github.com/agentstation/starmap/internal/runtimeacl/windows"
)

func inspectPermissions(item *FileObservation, expected fs.FileInfo) {
	item.PermissionScope = "windows-owner-and-dacl"
	item.WindowsSecurity = &WindowsSecurity{DACLState: "unverified", PolicyStatus: "unverified", Reason: "security-descriptor-unavailable"}
	if expected.Mode()&os.ModeSymlink != 0 || (!expected.IsDir() && !expected.Mode().IsRegular()) {
		item.WindowsSecurity.Reason = "unsupported-file-type"
		return
	}
	file, err := openWindowsInspectionMetadata(item.Path)
	if err != nil {
		return
	}
	defer func() { _ = file.Close() }()
	if !sameWindowsInspectionFile(file, item.Path, expected) {
		item.WindowsSecurity.Reason = "changed-during-inspection"
		return
	}
	var info windows.ByHandleFileInformation
	if err := windows.GetFileInformationByHandle(windows.Handle(file.Fd()), &info); err != nil {
		return
	}
	if info.FileAttributes&windows.FILE_ATTRIBUTE_REPARSE_POINT != 0 {
		item.WindowsSecurity.Reason = "unsupported-reparse-point"
		return
	}
	sd, err := windows.GetSecurityInfo(windows.Handle(file.Fd()), windows.SE_FILE_OBJECT, windows.OWNER_SECURITY_INFORMATION|windows.DACL_SECURITY_INFORMATION)
	if err != nil {
		return
	}
	if !sameWindowsInspectionFile(file, item.Path, expected) {
		item.WindowsSecurity.Reason = "changed-during-inspection"
		return
	}
	descriptor, err := aclpolicy.DecodeDescriptor(sd, "inspection")
	if err != nil {
		item.WindowsSecurity.Reason = "unsupported-security-descriptor"
		return
	}
	account, err := windows.GetCurrentProcessToken().GetTokenUser()
	if err != nil || account == nil || account.User.Sid == nil || !account.User.Sid.IsValid() {
		item.WindowsSecurity.Reason = "process-owner-unavailable"
		return
	}
	item.WindowsSecurity = observeWindowsDescriptor(account.User.Sid.String(), descriptor)
}

func openWindowsInspectionMetadata(path string) (*os.File, error) {
	native := strings.ReplaceAll(path, "/", `\`)
	if !strings.HasPrefix(native, `\\?\`) {
		if strings.HasPrefix(native, `\\`) {
			native = `\\?\UNC\` + strings.TrimPrefix(native, `\\`)
		} else {
			native = `\\?\` + native
		}
	}
	name, err := windows.UTF16PtrFromString(native)
	if err != nil {
		return nil, err
	}
	handle, err := windows.CreateFile(name, windows.READ_CONTROL|windows.FILE_READ_ATTRIBUTES, windows.FILE_SHARE_READ|windows.FILE_SHARE_WRITE|windows.FILE_SHARE_DELETE, nil, windows.OPEN_EXISTING, windows.FILE_FLAG_BACKUP_SEMANTICS|windows.FILE_FLAG_OPEN_REPARSE_POINT, 0)
	if err != nil {
		return nil, err
	}
	return os.NewFile(uintptr(handle), path), nil
}

func sameWindowsInspectionFile(file *os.File, path string, expected fs.FileInfo) bool {
	actual, err := file.Stat()
	if err != nil || !os.SameFile(expected, actual) {
		return false
	}
	selected, err := os.Lstat(path)
	return err == nil && selected.Mode()&os.ModeSymlink == 0 && os.SameFile(expected, selected)
}

func openInspectionDirectory(root *os.Root, name string) (*os.File, error) {
	return root.Open(name)
}

func observeWindowsDescriptor(account string, descriptor aclpolicy.Descriptor) *WindowsSecurity {
	assessment := aclpolicy.Assess(account, descriptor)
	result := &WindowsSecurity{OwnerSID: descriptor.Owner, ProcessSID: account, DACLState: assessment.DACLState, EntryCount: assessment.EntryCount, PolicyStatus: assessment.Status, Reason: assessment.Reason, ServicePolicyStatus: "compatible"}
	if aclpolicy.ValidateConfiguration(account, descriptor, "inspection") != nil {
		result.ServicePolicyStatus = "conflict"
		result.ServicePolicyReason = "native-service-policy-conflict"
	}
	return result
}
