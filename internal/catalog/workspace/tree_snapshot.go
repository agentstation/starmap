package workspace

import (
	"cmp"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	stderrors "errors"
	"io"
	"os"
	"path"
	"path/filepath"
	"slices"

	"github.com/agentstation/starmap/pkg/errors"
)

type treeEntry struct {
	Path         string `json:"path"`
	Directory    bool   `json:"directory"`
	Mode         uint32 `json:"mode"`
	Size         int64  `json:"size"`
	SHA256       string `json:"sha256,omitempty"`
	AccessSHA256 string `json:"access_sha256"`
}

type treeSnapshot struct {
	ID         string      `json:"id"`
	Digest     string      `json:"digest"`
	Entries    []treeEntry `json:"entries"`
	identities map[string]string
}

type treeScanner struct {
	ctx        context.Context
	root       *os.Root
	entries    []treeEntry
	seen       int
	nameBytes  int
	bytes      int64
	identities map[string]string
}

func snapshotTree(ctx context.Context, target string) (treeSnapshot, error) {
	parent, err := os.OpenRoot(filepath.Dir(target))
	if err != nil {
		return treeSnapshot{}, err
	}
	defer func() { _ = parent.Close() }()
	return snapshotTreeAt(ctx, parent, filepath.Base(target))
}

func snapshotTreeAt(ctx context.Context, parent *os.Root, target string) (treeSnapshot, error) {
	info, err := parent.Lstat(target)
	if err != nil {
		return treeSnapshot{}, err
	}
	if !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
		return treeSnapshot{}, replacementConflict(target, "expected a real workspace directory")
	}
	root, err := parent.OpenRoot(target)
	if err != nil {
		return treeSnapshot{}, err
	}
	defer func() { _ = root.Close() }()
	file, err := root.Open(".")
	if err != nil {
		return treeSnapshot{}, err
	}
	defer func() { _ = file.Close() }()
	opened, err := file.Stat()
	if err != nil {
		return treeSnapshot{}, err
	}
	if !os.SameFile(info, opened) {
		return treeSnapshot{}, replacementConflict(target, "directory changed before inspection")
	}
	id, err := entryIdentity(file)
	if err != nil {
		return treeSnapshot{}, err
	}
	scanner := treeScanner{ctx: ctx, root: root, seen: 1, identities: make(map[string]string)}
	if err := scanner.visit("."); err != nil {
		return treeSnapshot{}, err
	}
	after, err := parent.Lstat(target)
	if err != nil {
		return treeSnapshot{}, err
	}
	if !os.SameFile(info, after) || after.Mode()&os.ModeSymlink != 0 {
		return treeSnapshot{}, replacementConflict(target, "directory changed during inspection")
	}
	slices.SortFunc(scanner.entries, func(a, b treeEntry) int { return cmp.Compare(a.Path, b.Path) })
	digest, err := treeEntriesDigest(scanner.entries)
	if err != nil {
		return treeSnapshot{}, err
	}
	return treeSnapshot{ID: id, Digest: digest, Entries: scanner.entries, identities: scanner.identities}, nil
}

func (s *treeScanner) visit(name string) error {
	entry, err := s.entry(name)
	if err != nil {
		return err
	}
	s.entries = append(s.entries, entry)
	if !entry.Directory {
		return nil
	}
	directory, err := s.root.Open(filepath.FromSlash(name))
	if err != nil {
		return err
	}
	children, readErr := s.readChildren(directory, name)
	closeErr := directory.Close()
	if readErr != nil {
		return readErr
	}
	if closeErr != nil {
		return closeErr
	}
	slices.Sort(children)
	for _, child := range children {
		if err := s.visit(path.Join(name, child)); err != nil {
			return err
		}
	}
	return nil
}

func (s *treeScanner) readChildren(directory *os.File, parent string) ([]string, error) {
	var names []string
	for {
		if err := s.ctx.Err(); err != nil {
			return nil, err
		}
		entries, err := directory.ReadDir(replacementReadBatch)
		if err != nil && !stderrors.Is(err, io.EOF) {
			return nil, err
		}
		for _, entry := range entries {
			s.seen++
			s.nameBytes += len(path.Join(parent, entry.Name()))
			if s.seen > replacementMaxEntries || s.nameBytes > replacementMaxNameBytes {
				return nil, replacementLimit("inventory")
			}
			names = append(names, entry.Name())
		}
		if stderrors.Is(err, io.EOF) {
			return names, nil
		}
	}
}

func (s *treeScanner) entry(name string) (treeEntry, error) {
	if err := s.ctx.Err(); err != nil {
		return treeEntry{}, err
	}
	info, err := s.root.Lstat(filepath.FromSlash(name))
	if err != nil {
		return treeEntry{}, err
	}
	entry := treeEntry{Path: name, Directory: info.IsDir(), Mode: uint32(info.Mode() & workspaceAccessMode)}
	if !info.IsDir() && !info.Mode().IsRegular() {
		return treeEntry{}, replacementConflict(name, "workspace entries must be regular files or directories")
	}
	if !info.IsDir() && (info.Size() < 0 || info.Size() > replacementMaxBytes-s.bytes) {
		return treeEntry{}, replacementLimit("bytes")
	}
	file, err := openSnapshotEntry(s.root, filepath.FromSlash(name), info)
	if err != nil {
		return treeEntry{}, err
	}
	defer func() { _ = file.Close() }()
	opened, err := file.Stat()
	if err != nil {
		return treeEntry{}, err
	}
	if !os.SameFile(info, opened) || info.Mode().Type() != opened.Mode().Type() {
		return treeEntry{}, replacementConflict(name, "file changed before inspection")
	}
	if s.identities != nil {
		id, err := entryIdentity(file)
		if err != nil {
			return treeEntry{}, err
		}
		s.identities[name] = id
	}
	entry.AccessSHA256, err = entryAccessDigest(file)
	if err != nil {
		return treeEntry{}, err
	}
	if info.IsDir() {
		return entry, s.checkEntryAccess(name, file, info, entry.AccessSHA256)
	}
	hash := sha256.New()
	var n int64
	// Windows rejects reads within a held lock, including reads past an empty file.
	if info.Size() != 0 {
		n, err = io.CopyN(hash, snapshotReader{ctx: s.ctx, file: file}, info.Size()+1)
		if err != nil && !stderrors.Is(err, io.EOF) {
			return treeEntry{}, err
		}
	}
	if n != info.Size() {
		return treeEntry{}, replacementConflict(name, "file size changed during inspection")
	}
	after, err := s.root.Lstat(filepath.FromSlash(name))
	if err != nil {
		return treeEntry{}, err
	}
	if !os.SameFile(info, after) || !after.Mode().IsRegular() || after.Size() != n || !after.ModTime().Equal(info.ModTime()) {
		return treeEntry{}, replacementConflict(name, "file changed during inspection")
	}
	s.bytes += n
	entry.Size = n
	entry.SHA256 = hex.EncodeToString(hash.Sum(nil))
	return entry, s.checkEntryAccess(name, file, info, entry.AccessSHA256)
}

func (s *treeScanner) checkEntryAccess(name string, file *os.File, expected os.FileInfo, access string) error {
	current, err := s.root.Lstat(filepath.FromSlash(name))
	if err != nil {
		return err
	}
	if !os.SameFile(expected, current) || current.Mode().Type() != expected.Mode().Type() ||
		current.Mode()&workspaceAccessMode != expected.Mode()&workspaceAccessMode {
		return replacementConflict(name, "workspace entry changed during access inspection")
	}
	after, err := entryAccessDigest(file)
	if err != nil {
		return err
	}
	if after != access {
		return replacementConflict(name, "workspace access changed during inspection")
	}
	return nil
}

type snapshotReader struct {
	ctx  context.Context
	file *os.File
}

func (r snapshotReader) Read(buffer []byte) (int, error) {
	if err := r.ctx.Err(); err != nil {
		return 0, err
	}
	return r.file.Read(buffer)
}

func treeEntriesDigest(entries []treeEntry) (string, error) {
	data, err := json.Marshal(entries)
	if err != nil {
		return "", err
	}
	hash := sha256.Sum256(data)
	return hex.EncodeToString(hash[:]), nil
}

func replacementConflict(target, message string) error {
	return &errors.ConflictError{Resource: "workspace replacement", Actual: target, Message: message}
}

func replacementLimit(field string) error {
	return &errors.ValidationError{Field: "workspace_replacement." + field, Message: "exceeds the replacement resource limit"}
}
