package workspace

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	stderrors "errors"
	"io"
	"os"
	"path/filepath"

	"github.com/agentstation/starmap/internal/filepublish"
)

type workspaceRecordWriter struct {
	beforePublish func(string) error
	afterPublish  func(string) error
	writeBytes    func(*os.File, []byte) (int, error)
}

type recordPublication struct {
	replace       bool
	normalizeMode bool
}

type workspaceRecordState struct {
	entry    treeEntry
	identity string
}

func (h workspaceRecordWriter) write(file *os.File, data []byte) (int, error) {
	if h.writeBytes != nil {
		return h.writeBytes(file, data)
	}
	return file.Write(data)
}

func (h workspaceRecordWriter) publish(ctx context.Context, root *os.Root, name string, data []byte, options recordPublication) (published bool, resultErr error) {
	if err := ctx.Err(); err != nil {
		return false, err
	}
	if !replacementChildName(name) {
		return false, invalidReplacement("record_name")
	}
	if len(data) > replacementJournalMax {
		return false, replacementLimit("record_bytes")
	}
	before, err := optionalWorkspaceRecord(ctx, root, name)
	if err != nil {
		return false, err
	}
	if !options.replace && before.identity != "" {
		return false, replacementConflict(name, "record destination already exists")
	}
	temporary := "." + name + "." + rand.Text()
	file, err := createStagedFile(root, temporary)
	if err != nil {
		return false, err
	}
	owned, err := newWorkspaceRecordState(temporary, file)
	if err != nil {
		_ = file.Close()
		return false, err
	}
	defer func() {
		cleanupCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), workspaceCleanupTimeout)
		defer cancel()
		resultErr = stderrors.Join(resultErr, owned.cleanup(cleanupCtx, root), file.Close())
	}()
	if options.normalizeMode {
		if err := file.Chmod(fileMode); err != nil {
			return false, err
		}
		normalized, err := newWorkspaceRecordState(temporary, file)
		if err != nil {
			return false, err
		}
		owned = normalized
	}
	n, err := h.write(file, data)
	if n < 0 || n > len(data) {
		return false, invalidReplacement("record_write_count")
	}
	digest := sha256.Sum256(data[:n])
	owned.entry.Size, owned.entry.SHA256 = int64(n), hex.EncodeToString(digest[:])
	if err != nil {
		return false, err
	}
	if n != len(data) {
		return false, io.ErrShortWrite
	}
	if err := ctx.Err(); err != nil {
		return false, err
	}
	if err := file.Sync(); err != nil {
		return false, err
	}
	if h.beforePublish != nil {
		if err := h.beforePublish(filepath.Join(root.Name(), temporary)); err != nil {
			return false, err
		}
	}
	if err := owned.check(ctx, root); err != nil {
		return false, err
	}
	current, err := optionalWorkspaceRecord(ctx, root, name)
	if err != nil {
		return false, err
	}
	if current != before {
		return false, replacementConflict(name, "record destination changed during publication")
	}
	if err := ctx.Err(); err != nil {
		return false, err
	}
	if options.replace {
		err = root.Rename(temporary, name)
	} else {
		err = root.Link(temporary, name)
	}
	if err != nil {
		return false, err
	}
	if h.afterPublish != nil {
		if err := h.afterPublish(filepath.Join(root.Name(), temporary)); err != nil {
			return true, err
		}
	}
	return true, filepublish.SyncDirectory(root)
}

func newWorkspaceRecordState(name string, file *os.File) (workspaceRecordState, error) {
	info, err := file.Stat()
	if err != nil {
		return workspaceRecordState{}, err
	}
	if !info.Mode().IsRegular() || info.Size() != 0 {
		return workspaceRecordState{}, replacementConflict(name, "new record must be empty and regular")
	}
	identity, err := entryIdentity(file)
	if err != nil {
		return workspaceRecordState{}, err
	}
	access, err := entryAccessDigest(file)
	if err != nil {
		return workspaceRecordState{}, err
	}
	digest := sha256.Sum256(nil)
	return workspaceRecordState{
		identity: identity,
		entry:    treeEntry{Path: name, Mode: uint32(info.Mode() & workspaceAccessMode), AccessSHA256: access, SHA256: hex.EncodeToString(digest[:])},
	}, nil
}

func optionalWorkspaceRecord(ctx context.Context, root *os.Root, name string) (workspaceRecordState, error) {
	scanner := treeScanner{ctx: ctx, root: root, bytes: replacementMaxBytes - replacementJournalMax, identities: make(map[string]string)}
	entry, err := scanner.entry(name)
	if os.IsNotExist(err) {
		return workspaceRecordState{}, nil
	}
	if err != nil {
		return workspaceRecordState{}, err
	}
	if entry.Directory {
		return workspaceRecordState{}, replacementConflict(name, "record destination must be a regular file")
	}
	return workspaceRecordState{entry: entry, identity: scanner.identities[name]}, nil
}

func (s workspaceRecordState) check(ctx context.Context, root *os.Root) error {
	scanner := treeScanner{ctx: ctx, root: root, bytes: replacementMaxBytes - s.entry.Size, identities: make(map[string]string)}
	actual, err := scanner.entry(s.entry.Path)
	if err != nil {
		return err
	}
	if actual != s.entry || scanner.identities[s.entry.Path] != s.identity {
		return replacementConflict(s.entry.Path, "record changed after creation")
	}
	return nil
}

func (s workspaceRecordState) cleanup(ctx context.Context, root *os.Root) error {
	if err := s.check(ctx, root); err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	if err := root.Remove(s.entry.Path); err != nil {
		return err
	}
	return filepublish.SyncDirectory(root)
}
