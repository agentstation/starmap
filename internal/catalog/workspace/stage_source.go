package workspace

import (
	"context"
	"os"
	"path/filepath"
	"strings"
)

func (s *workspaceStage) readSource(ctx context.Context, target string, expected *treeSnapshot) error {
	tree, err := optionalTree(ctx, s.parent, filepath.Base(target))
	if err != nil {
		return err
	}
	if expected != nil && (tree.ID != "" || expected.ID != "") && !sameTree(tree, *expected) {
		return replacementConflict(target, "workspace changed before staging")
	}
	s.original = tree
	if tree.ID == "" {
		return nil
	}
	s.source, err = s.parent.OpenRoot(filepath.Base(target))
	if err != nil {
		return err
	}
	file, err := s.source.Open(".")
	if err != nil {
		return err
	}
	defer func() { _ = file.Close() }()
	id, err := entryIdentity(file)
	if err != nil {
		return err
	}
	if id != tree.ID {
		return replacementConflict(target, "workspace identity changed before staging")
	}
	return nil
}

func (s *workspaceStage) copySource(ctx context.Context) error {
	if s.source == nil {
		return nil
	}
	render := s.trees["render"]
	for _, entry := range s.original.Entries {
		if err := ctx.Err(); err != nil {
			return err
		}
		if entry.Path == "." || managedWorkspaceRecord(entry.Path, entry.Directory) {
			continue
		}
		if existing, present := render.entries[entry.Path]; present {
			if existing.Directory != entry.Directory {
				return replacementConflict(entry.Path, "operator entry conflicts with a generated record")
			}
			continue
		}
		if entry.Directory {
			if err := render.directory(ctx, entry.Path); err != nil {
				return err
			}
			continue
		}
		if err := copyOwnedSourceFile(ctx, s.source, render, entry); err != nil {
			return err
		}
	}
	return nil
}

func managedWorkspaceRecord(name string, directory bool) bool {
	if !directory {
		switch name {
		case "providers.yaml", "authors.yaml", "endpoints.yaml", "provenance.yaml", "canonical-aliases.yaml":
			return true
		}
	}
	parts := strings.Split(name, "/")
	if len(parts) < 3 || (parts[0] != "providers" && parts[0] != "authors") || parts[2] != "models" {
		return false
	}
	return (directory && len(parts) == 3) || (!directory && strings.HasSuffix(name, ".yaml"))
}

func copyOwnedSourceFile(ctx context.Context, source *os.Root, destination *preparationTree, entry treeEntry) error {
	name := filepath.FromSlash(entry.Path)
	info, err := source.Lstat(name)
	if err != nil {
		return err
	}
	if !info.Mode().IsRegular() || info.Size() != entry.Size {
		return replacementConflict(name, "source file changed before copy")
	}
	input, err := openSnapshotEntry(source, name, info)
	if err != nil {
		return err
	}
	defer func() { _ = input.Close() }()
	actual, err := input.Stat()
	if err != nil {
		return err
	}
	if !os.SameFile(info, actual) {
		return replacementConflict(name, "source file identity changed before copy")
	}
	return destination.writeFrom(ctx, entry.Path, snapshotReader{ctx: ctx, file: input}, entry.Size, entry.SHA256, nil)
}
