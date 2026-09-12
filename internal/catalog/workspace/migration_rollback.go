package workspace

import (
	"context"
	stderrors "errors"
	"io/fs"
	"os"
	"path/filepath"

	"github.com/agentstation/starmap/internal/filepublish"
	"github.com/agentstation/starmap/pkg/errors"
)

type legacyStoreMove struct {
	originalParent *os.Root
	stateParent    *os.Root
	originalName   string
	stateName      string
	store          string
}

func captureLegacyStoreIdentity(legacy string) (string, error) {
	parent, err := os.OpenRoot(filepath.Dir(legacy))
	if err != nil {
		return "", err
	}
	defer func() { _ = parent.Close() }()
	return legacyStoreIdentity(parent, filepath.Base(legacy))
}

func legacyStoreIdentity(parent *os.Root, name string) (string, error) {
	info, err := parent.Lstat(name)
	if err != nil {
		return "", err
	}
	if !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
		return "", replacementConflict(name, "legacy catalog store must remain a real directory")
	}
	file, err := openSnapshotEntry(parent, name, info)
	if err != nil {
		return "", err
	}
	defer func() { _ = file.Close() }()
	opened, err := file.Stat()
	if err != nil {
		return "", err
	}
	if !os.SameFile(info, opened) {
		return "", replacementConflict(name, "legacy catalog store changed during inspection")
	}
	id, err := entryIdentity(file)
	if err != nil {
		return "", err
	}
	current, err := parent.Lstat(name)
	if err != nil {
		return "", err
	}
	if !os.SameFile(opened, current) {
		return "", replacementConflict(name, "legacy catalog store changed after inspection")
	}
	return id, nil
}

func prepareLegacyStoreMove(legacy, state string, store string) (*legacyStoreMove, error) {
	originalParent, err := os.OpenRoot(filepath.Dir(legacy))
	if err != nil {
		return nil, err
	}
	stateParent, err := os.OpenRoot(filepath.Dir(state))
	if err != nil {
		_ = originalParent.Close()
		return nil, err
	}
	return &legacyStoreMove{
		originalParent: originalParent, stateParent: stateParent,
		originalName: filepath.Base(legacy), stateName: filepath.Base(state), store: store,
	}, nil
}

func (m *legacyStoreMove) close() {
	_ = m.originalParent.Close()
	_ = m.stateParent.Close()
}

func (m *legacyStoreMove) checkStore(parent *os.Root, name string) error {
	id, err := legacyStoreIdentity(parent, name)
	if err != nil {
		return err
	}
	if id != m.store {
		return &errors.ConflictError{
			Resource: "legacy catalog store", Actual: filepath.Join(parent.Name(), name),
			Message: "store identity changed during migration",
		}
	}
	return nil
}

func (m *legacyStoreMove) relocate() error {
	if err := m.checkStore(m.originalParent, m.originalName); err != nil {
		return err
	}
	return filepublish.DirectoryBetweenRootsNoReplace(m.originalParent, m.originalName, m.stateParent, m.stateName)
}

func (m *legacyStoreMove) rollback(ctx context.Context, projected treeSnapshot) error {
	cleanup, cancel := context.WithTimeout(context.WithoutCancel(ctx), workspaceCleanupTimeout)
	defer cancel()
	if err := m.checkStore(m.stateParent, m.stateName); err != nil {
		return err
	}
	if projected.ID == "" {
		_, err := m.originalParent.Lstat(m.originalName)
		if err == nil {
			return replacementConflict(m.originalName, "vacated catalog path was recreated during migration")
		}
		if !stderrors.Is(err, fs.ErrNotExist) {
			return err
		}
	} else if err := cleanupWorkspaceTreeAt(cleanup, m.originalParent, m.originalName, projected); err != nil {
		return err
	}
	if err := cleanup.Err(); err != nil {
		return err
	}
	if err := m.restore(); err != nil {
		return err
	}
	return m.sync()
}

func (m *legacyStoreMove) restore() error {
	if err := m.checkStore(m.stateParent, m.stateName); err != nil {
		return err
	}
	return filepublish.DirectoryBetweenRootsNoReplace(m.stateParent, m.stateName, m.originalParent, m.originalName)
}

func (m *legacyStoreMove) sync() error {
	if err := filepublish.SyncDirectory(m.originalParent); err != nil {
		return err
	}
	return filepublish.SyncDirectory(m.stateParent)
}
