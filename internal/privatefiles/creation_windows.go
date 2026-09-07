package privatefiles

import (
	"os"
	"path/filepath"
	"strings"
	"unsafe"

	"golang.org/x/sys/windows"

	"github.com/agentstation/starmap/pkg/errors"
)

func privateWindowsSecurityDescriptor() (*windows.SECURITY_DESCRIPTOR, error) {
	account, err := windowsProcessSID()
	if err != nil {
		return nil, err
	}
	return windows.SecurityDescriptorFromString("O:" + account + "D:P(A;OICI;FA;;;" + account + ")(A;OICI;FA;;;SY)(A;OICI;FA;;;BA)")
}

// CreateDirectory creates missing directories with private access.
func CreateDirectory(directory string) error {
	if err := ValidateAncestors(directory); err != nil {
		return err
	}
	directory = filepath.Clean(directory)
	info, err := os.Lstat(directory)
	if err == nil {
		if !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
			return &errors.ValidationError{Field: "runtime.directory", Message: "must be a real directory"}
		}
		return nil
	}
	if !os.IsNotExist(err) {
		return err
	}
	parent := filepath.Dir(directory)
	if parent == directory {
		return err
	}
	if err := CreateDirectory(parent); err != nil {
		return err
	}
	root, err := os.OpenRoot(parent)
	if err != nil {
		return err
	}
	defer func() { _ = root.Close() }()
	expected, err := root.Stat(".")
	if err != nil {
		return err
	}
	account, err := windowsProcessSID()
	if err != nil {
		return err
	}
	if err := validateWindowsAncestorEntry(parent, expected, account); err != nil {
		return err
	}
	if err := CreateChild(root, filepath.Base(directory)); err != nil {
		if !os.IsExist(err) {
			return err
		}
		info, err := root.Lstat(filepath.Base(directory))
		if err != nil {
			return err
		}
		if !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
			return &errors.ConflictError{Resource: "runtime directory", Message: "another creator selected a different file type"}
		}
	}
	if err := validateWindowsAncestorEntry(parent, expected, account); err != nil {
		return err
	}
	return ValidateAncestors(directory)
}

// CreateChild exclusively creates one private child directory.
func CreateChild(root *os.Root, name string) error {
	file, err := createPrivateWindowsEntry(root, name, true)
	if err != nil {
		return err
	}
	return file.Close()
}

// CreateFile exclusively creates one private regular file.
func CreateFile(root *os.Root, name string) (*os.File, error) {
	return createPrivateWindowsEntry(root, name, false)
}

// createPrivateWindowsEntry sets ownership and a protected DACL during exclusive creation.
// It uses one child name relative to an open directory and cannot replace an existing entry.
func createPrivateWindowsEntry(root *os.Root, name string, directory bool) (*os.File, error) {
	if !filepath.IsLocal(name) || name == "." || strings.ContainsAny(name, "/\\:") || strings.TrimRight(name, " .") != name {
		return nil, &errors.ValidationError{Field: "runtime.private_file", Message: "requires one direct child name"}
	}
	sd, err := privateWindowsSecurityDescriptor()
	if err != nil {
		return nil, err
	}
	parent, err := root.Open(".")
	if err != nil {
		return nil, err
	}
	defer func() { _ = parent.Close() }()
	objectName, err := windows.NewNTUnicodeString(name)
	if err != nil {
		return nil, err
	}
	attributes := windows.OBJECT_ATTRIBUTES{RootDirectory: windows.Handle(parent.Fd()), ObjectName: objectName, Attributes: windows.OBJ_CASE_INSENSITIVE | windows.OBJ_DONT_REPARSE, SecurityDescriptor: sd}
	attributes.Length = uint32(unsafe.Sizeof(attributes))
	options := uint32(windows.FILE_NON_DIRECTORY_FILE | windows.FILE_SYNCHRONOUS_IO_NONALERT | windows.FILE_OPEN_REPARSE_POINT)
	access := uint32(windows.FILE_GENERIC_READ | windows.FILE_GENERIC_WRITE)
	if directory {
		options = windows.FILE_DIRECTORY_FILE | windows.FILE_SYNCHRONOUS_IO_NONALERT | windows.FILE_OPEN_REPARSE_POINT
		access = windows.FILE_LIST_DIRECTORY | windows.FILE_TRAVERSE | windows.READ_CONTROL | windows.SYNCHRONIZE
	}
	var handle windows.Handle
	var status windows.IO_STATUS_BLOCK
	err = windows.NtCreateFile(&handle, access, &attributes, &status, nil, 0, windows.FILE_SHARE_READ|windows.FILE_SHARE_WRITE|windows.FILE_SHARE_DELETE, windows.FILE_CREATE, options, 0, 0)
	if err != nil {
		if ntstatus, ok := err.(windows.NTStatus); ok {
			err = ntstatus.Errno()
		}
		return nil, &os.PathError{Op: "create private runtime entry", Path: name, Err: err}
	}
	return os.NewFile(uintptr(handle), filepath.Join(root.Name(), name)), nil
}
