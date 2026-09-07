//go:build !windows && !darwin && !linux

package privatefiles

import "os"

// CreateDirectory creates missing directories with private access.
func CreateDirectory(directory string) error {
	return os.MkdirAll(directory, DirectoryMode)
}

// CreateChild exclusively creates one private child directory.
func CreateChild(root *os.Root, name string) error {
	return root.Mkdir(name, DirectoryMode)
}

// CreateFile exclusively creates one private regular file.
func CreateFile(root *os.Root, name string) (*os.File, error) {
	return root.OpenFile(name, os.O_CREATE|os.O_EXCL|os.O_WRONLY, FileMode)
}
