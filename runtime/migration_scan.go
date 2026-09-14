package runtime

import (
	"cmp"
	"context"
	stderrors "errors"
	"io"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"slices"
)

const (
	migrationSourceMaxEntries = 4 * migrationManifestMaxFiles
	migrationScanBatch        = 128
)

type migrationScanner struct {
	ctx       context.Context
	root      *os.Root
	limit     int
	seen      int
	nameBytes int
}

// walkMigrationTree counts every entry before retaining a directory inventory.
// The caller supplies the entry limit. Aggregate path names cannot exceed the manifest byte limit.
func walkMigrationTree(ctx context.Context, root *os.Root, limit int, visit fs.WalkDirFunc) error {
	if limit < 1 {
		return invalidMigrationIntent("scan_entries")
	}
	info, err := root.Lstat(".")
	if err != nil {
		return err
	}
	scanner := migrationScanner{ctx: ctx, root: root, limit: limit, seen: 1, nameBytes: 1}
	return scanner.walk(".", fs.FileInfoToDirEntry(info), visit)
}

func (s *migrationScanner) walk(name string, entry fs.DirEntry, visit fs.WalkDirFunc) error {
	if err := s.ctx.Err(); err != nil {
		return err
	}
	if err := visit(name, entry, nil); err != nil {
		return err
	}
	if !entry.IsDir() {
		return nil
	}
	before, err := s.root.Lstat(filepath.FromSlash(name))
	if err != nil {
		return err
	}
	if !before.IsDir() || before.Mode()&os.ModeSymlink != 0 {
		return invalidMigrationIntent("scan_directory")
	}
	directory, err := s.root.Open(filepath.FromSlash(name))
	if err != nil {
		return err
	}
	opened, err := directory.Stat()
	if err != nil || !os.SameFile(before, opened) {
		_ = directory.Close()
		return stderrors.Join(invalidMigrationIntent("scan_directory"), err)
	}
	children, readErr := s.children(directory, name)
	closeErr := directory.Close()
	if readErr != nil || closeErr != nil {
		return stderrors.Join(readErr, closeErr)
	}
	slices.SortFunc(children, func(a, b fs.DirEntry) int { return cmp.Compare(a.Name(), b.Name()) })
	for _, child := range children {
		if err := s.walk(path.Join(name, child.Name()), child, visit); err != nil {
			return err
		}
	}
	return nil
}

func (s *migrationScanner) children(directory *os.File, parent string) ([]fs.DirEntry, error) {
	var children []fs.DirEntry
	for {
		if err := s.ctx.Err(); err != nil {
			return nil, err
		}
		batch, err := directory.ReadDir(min(migrationScanBatch, s.limit-s.seen+1))
		if err != nil && !stderrors.Is(err, io.EOF) {
			return nil, err
		}
		for _, entry := range batch {
			s.seen++
			s.nameBytes += len(path.Join(parent, entry.Name()))
			if s.seen > s.limit {
				return nil, invalidMigrationIntent("scan_entries")
			}
			if s.nameBytes > migrationManifestMaxBytes {
				return nil, invalidMigrationIntent("scan_path_bytes")
			}
			children = append(children, entry)
		}
		if stderrors.Is(err, io.EOF) {
			return children, nil
		}
	}
}
