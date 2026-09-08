package filepublish

import (
	"os"

	"golang.org/x/sys/unix"
)

func renameDirectory(parent *os.Root, source string, destination *os.Root, target string) error {
	directory, err := parent.Open(".")
	if err != nil {
		return err
	}
	defer func() { _ = directory.Close() }()
	to, err := destination.Open(".")
	if err != nil {
		return err
	}
	defer func() { _ = to.Close() }()
	return unix.RenameatxNp(int(directory.Fd()), source, int(to.Fd()), target, unix.RENAME_EXCL)
}
