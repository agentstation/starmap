//go:build windows

package workspace

import (
	"encoding/hex"
	"os"

	"golang.org/x/sys/windows"
)

// FILE_ID_INFO contains an eight-byte volume serial and a 16-byte file ID.
const directoryFileIDBytes = 24

func entryIdentity(file *os.File) (string, error) {
	var data [directoryFileIDBytes]byte
	if err := windows.GetFileInformationByHandleEx(windows.Handle(file.Fd()), windows.FileIdInfo, &data[0], directoryFileIDBytes); err != nil {
		return "", err
	}
	return "windows:" + hex.EncodeToString(data[:]), nil
}
