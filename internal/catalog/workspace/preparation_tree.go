package workspace

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	stderrors "errors"
	"hash"
	"io"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"slices"
	"strings"
)

type preparationTree struct {
	root       *os.Root
	entries    map[string]treeEntry
	identities map[string]string
	bytes      int64
	nameBytes  int
	syncWrites bool
	record     func(treeEntry, string) error
}

func trackPreparationTree(root *os.Root) (*preparationTree, error) {
	tree := &preparationTree{root: root, entries: make(map[string]treeEntry), identities: make(map[string]string)}
	file, err := openStagedDirectory(root, ".")
	if err != nil {
		return nil, err
	}
	defer func() { _ = file.Close() }()
	if err := tree.created(".", file); err != nil {
		return nil, err
	}
	return tree, nil
}

func (t *preparationTree) snapshot() (treeSnapshot, error) {
	names := make([]string, 0, len(t.entries))
	for name := range t.entries {
		names = append(names, name)
	}
	slices.Sort(names)
	entries := make([]treeEntry, 0, len(names))
	for _, name := range names {
		entries = append(entries, t.entries[name])
	}
	digest, err := treeEntriesDigest(entries)
	return treeSnapshot{ID: t.identities["."], Entries: entries, Digest: digest, identities: t.identities}, err
}

func (t *preparationTree) allowEntry(name string) error {
	if len(t.entries) >= replacementMaxEntries || len(name) > replacementMaxNameBytes-t.nameBytes {
		return replacementLimit("inventory")
	}
	if _, present := t.entries[name]; present {
		return replacementConflict(name, "preparation entry already has an owner")
	}
	return nil
}

func (t *preparationTree) created(name string, file *os.File) error {
	if err := t.allowEntry(name); err != nil {
		return err
	}
	info, err := file.Stat()
	if err != nil {
		return err
	}
	if !info.IsDir() && (!info.Mode().IsRegular() || info.Size() != 0) {
		return replacementConflict(name, "new preparation file must be empty and regular")
	}
	id, err := entryIdentity(file)
	if err != nil {
		return err
	}
	access, err := entryAccessDigest(file)
	if err != nil {
		return err
	}
	entry := treeEntry{Path: name, Directory: info.IsDir(), Mode: uint32(info.Mode() & workspaceAccessMode), AccessSHA256: access}
	if !entry.Directory {
		digest := sha256.Sum256(nil)
		entry.SHA256 = hex.EncodeToString(digest[:])
	}
	t.entries[name], t.identities[name] = entry, id
	t.nameBytes += len(name)
	return t.persist(name, file)
}

func (t *preparationTree) check(ctx context.Context, name string) error {
	scanner := treeScanner{ctx: ctx, root: t.root, identities: make(map[string]string)}
	actual, err := scanner.entry(name)
	if err != nil {
		return err
	}
	if actual != t.entries[name] || scanner.identities[name] != t.identities[name] {
		return replacementConflict(name, "preparation entry changed after creation")
	}
	return nil
}

func (t *preparationTree) directory(ctx context.Context, name string) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if !fs.ValidPath(name) || strings.ContainsAny(name, "\\:\x00") {
		return invalidReplacement("preparation_path")
	}
	if entry, exists := t.entries[name]; exists {
		if !entry.Directory {
			return replacementConflict(name, "preparation parent is not a directory")
		}
		return t.check(ctx, name)
	}
	if err := t.directory(ctx, path.Dir(name)); err != nil {
		return err
	}
	if err := t.allowEntry(name); err != nil {
		return err
	}
	if err := t.root.Mkdir(filepath.FromSlash(name), directoryMode); err != nil {
		return err
	}
	file, err := openStagedDirectory(t.root, filepath.FromSlash(name))
	if err != nil {
		return err
	}
	defer func() { _ = file.Close() }()
	return t.created(name, file)
}

func (t *preparationTree) rememberAccess(name string, file *os.File) error {
	entry, exists := t.entries[name]
	id, err := entryIdentity(file)
	if err != nil {
		return err
	}
	info, err := file.Stat()
	if err != nil {
		return err
	}
	if !exists || id != t.identities[name] || info.IsDir() != entry.Directory || (!entry.Directory && info.Size() != entry.Size) {
		return replacementConflict(name, "preparation identity changed during access restoration")
	}
	access, err := entryAccessDigest(file)
	if err != nil {
		return err
	}
	entry.Mode = uint32(info.Mode() & workspaceAccessMode)
	entry.AccessSHA256 = access
	t.entries[name] = entry
	return t.persist(name, file)
}

func (t *preparationTree) persist(name string, file *os.File) error {
	if t.record == nil {
		return nil
	}
	if !t.entries[name].Directory {
		if err := file.Sync(); err != nil {
			return err
		}
	}
	return t.record(t.entries[name], t.identities[name])
}

func (t *preparationTree) writeFile(ctx context.Context, name string, data []byte) error {
	digest := sha256.Sum256(data)
	return t.writeFrom(ctx, filepath.ToSlash(name), bytes.NewReader(data), int64(len(data)), hex.EncodeToString(digest[:]), nil)
}

func (t *preparationTree) writeFrom(ctx context.Context, name string, input io.Reader, size int64, digest string, after func(*os.File) error) (resultErr error) {
	if !fs.ValidPath(name) || strings.ContainsAny(name, "\\:\x00") || name == "." {
		return invalidReplacement("preparation_path")
	}
	if size < 0 || size > replacementMaxBytes-t.bytes {
		return replacementLimit("bytes")
	}
	if err := t.directory(ctx, path.Dir(name)); err != nil {
		return err
	}
	if err := t.allowEntry(name); err != nil {
		return err
	}
	output, err := createStagedFile(t.root, filepath.FromSlash(name))
	if err != nil {
		return err
	}
	defer func() { resultErr = stderrors.Join(resultErr, output.Close()) }()
	if err := t.created(name, output); err != nil {
		return err
	}
	writer := preparationWriter{ctx: ctx, file: output, hash: sha256.New()}
	_, writeErr := io.CopyN(&writer, input, size)
	entry := t.entries[name]
	entry.Size, entry.SHA256 = writer.bytes, hex.EncodeToString(writer.hash.Sum(nil))
	t.entries[name] = entry
	t.bytes += writer.bytes
	if err := t.persist(name, output); err != nil {
		return stderrors.Join(writeErr, err)
	}
	if writeErr != nil {
		return writeErr
	}
	if entry.Size != size || entry.SHA256 != digest {
		return replacementConflict(name, "preparation input changed during copy")
	}
	if after != nil {
		if err := after(output); err != nil {
			return err
		}
		if err := t.rememberAccess(name, output); err != nil {
			return err
		}
	}
	if t.syncWrites {
		return output.Sync()
	}
	return ctx.Err()
}

type preparationWriter struct {
	ctx   context.Context
	file  *os.File
	hash  hash.Hash
	bytes int64
}

func (w *preparationWriter) Write(data []byte) (int, error) {
	if err := w.ctx.Err(); err != nil {
		return 0, err
	}
	n, err := w.file.Write(data)
	_, _ = w.hash.Write(data[:n])
	w.bytes += int64(n)
	return n, err
}
