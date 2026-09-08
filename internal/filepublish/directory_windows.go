package filepublish

import (
	"encoding/binary"
	"math"
	"os"
	"unsafe"

	"golang.org/x/sys/windows"
)

func renameDirectory(parent *os.Root, source string, destination *os.Root, target string) error {
	directory, err := parent.Open(".")
	if err != nil {
		return err
	}
	defer func() { _ = directory.Close() }()
	parentHandle := windows.Handle(directory.Fd())
	name, err := windows.NewNTUnicodeString(source)
	if err != nil {
		return err
	}
	attributes := windows.OBJECT_ATTRIBUTES{
		RootDirectory: parentHandle, ObjectName: name,
		Attributes: windows.OBJ_CASE_INSENSITIVE | windows.OBJ_DONT_REPARSE,
	}
	attributes.Length = uint32(unsafe.Sizeof(attributes))
	var handle windows.Handle
	var status windows.IO_STATUS_BLOCK
	err = windows.NtCreateFile(&handle, windows.DELETE|windows.SYNCHRONIZE, &attributes, &status, nil, 0,
		windows.FILE_SHARE_READ|windows.FILE_SHARE_WRITE|windows.FILE_SHARE_DELETE, windows.FILE_OPEN,
		windows.FILE_DIRECTORY_FILE|windows.FILE_OPEN_REPARSE_POINT|windows.FILE_SYNCHRONOUS_IO_NONALERT, 0, 0)
	if err != nil {
		return nativeError(err)
	}
	defer func() { _ = windows.CloseHandle(handle) }()
	to, err := destination.Open(".")
	if err != nil {
		return err
	}
	defer func() { _ = to.Close() }()
	buffer, err := renameInformation(windows.Handle(to.Fd()), target)
	if err != nil {
		return err
	}
	bufferSize := len(buffer)
	if bufferSize < 1 || bufferSize > math.MaxUint32 {
		return windows.ERROR_FILENAME_EXCED_RANGE
	}
	return nativeError(windows.NtSetInformationFile(handle, &status, &buffer[0], uint32(bufferSize), windows.FileRenameInformation))
}

// renameInformation encodes FILE_RENAME_INFORMATION with ReplaceIfExists set to false.
// Native field offsets retain ABI alignment without casting a byte buffer to a pointer.
func renameInformation(parent windows.Handle, target string) ([]byte, error) {
	name, err := windows.UTF16FromString(target)
	if err != nil {
		return nil, err
	}
	if len(name) > windows.MAX_LONG_PATH {
		return nil, windows.ERROR_FILENAME_EXCED_RANGE
	}
	var layout struct {
		replace bool
		root    windows.Handle
		length  uint32
		name    [1]uint16
	}
	buffer := make([]byte, int(unsafe.Sizeof(layout))+len(name)*2)
	rootOffset := int(unsafe.Offsetof(layout.root))
	if unsafe.Sizeof(parent) == 8 {
		binary.LittleEndian.PutUint64(buffer[rootOffset:], uint64(parent))
	} else {
		if parent > math.MaxUint32 {
			return nil, windows.ERROR_INVALID_HANDLE
		}
		binary.LittleEndian.PutUint32(buffer[rootOffset:], uint32(parent))
	}
	nameBytes := (len(name) - 1) * 2
	if nameBytes < 0 || nameBytes > math.MaxUint32 {
		return nil, windows.ERROR_FILENAME_EXCED_RANGE
	}
	binary.LittleEndian.PutUint32(buffer[unsafe.Offsetof(layout.length):], uint32(nameBytes))
	for i, unit := range name {
		binary.LittleEndian.PutUint16(buffer[int(unsafe.Offsetof(layout.name))+i*2:], unit)
	}
	return buffer, nil
}

func nativeError(err error) error {
	if status, ok := err.(windows.NTStatus); ok {
		return status.Errno()
	}
	return err
}
