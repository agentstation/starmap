package workspace

import (
	"context"
	stderrors "errors"
	"io/fs"
	"os"
	"path/filepath"

	"github.com/agentstation/starmap/pkg/catalogs"
)

func prepareRelocation(ctx context.Context, legacy, state string, generation catalogs.Generation, retained int, writer *workspaceWriter, lease *legacyStoreLease, afterEnclosure func(*workspaceStage) error) (_ *workspaceStage, resultErr error) {
	root, err := relocationRoot(ctx, legacy)
	if err != nil {
		return nil, err
	}
	current, err := relocationCurrent(generation)
	if err != nil {
		return nil, err
	}
	originalParent, err := relocationParentPath(filepath.Dir(legacy))
	if err != nil {
		return nil, err
	}
	stateParent, err := relocationParentPath(filepath.Dir(state))
	if err != nil {
		return nil, err
	}
	record := relocationRecord{State: state, OriginalParent: originalParent, StateParent: stateParent, Root: relocationTree{Tree: root, Identities: root.identities}, Current: current, Retained: retained}
	if lease.path != filepath.Join(legacy, ".commit.lock") {
		parent, err := os.OpenRoot(filepath.Dir(legacy))
		if err != nil {
			return nil, err
		}
		_, alias, readErr := readWorkspaceRecord(parent, filepath.Base(lease.path), replacementJournalMax)
		_ = parent.Close()
		if readErr != nil {
			return nil, readErr
		}
		record.Alias = &relocationFile{Entry: alias.entry, Identity: alias.identity}
	}
	stage, err := prepareWorkspaceEnclosure(ctx, legacy, writer)
	if err != nil {
		return nil, err
	}
	defer func() {
		if resultErr != nil {
			defer stage.releaseHandles()
			cleanup, cancel := context.WithTimeout(context.WithoutCancel(ctx), workspaceCleanupTimeout)
			defer cancel()
			resultErr = stderrors.Join(resultErr, stage.retireRelocation(cleanup))
		}
	}()
	if afterEnclosure != nil {
		if err := afterEnclosure(stage); err != nil {
			return nil, err
		}
	}
	if err := stage.appendRelocation(ctx, preparationEvent{Relocation: &record}); err != nil {
		return nil, err
	}
	_, err = scanLegacyGenerations(ctx, filepath.Join(legacy, "generations"), func(entry fs.DirEntry) error {
		tree, err := snapshotTree(ctx, filepath.Join(legacy, "generations", entry.Name()))
		if err != nil {
			return err
		}
		return stage.appendRelocation(ctx, preparationEvent{Generation: &relocationGeneration{Name: entry.Name(), Snapshot: relocationTree{Tree: tree, Identities: tree.identities}}})
	})
	if err != nil {
		return nil, err
	}
	if err := stage.appendRelocation(ctx, preparationEvent{RelocationReady: true}); err != nil {
		return nil, err
	}
	if err := stage.checkRelocationStore(ctx, legacy, lease); err != nil {
		return nil, err
	}
	return stage, nil
}

func relocationParentPath(path string) (string, error) {
	root, err := os.OpenRoot(path)
	if err != nil {
		return "", err
	}
	defer func() { _ = root.Close() }()
	return relocationParent(root)
}

func (s *workspaceStage) checkRelocationParents() error {
	r := s.relocation.record
	for _, parent := range []struct{ path, identity string }{{filepath.Dir(s.journal.writer.target), r.OriginalParent}, {filepath.Dir(r.State), r.StateParent}} {
		path, expected := parent.path, parent.identity
		actual, err := relocationParentPath(path)
		if err != nil {
			return err
		}
		if actual != expected {
			return replacementConflict(path, "relocation parent identity changed")
		}
	}
	return nil
}

func (s *workspaceStage) checkRelocationLease(ctx context.Context, store string, lease *legacyStoreLease) error {
	if err := s.checkChildren(ctx); err != nil {
		return err
	}
	if err := s.checkRelocationParents(); err != nil {
		return err
	}
	id, err := captureLegacyStoreIdentity(store)
	if err != nil {
		return err
	}
	if id != s.relocation.record.Root.Tree.ID {
		return replacementConflict(store, "relocation store identity changed")
	}
	return lease.check(store)
}

func (s *workspaceStage) checkRelocationStore(ctx context.Context, store string, lease *legacyStoreLease) error {
	if err := s.checkRelocationLease(ctx, store, lease); err != nil {
		return err
	}
	r := s.relocation
	if !r.ready {
		return replacementConflict(s.name, "relocation inventory is incomplete")
	}
	if err := requireLegacyStoreShape(store); err != nil {
		return err
	}
	actual, err := relocationRoot(ctx, store)
	if err != nil {
		return err
	}
	if !sameReplacementTree(actual, r.record.Root.Tree) {
		return replacementConflict(store, "relocation store metadata changed")
	}
	count, err := scanLegacyGenerations(ctx, filepath.Join(store, "generations"), func(entry fs.DirEntry) error {
		expected, ok := r.generations[entry.Name()]
		if !ok {
			return replacementConflict(entry.Name(), "relocation contains an unrecognized generation")
		}
		actual, err := snapshotTree(ctx, filepath.Join(store, "generations", entry.Name()))
		if err != nil {
			return err
		}
		if !sameReplacementTree(actual, expected) {
			return replacementConflict(entry.Name(), "retained generation changed during relocation")
		}
		return nil
	})
	if err != nil {
		return err
	}
	if count != r.record.Retained {
		return replacementConflict(store, "relocation retained generation count changed")
	}
	generation, _, retained, err := inspectLegacyStore(ctx, store, lease)
	if err != nil {
		return err
	}
	current, err := relocationCurrent(generation)
	if err != nil {
		return err
	}
	if current != r.record.Current || retained != r.record.Retained {
		return replacementConflict(store, "relocation current generation changed")
	}
	return s.checkRelocationLease(ctx, store, lease)
}
