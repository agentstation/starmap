//go:build darwin || linux

package privatefiles

import (
	"os"
	"path/filepath"
)

// CreateDirectory creates private children through verified directory handles.
// Existing directories retain their permissions. Every parent must pass the ancestor policy.
func CreateDirectory(directory string) error {
	return createDirectory(directory, nil)
}

func createDirectory(directory string, checkpoint func(*os.Root, string) error) error {
	directory, err := filepath.Abs(directory)
	if err != nil {
		return err
	}
	if err := ValidateAncestors(directory); err != nil {
		return err
	}
	var missing []string
	anchor := directory
	for {
		info, err := os.Lstat(anchor)
		if err == nil {
			if info.Mode()&os.ModeSymlink != 0 && anchor != directory {
				if err := validateAncestorOwner(info); err != nil {
					return err
				}
			} else if err := validateAncestorMetadata(info); err != nil {
				return err
			}
			break
		}
		if !os.IsNotExist(err) {
			return err
		}
		missing = append(missing, filepath.Base(anchor))
		anchor = filepath.Dir(anchor)
	}
	resolved, err := filepath.EvalSymlinks(anchor)
	if err != nil {
		return err
	}
	info, err := os.Lstat(resolved)
	if err != nil {
		return err
	}
	root, err := os.OpenRoot(resolved)
	if err != nil {
		return err
	}
	defer func() { _ = root.Close() }()
	if err := validateAncestorRoot(root, info); err != nil {
		return err
	}
	for index := len(missing) - 1; index >= 0; index-- {
		if err := ValidateAncestors(directory); err != nil {
			return err
		}
		if err := validateAncestorRoot(root, info); err != nil {
			return err
		}
		name := missing[index]
		if err := root.Mkdir(name, DirectoryMode); err != nil && !os.IsExist(err) {
			return err
		}
		if checkpoint != nil {
			if err := checkpoint(root, name); err != nil {
				return err
			}
		}
		childInfo, err := root.Lstat(name)
		if err != nil {
			return err
		}
		if err := validateAncestorMetadata(childInfo); err != nil {
			return err
		}
		child, err := root.OpenRoot(name)
		if err != nil {
			return err
		}
		if err := validateAncestorRoot(child, childInfo); err != nil {
			_ = child.Close()
			return err
		}
		if err := root.Close(); err != nil {
			_ = child.Close()
			return err
		}
		root = child
		info = childInfo
	}
	if err := ValidateAncestors(directory); err != nil {
		return err
	}
	return validateAncestorRoot(root, info)
}

// CreateChild exclusively creates one private child directory.
func CreateChild(root *os.Root, name string) error {
	return root.Mkdir(name, DirectoryMode)
}

// CreateFile exclusively creates one private regular file.
func CreateFile(root *os.Root, name string) (*os.File, error) {
	return root.OpenFile(name, os.O_CREATE|os.O_EXCL|os.O_WRONLY, FileMode)
}
