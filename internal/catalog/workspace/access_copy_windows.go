package workspace

import (
	stderrors "errors"
	"os"
	"path/filepath"
	"unsafe"

	"golang.org/x/sys/windows"
)

func copyNativeAccess(source, destination *os.File) error {
	sd, err := windows.GetSecurityInfo(windows.Handle(source.Fd()), windows.SE_FILE_OBJECT,
		workspaceWindowsSecurity)
	if err != nil {
		return err
	}
	owner, _, err := sd.Owner()
	if err != nil {
		return err
	}
	group, _, err := sd.Group()
	if err != nil {
		return err
	}
	dacl, _, err := sd.DACL()
	if err != nil {
		return err
	}
	control, _, err := sd.Control()
	if err != nil {
		return err
	}
	sacl, _, err := sd.SACL()
	if err != nil && !stderrors.Is(err, windows.ERROR_OBJECT_NOT_FOUND) {
		return err
	}
	flags := windows.SECURITY_INFORMATION(windows.OWNER_SECURITY_INFORMATION | windows.GROUP_SECURITY_INFORMATION | windows.DACL_SECURITY_INFORMATION | windows.LABEL_SECURITY_INFORMATION | windows.ATTRIBUTE_SECURITY_INFORMATION)
	if control&windows.SE_DACL_PROTECTED != 0 {
		flags |= windows.PROTECTED_DACL_SECURITY_INFORMATION
	} else {
		current, err := windows.GetSecurityInfo(windows.Handle(destination.Fd()), windows.SE_FILE_OBJECT, windows.DACL_SECURITY_INFORMATION)
		if err != nil {
			return err
		}
		currentControl, _, err := current.Control()
		if err != nil {
			return err
		}
		// Re-enabling inheritance can import grants from the private preparation parent.
		if currentControl&windows.SE_DACL_PROTECTED != 0 {
			flags |= windows.UNPROTECTED_DACL_SECURITY_INFORMATION
		}
	}
	if err := windows.SetSecurityInfo(windows.Handle(destination.Fd()), windows.SE_FILE_OBJECT, flags, owner, group, dacl, sacl); err != nil {
		return err
	}
	info, err := source.Stat()
	if err != nil {
		return err
	}
	return destination.Chmod(info.Mode() & workspaceAccessMode)
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
