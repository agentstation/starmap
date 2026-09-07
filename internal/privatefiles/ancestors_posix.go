//go:build darwin || linux

package privatefiles

import (
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"syscall"

	"github.com/agentstation/starmap/pkg/errors"
)

const maxAncestorSymlinks = 255

// ValidateAncestors checks existing parents and selected symlink routes without creating paths.
// Linux and macOS require root or effective-user ownership and protected directory entries.
// Trusted sticky directories permit shared creation. macOS also checks native ACL grants.
func ValidateAncestors(path string) error {
	if !filepath.IsAbs(path) {
		cwd, err := os.Getwd()
		if err != nil {
			return err
		}
		path = cwd + string(filepath.Separator) + path
	}
	if err := validateAncestorDirectory(string(filepath.Separator)); err != nil {
		return err
	}
	current := string(filepath.Separator)
	pending := strings.Split(path, string(filepath.Separator))
	links := 0
	// Inspect each component before processing .. so normalization cannot hide an unsafe ancestor.
	for len(pending) != 0 {
		part := pending[0]
		pending = pending[1:]
		switch part {
		case "", ".":
			continue
		case "..":
			current = filepath.Dir(current)
			continue
		}
		next := filepath.Join(current, part)
		info, err := os.Lstat(next)
		if os.IsNotExist(err) {
			return nil
		}
		if err != nil {
			return err
		}
		if info.Mode()&os.ModeSymlink != 0 {
			links++
			if links > maxAncestorSymlinks {
				return &errors.ValidationError{Field: "private.ancestor", Message: "selected route exceeds the supported symlink limit"}
			}
			if err := validateAncestorOwner(info); err != nil {
				return err
			}
			target, err := os.Readlink(next)
			if err != nil {
				return err
			}
			if filepath.IsAbs(target) {
				current = string(filepath.Separator)
			}
			pending = append(strings.Split(target, string(filepath.Separator)), pending...)
			continue
		}
		if len(pending) == 0 {
			return nil
		}
		if err := validateAncestorDirectory(next); err != nil {
			return err
		}
		current = next
	}
	return nil
}

func validateAncestorOwner(info fs.FileInfo) error {
	stat, ok := info.Sys().(*syscall.Stat_t)
	if !ok || (stat.Uid != 0 && int64(stat.Uid) != int64(os.Geteuid())) {
		return &errors.ValidationError{Field: "private.ancestor", Message: "must belong to root or the effective user. Review ancestor ownership before retrying"}
	}
	return nil
}

func validateAncestorMetadata(info fs.FileInfo) error {
	if !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
		return &errors.ValidationError{Field: "private.ancestor", Message: "must resolve to a real directory"}
	}
	if err := validateAncestorOwner(info); err != nil {
		return err
	}
	if info.Mode().Perm()&0o022 != 0 && info.Mode()&os.ModeSticky == 0 {
		return &errors.ValidationError{Field: "private.ancestor", Message: "permits other accounts to replace directory entries. Review parent permissions before retrying"}
	}
	return nil
}

func validateAncestorDirectory(path string) error {
	info, err := os.Lstat(path)
	if err != nil {
		return err
	}
	if err := validateAncestorMetadata(info); err != nil {
		return err
	}
	root, err := os.OpenRoot(path)
	if err != nil {
		return err
	}
	defer func() { _ = root.Close() }()
	return validateAncestorRoot(root, info)
}

func validateAncestorRoot(root *os.Root, expected fs.FileInfo) error {
	info, err := root.Stat(".")
	if err != nil {
		return err
	}
	if !os.SameFile(expected, info) {
		return changed(root.Name())
	}
	current, err := os.Lstat(root.Name())
	if err != nil {
		return err
	}
	if current.Mode()&os.ModeSymlink != 0 || !os.SameFile(expected, current) {
		return changed(root.Name())
	}
	if err := validateAncestorMetadata(info); err != nil {
		return err
	}
	return validateAncestorACL(root, info)
}
