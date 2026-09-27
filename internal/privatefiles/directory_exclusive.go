package privatefiles

import (
	stderrors "errors"
	"os"
	"path/filepath"

	"github.com/agentstation/starmap/pkg/errors"
)

// NewExclusiveDirectory creates one private directory beneath an existing trusted parent.
// An existing target causes an existence error. Failures preserve uncertain filesystem entries.
func NewExclusiveDirectory(path string) (_ *Directory, resultErr error) {
	if !filepath.IsAbs(path) || filepath.Clean(path) != path {
		return nil, &errors.ValidationError{Field: "private.directory", Message: "requires a clean absolute path"}
	}
	name := filepath.Base(path)
	if err := childName(name); err != nil {
		return nil, err
	}
	if err := ValidateAncestors(path); err != nil {
		return nil, err
	}
	parentPath := filepath.Dir(path)
	parent, err := os.Stat(parentPath)
	if err != nil {
		return nil, err
	}
	root, err := os.OpenRoot(parentPath)
	if err != nil {
		return nil, err
	}
	defer func() { resultErr = stderrors.Join(resultErr, root.Close()) }()
	held, err := root.Stat(".")
	if err != nil {
		return nil, err
	}
	if !os.SameFile(parent, held) {
		return nil, changed(parentPath)
	}
	if err := ValidateAncestors(path); err != nil {
		return nil, err
	}
	if err := CreateChild(root, name); err != nil {
		return nil, err
	}
	info, err := root.Lstat(name)
	if err != nil {
		return nil, err
	}
	current, err := os.Stat(parentPath)
	if err != nil {
		return nil, err
	}
	if !os.SameFile(held, current) {
		return nil, changed(parentPath)
	}
	directory := &Directory{path: path, identity: info}
	verified, err := directory.Open()
	if err != nil {
		return nil, err
	}
	if err := verified.Close(); err != nil {
		return nil, err
	}
	return directory, nil
}
