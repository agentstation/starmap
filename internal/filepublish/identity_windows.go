package filepublish

import (
	"encoding/hex"
	"os"

	"golang.org/x/sys/windows"
)

const nativeFileIDBytes = 24

// Identity returns the native volume and file identity for an open handle.
func Identity(file *os.File) (string, error) {
	var data [nativeFileIDBytes]byte
	if err := windows.GetFileInformationByHandleEx(windows.Handle(file.Fd()), windows.FileIdInfo, &data[0], nativeFileIDBytes); err != nil {
		return "", err
	}
	return "windows:" + hex.EncodeToString(data[:]), nil
}
