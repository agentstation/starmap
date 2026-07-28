//go:build linux

package workspace

import "golang.org/x/sys/unix"

func swapDirectories(staged, target string) error {
	return unix.Renameat2(
		unix.AT_FDCWD,
		staged,
		unix.AT_FDCWD,
		target,
		unix.RENAME_EXCHANGE,
	)
}
