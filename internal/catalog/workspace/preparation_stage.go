package workspace

import (
	"context"
	stderrors "errors"
	"io"
	"path/filepath"
	"slices"
)

func (s *workspaceStage) createTree(ctx context.Context, name string) (*preparationTree, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if err := s.private.Mkdir(name, directoryMode); err != nil {
		return nil, err
	}
	return s.trackTree(name)
}

func (s *workspaceStage) trackTree(name string) (*preparationTree, error) {
	root, err := s.private.OpenRoot(name)
	if err != nil {
		return nil, err
	}
	tree, err := trackPreparationTree(root)
	if err != nil {
		_ = root.Close()
		return nil, err
	}
	tree.syncWrites = name == "tree"
	s.trees[name] = tree
	if s.journal != nil {
		tree.record = func(entry treeEntry, identity string) error { return s.journal.record(name, entry, identity) }
		if err := tree.record(tree.entries["."], tree.identities["."]); err != nil {
			return nil, err
		}
	}
	return tree, nil
}

func (s *workspaceStage) removeTree(ctx context.Context, name string) error {
	tree := s.trees[name]
	expected, err := tree.snapshot()
	if err != nil {
		return err
	}
	if tree.root != nil {
		root := tree.root
		tree.root = nil
		if err := root.Close(); err != nil {
			return err
		}
	}
	if err := cleanupWorkspaceTreeAtChecked(ctx, s.private, name, s.checkWriter, expected); err != nil {
		return err
	}
	delete(s.trees, name)
	return nil
}

func (s *workspaceStage) close(ctx context.Context) error {
	return s.retire(ctx, false)
}

func (s *workspaceStage) detach(ctx context.Context) error {
	return s.retire(ctx, true)
}

func (s *workspaceStage) retire(ctx context.Context, retainJournal bool) error {
	defer s.releaseHandles()
	if s.relocation != nil {
		return replacementConflict(s.name, "legacy relocation requires migration recovery")
	}
	if s.enclosure.ID == "" {
		return replacementConflict(s.name, "preparation ownership is incomplete; preserve the directory")
	}
	cleanup, cancel := context.WithTimeout(context.WithoutCancel(ctx), workspaceCleanupTimeout)
	defer cancel()
	if err := s.checkChildren(cleanup); err != nil {
		return err
	}
	if retainJournal {
		if s.handoff == nil || s.trees["tree"] != nil {
			return invalidReplacement("preparation_handoff")
		}
	} else if err := s.cleanupCandidate(cleanup); err != nil {
		return err
	}
	if err := s.cleanupRecord(cleanup); err != nil {
		return err
	}
	names := make([]string, 0, len(s.trees))
	for name := range s.trees {
		names = append(names, name)
	}
	slices.Sort(names)
	for _, name := range names {
		if err := s.removeTree(cleanup, name); err != nil {
			return err
		}
	}
	if retainJournal {
		return s.journal.unchanged(cleanup)
	}
	if s.journal != nil {
		if err := s.journal.remove(cleanup); err != nil {
			return err
		}
	}
	if err := s.private.Close(); err != nil {
		return err
	}
	s.private = nil
	var check func() error
	if s.journal != nil {
		check = s.journal.writer.check
	}
	return cleanupWorkspaceTreeAtChecked(cleanup, s.parent, s.name, check, s.enclosure)
}

func (s *workspaceStage) releaseHandles() {
	if s.journal != nil {
		_ = s.journal.file.Close()
	}
	if s.source != nil {
		_ = s.source.Close()
	}
	for _, tree := range s.trees {
		if tree.root != nil {
			_ = tree.root.Close()
		}
	}
	if s.private != nil {
		_ = s.private.Close()
	}
	_ = s.parent.Close()
}

func (s *workspaceStage) checkWriter() error {
	if s.journal != nil {
		return s.journal.check()
	}
	return nil
}

func (s *workspaceStage) checkChildren(ctx context.Context) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if err := verifyCleanupRoot(s.parent, s.name, s.private, s.enclosure.ID); err != nil {
		return err
	}
	scanner := treeScanner{ctx: ctx, root: s.private}
	entry, err := scanner.entry(".")
	if err != nil {
		return err
	}
	if entry != s.enclosure.Entries[0] {
		return replacementConflict(s.name, "preparation directory access changed")
	}
	if s.journal != nil {
		if err := s.journal.unchanged(ctx); err != nil {
			return err
		}
	}
	file, err := s.private.Open(".")
	if err != nil {
		return err
	}
	defer func() { _ = file.Close() }()
	entries, err := file.ReadDir(len(s.trees) + 2)
	if err != nil && !stderrors.Is(err, io.EOF) {
		return err
	}
	for _, entry := range entries {
		if entry.Name() == preparationJournalName && s.journal != nil {
			continue
		}
		tree := s.trees[entry.Name()]
		if tree == nil || !entry.IsDir() {
			return replacementConflict(filepath.Join(s.private.Name(), entry.Name()), "preparation contains an unrecognized entry")
		}
		root, err := s.private.OpenRoot(entry.Name())
		if err != nil {
			return err
		}
		checkErr := verifyCleanupRoot(s.private, entry.Name(), root, tree.identities["."])
		closeErr := root.Close()
		if err := stderrors.Join(checkErr, closeErr); err != nil {
			return err
		}
	}
	return nil
}
