// Package filepublish publishes staged filesystem entries atomically.
package filepublish

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/agentstation/starmap/pkg/errors"
)

// DirectoryNoReplace renames a staged directory between direct children of parent.
// The filesystem operation refuses an existing destination, including an empty directory.
// It uses the open parent rather than its original path.
// Callers must synchronize staged contents and directory metadata.
// Success alone does not establish durability.
func DirectoryNoReplace(parent *os.Root, source, target string) error {
	return DirectoryBetweenRootsNoReplace(parent, source, parent, target)
}

// DirectoryBetweenRootsNoReplace moves one direct child between open directories without replacing a destination.
// Callers must synchronize the source tree and both parent directories.
func DirectoryBetweenRootsNoReplace(sourceRoot *os.Root, source string, targetRoot *os.Root, target string) error {
	if sourceRoot == nil || targetRoot == nil || (sourceRoot == targetRoot && source == target) || !childName(source) || !childName(target) {
		return &errors.ValidationError{Field: "publication.directory", Message: "requires an open root and distinct direct child names"}
	}
	info, err := sourceRoot.Lstat(source)
	if err != nil {
		return err
	}
	if !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
		return &errors.ValidationError{Field: "publication.source", Message: "must be a real directory"}
	}
	if err := renameDirectory(sourceRoot, source, targetRoot, target); err != nil {
		return &os.LinkError{Op: "publish directory", Old: source, New: target, Err: err}
	}
	return nil
}

func childName(name string) bool {
	return filepath.IsLocal(name) && name != "." && !strings.ContainsAny(name, "/\\:\x00") && strings.TrimRight(name, " .") == name
}
