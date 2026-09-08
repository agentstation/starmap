package workspace

import (
	stderrors "errors"
	"os"
	"strings"

	"golang.org/x/sys/windows"
)

// readTargetInfo captures identity through a metadata handle before the read callback runs.
func readTargetInfo(path string) (os.FileInfo, error) {
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
	handle, err := windows.CreateFile(name, windows.FILE_READ_ATTRIBUTES,
		windows.FILE_SHARE_READ|windows.FILE_SHARE_WRITE|windows.FILE_SHARE_DELETE,
		nil, windows.OPEN_EXISTING, windows.FILE_FLAG_BACKUP_SEMANTICS|windows.FILE_FLAG_OPEN_REPARSE_POINT, 0)
	if err != nil {
		return nil, err
	}
	file := os.NewFile(uintptr(handle), path)
	info, statErr := file.Stat()
	return info, stderrors.Join(statErr, file.Close())
}
