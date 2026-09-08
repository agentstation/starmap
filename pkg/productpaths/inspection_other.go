//go:build !darwin && !linux && !windows

package productpaths

import (
	"io/fs"
	"os"
)

func inspectPermissions(item *FileObservation, _ fs.FileInfo) {
	item.PermissionScope = "native-permissions-unverified"
}

func openInspectionDirectory(root *os.Root, name string) (*os.File, error) {
	return root.Open(name)
}

func inspectionLstat(path string) (os.FileInfo, error) { return os.Lstat(path) }
