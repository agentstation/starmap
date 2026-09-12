package workspace

import (
	"context"
	stderrors "errors"
	"io/fs"
	"os"
	"path/filepath"

	"github.com/agentstation/starmap/internal/filepublish"
)

type stagedWorkspace struct {
	path     string
	tree     treeSnapshot
	original treeSnapshot
}

func (s stagedWorkspace) cleanup(ctx context.Context, exchanged treeSnapshot) error {
	cleanup, cancel := context.WithTimeout(context.WithoutCancel(ctx), workspaceCleanupTimeout)
	defer cancel()
	return cleanupWorkspaceTree(cleanup, s.path, s.tree, exchanged)
}

func cleanupWorkspaceTree(ctx context.Context, target string, expected ...treeSnapshot) error {
	parent, err := os.OpenRoot(filepath.Dir(target))
	if err != nil {
		return err
	}
	defer func() { _ = parent.Close() }()
	return cleanupWorkspaceTreeAt(ctx, parent, filepath.Base(target), expected...)
}

func cleanupWorkspaceTreeAt(ctx context.Context, parent *os.Root, name string, expected ...treeSnapshot) error {
	target := filepath.Join(parent.Name(), name)
	actual, err := optionalTree(ctx, parent, name)
	if err != nil || actual.ID == "" {
		return err
	}
	var owned treeSnapshot
	for _, tree := range expected {
		if tree.ID == actual.ID {
			owned = tree
			break
		}
	}
	if owned.ID == "" || len(owned.identities) != len(owned.Entries) {
		return replacementConflict(target, "staging identity is not owned by this operation")
	}
	entries := make(map[string]treeEntry, len(owned.Entries))
	for _, entry := range owned.Entries {
		entries[entry.Path] = entry
	}
	for _, entry := range actual.Entries {
		if entries[entry.Path] != entry || owned.identities[entry.Path] != actual.identities[entry.Path] {
			return replacementConflict(target, "staging contains changed or unrecognized entries; preserve it for recovery")
		}
	}
	root, err := parent.OpenRoot(name)
	if err != nil {
		return err
	}
	defer func() { _ = root.Close() }()
	if err := verifyCleanupRoot(parent, name, root, owned.ID); err != nil {
		return err
	}
	scanner := treeScanner{ctx: ctx, root: root, identities: make(map[string]string)}
	for i := len(actual.Entries) - 1; i > 0; i-- {
		entry := actual.Entries[i]
		current, err := scanner.entry(entry.Path)
		if stderrors.Is(err, fs.ErrNotExist) {
			continue
		}
		if err != nil {
			return err
		}
		if current != entry || scanner.identities[entry.Path] != owned.identities[entry.Path] {
			return replacementConflict(target, "staging entry changed during cleanup")
		}
		if err := verifyCleanupRoot(parent, name, root, owned.ID); err != nil {
			return err
		}
		if err := root.Remove(filepath.FromSlash(entry.Path)); err != nil {
			return err
		}
	}
	if err := root.Close(); err != nil {
		return err
	}
	empty, err := snapshotTreeAt(ctx, parent, name)
	if err != nil {
		return err
	}
	if empty.ID != owned.ID || len(empty.Entries) != 1 || empty.Entries[0] != owned.Entries[0] {
		return replacementConflict(target, "staging is not the verified empty directory")
	}
	if err := parent.Remove(name); err != nil {
		return err
	}
	return filepublish.SyncDirectory(parent)
}

func verifyCleanupRoot(parent *os.Root, name string, root *os.Root, expected string) error {
	file, err := root.Open(".")
	if err != nil {
		return err
	}
	defer func() { _ = file.Close() }()
	id, err := entryIdentity(file)
	if err != nil {
		return err
	}
	opened, err := file.Stat()
	if err != nil {
		return err
	}
	current, err := parent.Lstat(name)
	if err != nil {
		return err
	}
	if id != expected || !current.IsDir() || !os.SameFile(opened, current) {
		return replacementConflict(name, "staging directory changed during cleanup")
	}
	return nil
}
