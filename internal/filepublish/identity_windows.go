package filepublish

import (
	"encoding/hex"
	"os"
	"runtime"

	"golang.org/x/sys/windows"
)

const nativeFileIDBytes = 24

// windowsFileIDBuffer holds FILE_ID_INFO bytes with native uint64 alignment.
// The zero-length field aligns the buffer without converting pointers.
type windowsFileIDBuffer struct {
	_    [0]uint64
	data [nativeFileIDBytes]byte
}

// Identity returns the native volume and file identity for an open handle.
func Identity(file *os.File) (string, error) {
	var info windowsFileIDBuffer
	err := windows.GetFileInformationByHandleEx(windows.Handle(file.Fd()), windows.FileIdInfo,
		&info.data[0], nativeFileIDBytes)
	runtime.KeepAlive(file)
	if err != nil {
		return "", &os.PathError{Op: "query native file identity", Path: file.Name(), Err: err}
	}
	return "windows:" + hex.EncodeToString(info.data[:]), nil
}
