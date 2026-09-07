//go:build darwin || linux

package productpaths

import (
	"fmt"
	"io/fs"
	"os"
	"syscall"
)

func inspectPermissions(item *FileObservation, info fs.FileInfo) {
	item.Mode = fmt.Sprintf("%04o", info.Mode().Perm())
	item.PermissionScope = "posix-mode-bits-only"
	if stat, ok := info.Sys().(*syscall.Stat_t); ok {
		item.Owner = &FileOwner{UID: stat.Uid, GID: stat.Gid, MatchesEffectiveUser: int64(stat.Uid) == int64(os.Geteuid())}
	}
}

func openInspectionDirectory(root *os.Root, name string) (*os.File, error) {
	return root.OpenFile(name, os.O_RDONLY|syscall.O_DIRECTORY|syscall.O_NOFOLLOW|syscall.O_NONBLOCK, 0)
}
