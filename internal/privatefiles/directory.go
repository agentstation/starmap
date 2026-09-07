// Package privatefiles owns private file creation, access checks, and bounded record I/O.
package privatefiles

import (
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/agentstation/starmap/pkg/errors"
)

const (
	// DirectoryMode excludes group and other access on POSIX.
	DirectoryMode fs.FileMode = 0o700
	// FileMode excludes group and other access on POSIX.
	FileMode fs.FileMode = 0o600
)

// Directory binds a private directory to its original filesystem identity.
// Each operation reopens and validates it. No open handle survives the operation.
type Directory struct {
	path          string
	identity      fs.FileInfo
	beforePublish func(string) error
}

// NewDirectory validates or creates one private directory without changing existing permissions.
// Linux, macOS, and Windows also validate ancestor ownership and access.
func NewDirectory(path string) (*Directory, error) {
	d, err := ExistingDirectory(path)
	if !os.IsNotExist(err) {
		return d, err
	}
	if err := CreateDirectory(path); err != nil {
		return nil, err
	}
	return ExistingDirectory(path)
}

// ExistingDirectory binds an existing private directory without creating any paths.
func ExistingDirectory(path string) (*Directory, error) {
	if !filepath.IsAbs(path) {
		return nil, &errors.ValidationError{Field: "private.directory", Message: "must be absolute"}
	}
	info, err := os.Lstat(path)
	if err != nil {
		return nil, err
	}
	d := &Directory{path: filepath.Clean(path), identity: info}
	root, err := d.Open()
	if err != nil {
		return nil, err
	}
	return d, root.Close()
}

// Open verifies directory identity, ownership, and private access before returning a confined root.
// The caller must close the root.
func (d *Directory) Open() (*os.Root, error) {
	if err := ValidateAncestors(d.path); err != nil {
		return nil, err
	}
	info, err := os.Lstat(d.path)
	if os.IsNotExist(err) {
		return nil, changed(d.path)
	}
	if err != nil {
		return nil, err
	}
	if !info.IsDir() || info.Mode()&os.ModeSymlink != 0 || !os.SameFile(d.identity, info) {
		return nil, changed(d.path)
	}
	if err := ValidateMetadata(info, "private.directory"); err != nil {
		return nil, err
	}
	root, err := os.OpenRoot(d.path)
	if err != nil {
		return nil, err
	}
	valid := false
	defer func() {
		if !valid {
			_ = root.Close()
		}
	}()
	opened, err := root.Stat(".")
	if err != nil {
		return nil, err
	}
	if !os.SameFile(info, opened) {
		return nil, changed(d.path)
	}
	if err := ValidateMetadata(opened, "private.directory"); err != nil {
		return nil, err
	}
	if err := ValidateACL(root, ".", opened, "private.directory"); err != nil {
		return nil, err
	}
	valid = true
	return root, nil
}

// Child validates or privately creates one child under the bound directory handle.
func (d *Directory) Child(name string) (*Directory, error) {
	return d.child(name, true)
}

// ExistingChild binds an existing child without creating any paths.
func (d *Directory) ExistingChild(name string) (*Directory, error) {
	return d.child(name, false)
}

func (d *Directory) child(name string, create bool) (*Directory, error) {
	if err := childName(name); err != nil {
		return nil, err
	}
	root, err := d.Open()
	if err != nil {
		return nil, err
	}
	defer func() { _ = root.Close() }()
	info, err := root.Lstat(name)
	if os.IsNotExist(err) && create {
		if err := CreateChild(root, name); err != nil && !os.IsExist(err) {
			return nil, err
		}
		info, err = root.Lstat(name)
	}
	if err != nil {
		return nil, err
	}
	if err := d.validateLocation(root); err != nil {
		return nil, err
	}
	child := &Directory{path: filepath.Join(d.path, name), identity: info}
	opened, err := child.Open()
	if err != nil {
		return nil, err
	}
	return child, opened.Close()
}

func childName(name string) error {
	if !filepath.IsLocal(name) || name == "." || strings.ContainsAny(name, "/\\:\x00") || strings.TrimRight(name, " .") != name {
		return &errors.ValidationError{Field: "private.file", Message: "requires one portable child name"}
	}
	return nil
}

func changed(path string) error {
	return &errors.ConflictError{Resource: "private file", Actual: path, Message: "file identity or contents changed during access"}
}
