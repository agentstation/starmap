package workspace

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"io"
	"os"
	"path/filepath"
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
	render, err := s.private.OpenRoot("render")
	if err != nil {
		return err
	}
	defer func() { _ = render.Close() }()
	for _, entry := range s.original.Entries {
		if err := ctx.Err(); err != nil {
			return err
		}
		if entry.Path == "." {
			continue
		}
		name := filepath.FromSlash(entry.Path)
		if entry.Directory {
			if err := render.Mkdir(name, directoryMode); err != nil {
				return err
			}
			continue
		}
		if err := copyRecordedSourceFile(ctx, s.source, render, entry); err != nil {
			return err
		}
	}
	return nil
}

func copyRecordedSourceFile(ctx context.Context, source, destination *os.Root, entry treeEntry) error {
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
	output, err := createStagedFile(destination, name)
	if err != nil {
		return err
	}
	defer func() { _ = output.Close() }()
	hash := sha256.New()
	_, err = io.CopyN(io.MultiWriter(output, hash), snapshotReader{ctx: ctx, file: input}, entry.Size)
	if err != nil {
		return err
	}
	if hex.EncodeToString(hash.Sum(nil)) != entry.SHA256 {
		return replacementConflict(name, "source file changed during copy")
	}
	return nil
}
