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
	"strings"

	"github.com/agentstation/starmap/internal/filepublish"
)

type workspaceRecordWriter struct {
	writer        *workspaceWriter
	checkWriter   func() error
	beforePublish func(string) error
	afterPublish  func(string) error
	afterRecord   func(string) error
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

func (h workspaceRecordWriter) publish(ctx context.Context, root *os.Root, name string, data []byte, options recordPublication) (published workspaceRecordState, resultErr error) {
	if err := ctx.Err(); err != nil {
		return workspaceRecordState{}, err
	}
	if !replacementChildName(name) {
		return workspaceRecordState{}, invalidReplacement("record_name")
	}
	if len(data) > replacementJournalMax {
		return workspaceRecordState{}, replacementLimit("record_bytes")
	}
	before, err := optionalWorkspaceRecord(ctx, root, name)
	if err != nil {
		return workspaceRecordState{}, err
	}
	if !options.replace && before.identity != "" {
		return workspaceRecordState{}, replacementConflict(name, "record destination already exists")
	}
	stage, err := h.prepareRecord(ctx, root, name)
	if err != nil {
		return workspaceRecordState{}, err
	}
	if stage != nil {
		defer func() { resultErr = stderrors.Join(resultErr, stage.close(ctx)) }()
	}
	suffix := rand.Text()
	if stage != nil {
		suffix = strings.TrimPrefix(stage.name, "."+filepath.Base(h.writer.target)+".preparing-")
	}
	temporary := "." + name + "." + suffix
	file, err := createStagedFile(root, temporary)
	if err != nil {
		return workspaceRecordState{}, err
	}
	owned, err := newWorkspaceRecordState(temporary, file)
	if err != nil {
		_ = file.Close()
		return workspaceRecordState{}, err
	}
	defer func() {
		cleanupCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), workspaceCleanupTimeout)
		defer cancel()
		if stage == nil {
			resultErr = stderrors.Join(resultErr, owned.cleanup(cleanupCtx, root))
		}
		resultErr = stderrors.Join(resultErr, file.Close())
	}()
	if options.normalizeMode {
		if err := file.Chmod(fileMode); err != nil {
			return workspaceRecordState{}, err
		}
		normalized, err := newWorkspaceRecordState(temporary, file)
		if err != nil {
			return workspaceRecordState{}, err
		}
		owned = normalized
	}
	if err := h.persistRecord(ctx, stage, root, name, file, owned); err != nil {
		return workspaceRecordState{}, err
	}
	n, err := h.write(file, data)
	if n < 0 || n > len(data) {
		return workspaceRecordState{}, invalidReplacement("record_write_count")
	}
	digest := sha256.Sum256(data[:n])
	owned.entry.Size, owned.entry.SHA256 = int64(n), hex.EncodeToString(digest[:])
	if recordErr := h.persistRecord(ctx, stage, root, name, file, owned); recordErr != nil {
		return workspaceRecordState{}, stderrors.Join(err, recordErr)
	}
	if err != nil {
		return workspaceRecordState{}, err
	}
	if n != len(data) {
		return workspaceRecordState{}, io.ErrShortWrite
	}
	if err := ctx.Err(); err != nil {
		return workspaceRecordState{}, err
	}
	if err := file.Sync(); err != nil {
		return workspaceRecordState{}, err
	}
	if h.beforePublish != nil {
		if err := h.beforePublish(filepath.Join(root.Name(), temporary)); err != nil {
			return workspaceRecordState{}, err
		}
	}
	if err := owned.check(ctx, root); err != nil {
		return workspaceRecordState{}, err
	}
	if stage != nil {
		if err := stage.checkChildren(ctx); err != nil {
			return workspaceRecordState{}, err
		}
	}
	current, err := optionalWorkspaceRecord(ctx, root, name)
	if err != nil {
		return workspaceRecordState{}, err
	}
	if current != before {
		return workspaceRecordState{}, replacementConflict(name, "record destination changed during publication")
	}
	if err := ctx.Err(); err != nil {
		return workspaceRecordState{}, err
	}
	if h.checkWriter != nil {
		if err := h.checkWriter(); err != nil {
			return workspaceRecordState{}, err
		}
	}
	if options.replace {
		err = root.Rename(temporary, name)
	} else {
		err = root.Link(temporary, name)
	}
	if err != nil {
		return workspaceRecordState{}, err
	}
	published = owned
	published.entry.Path = name
	if h.afterPublish != nil {
		if err := h.afterPublish(filepath.Join(root.Name(), temporary)); err != nil {
			return published, err
		}
	}
	return published, filepublish.SyncDirectory(root)
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
