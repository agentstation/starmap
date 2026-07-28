//go:build darwin

package workspace

import "golang.org/x/sys/unix"

func swapDirectories(staged, target string) error {
	return unix.RenameatxNp(
		unix.AT_FDCWD,
		staged,
		unix.AT_FDCWD,
		target,
		unix.RENAME_SWAP,
	)
}
