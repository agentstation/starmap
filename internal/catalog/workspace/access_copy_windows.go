package workspace

import (
	"os"
	"path/filepath"
	"runtime"
	"unsafe"

	"golang.org/x/sys/windows"
)

var ntSetWorkspaceSecurity = windows.NewLazySystemDLL("ntdll.dll").NewProc("NtSetSecurityObject")

func copyNativeAccess(source, destination *os.File) error {
	sd, err := windows.GetSecurityInfo(windows.Handle(source.Fd()), windows.SE_FILE_OBJECT,
		workspaceWindowsSecurity)
	if err != nil {
		return err
	}
	if sd == nil || !sd.IsValid() {
		return invalidWorkspaceDescriptor()
	}
	if err := assignNativeAccess(destination, sd); err != nil {
		return err
	}
	info, err := source.Stat()
	if err != nil {
		return err
	}
	return destination.Chmod(info.Mode() & workspaceAccessMode)
}

func assignNativeAccess(destination *os.File, sd *windows.SECURITY_DESCRIPTOR) error {
	control, _, err := sd.Control()
	if err != nil {
		return err
	}
	// Native assignment needs request bits to retain the automatic-inheritance model.
	var inheritance windows.SECURITY_DESCRIPTOR_CONTROL
	if control&windows.SE_DACL_AUTO_INHERITED != 0 {
		inheritance |= windows.SE_DACL_AUTO_INHERIT_REQ
	}
	if control&windows.SE_SACL_AUTO_INHERITED != 0 {
		inheritance |= windows.SE_SACL_AUTO_INHERIT_REQ
	}
	if err := sd.SetControl(windows.SE_DACL_AUTO_INHERIT_REQ|windows.SE_SACL_AUTO_INHERIT_REQ, inheritance); err != nil {
		return err
	}
	// Central policy assignment needs a privileged handle. Snapshot equality still checks it.
	flags := workspaceWindowsSecurity &^ windows.SCOPE_SECURITY_INFORMATION
	if control&windows.SE_SACL_PRESENT == 0 {
		flags &^= windows.LABEL_SECURITY_INFORMATION | windows.ATTRIBUTE_SECURITY_INFORMATION
	}
	if control&windows.SE_DACL_PROTECTED != 0 {
		flags |= windows.PROTECTED_DACL_SECURITY_INFORMATION
	} else {
		flags |= windows.UNPROTECTED_DACL_SECURITY_INFORMATION
	}
	if err := ntSetWorkspaceSecurity.Find(); err != nil {
		return err
	}
	// Assign the captured descriptor without importing grants from the staging parent.
	status, _, _ := ntSetWorkspaceSecurity.Call(destination.Fd(), uintptr(flags), uintptr(unsafe.Pointer(sd))) //nolint:gosec // G103: The validated self-relative descriptor stays live until the synchronous call returns.
	runtime.KeepAlive(sd)
	if status != 0 {
		return windows.NTStatus(status & 0xffffffff).Errno()
	}
	return nil
}

func openStagedDirectory(root *os.Root, name string) (*os.File, error) {
	return openWindowsStagedEntry(root, name, true, false)
}

func createStagedFile(root *os.Root, name string) (*os.File, error) {
	return openWindowsStagedEntry(root, name, false, true)
}

func privateStageAccess(_ *os.File) error { return nil }

func openWindowsStagedEntry(root *os.Root, name string, directory, create bool) (*os.File, error) {
	parent, err := root.Open(".")
	if err != nil {
		return nil, err
	}
	defer func() { _ = parent.Close() }()
	selected := name
	if selected == "." {
		selected = ""
	}
	nativeName, err := windows.NewNTUnicodeString(filepath.FromSlash(selected))
	if err != nil {
		return nil, err
	}
	attributes := windows.OBJECT_ATTRIBUTES{RootDirectory: windows.Handle(parent.Fd()), ObjectName: nativeName, Attributes: windows.OBJ_CASE_INSENSITIVE | windows.OBJ_DONT_REPARSE}
	attributes.Length = uint32(unsafe.Sizeof(attributes))
	options := uint32(windows.FILE_NON_DIRECTORY_FILE | windows.FILE_SYNCHRONOUS_IO_NONALERT | windows.FILE_OPEN_REPARSE_POINT)
	if directory {
		options = windows.FILE_DIRECTORY_FILE | windows.FILE_SYNCHRONOUS_IO_NONALERT | windows.FILE_OPEN_REPARSE_POINT
	}
	disposition := uint32(windows.FILE_OPEN)
	if create {
		disposition = windows.FILE_CREATE
	}
	var handle windows.Handle
	var status windows.IO_STATUS_BLOCK
	err = windows.NtCreateFile(&handle, windows.FILE_GENERIC_READ|windows.FILE_GENERIC_WRITE|windows.WRITE_DAC|windows.WRITE_OWNER, &attributes, &status, nil, 0,
		windows.FILE_SHARE_READ|windows.FILE_SHARE_WRITE|windows.FILE_SHARE_DELETE, disposition, options, 0, 0)
	if err != nil {
		if native, ok := err.(windows.NTStatus); ok {
			err = native.Errno()
		}
		return nil, err
	}
	return os.NewFile(uintptr(handle), filepath.Join(root.Name(), name)), nil
}
