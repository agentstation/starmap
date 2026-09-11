package bootstrap

import (
	"encoding/hex"
	"os"

	"golang.org/x/sys/windows"
)

const baselineFileIDBytes = 24

func baselineFileIdentity(file *os.File) (string, error) {
	var data [baselineFileIDBytes]byte
	if err := windows.GetFileInformationByHandleEx(windows.Handle(file.Fd()), windows.FileIdInfo, &data[0], baselineFileIDBytes); err != nil {
		return "", err
	}
	return "windows:" + hex.EncodeToString(data[:]), nil
}
