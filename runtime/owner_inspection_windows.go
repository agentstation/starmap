package runtime

import (
	"os"
	"path/filepath"
	"unsafe"

	"golang.org/x/sys/windows"
)

const ownerRecordInspectionSupported = true

func openOwnerRecordForInspection(root *os.Root) (*os.File, error) {
	parent, err := root.Open(".")
	if err != nil {
		return nil, err
	}
	defer func() { _ = parent.Close() }()
	name, err := windows.NewNTUnicodeString(ownerRecordName)
	if err != nil {
		return nil, err
	}
	attributes := windows.OBJECT_ATTRIBUTES{
		RootDirectory: windows.Handle(parent.Fd()), ObjectName: name,
		Attributes: windows.OBJ_CASE_INSENSITIVE | windows.OBJ_DONT_REPARSE,
	}
	attributes.Length = uint32(unsafe.Sizeof(attributes))
	var handle windows.Handle
	var status windows.IO_STATUS_BLOCK
	err = windows.NtCreateFile(&handle, windows.FILE_GENERIC_READ, &attributes, &status, nil, 0,
		windows.FILE_SHARE_READ|windows.FILE_SHARE_WRITE|windows.FILE_SHARE_DELETE, windows.FILE_OPEN,
		windows.FILE_NON_DIRECTORY_FILE|windows.FILE_OPEN_REPARSE_POINT|windows.FILE_SYNCHRONOUS_IO_NONALERT, 0, 0)
	if err != nil {
		if native, ok := err.(windows.NTStatus); ok {
			err = native.Errno()
		}
		return nil, &os.PathError{Op: "inspect runtime owner", Path: ownerRecordName, Err: err}
	}
	return os.NewFile(uintptr(handle), filepath.Join(root.Name(), ownerRecordName)), nil
}
