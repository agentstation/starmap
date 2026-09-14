package workspace

import (
	"context"
	"os"
	"path/filepath"
)

func pendingRelocation(ctx context.Context, target string, writer *workspaceWriter) (_ *workspaceStage, resultErr error) {
	names, err := preparationNames(ctx, target)
	if err != nil {
		return nil, err
	}
	var retained *workspaceStage
	defer func() {
		if resultErr != nil && retained != nil {
			retained.releaseHandles()
		}
	}()
	for _, name := range names {
		stage, err := readPreparation(ctx, target, name, writer)
		if err != nil {
			return nil, err
		}
		if stage.relocation == nil {
			stage.releaseHandles()
			continue
		}
		if retained != nil {
			stage.releaseHandles()
			return nil, replacementConflict(target, "multiple pending legacy relocations")
		}
		retained = stage
	}
	return retained, nil
}

func recoverLegacyRelocation(ctx context.Context, legacy, state string) error {
	names, err := preparationNames(ctx, legacy)
	if err != nil || len(names) == 0 {
		return err
	}
	writer, err := acquireWorkspaceWriter(legacy)
	if err != nil {
		return err
	}
	defer writer.close()
	stage, err := pendingRelocation(ctx, legacy, writer)
	if err != nil || stage == nil {
		return err
	}
	defer stage.releaseHandles()
	r := stage.relocation
	if r.record.State != state || !r.ready {
		return replacementConflict(legacy, "relocation destination or inventory does not match")
	}
	if err := stage.checkRelocationParents(); err != nil {
		return err
	}
	store := state
	if _, err := os.Lstat(state); os.IsNotExist(err) {
		store = legacy
	} else if err != nil {
		return err
	}
	lease, err := stage.acquireRelocationLease(ctx, store)
	if err != nil {
		return err
	}
	defer lease.close()
	if err := stage.checkRelocationStore(ctx, store, lease); err != nil {
		return err
	}
	if err := stage.checkRelocationAlias(ctx); err != nil {
		return err
	}
	move, err := prepareLegacyStoreMove(legacy, state, r.record.Root.Tree.ID)
	if err != nil {
		return err
	}
	defer move.close()
	if store == state {
		if err := stage.checkRelocationWorkspace(ctx, move); err != nil {
			return err
		}
	}
	if _, err := recoverWorkspaceExcept(ctx, legacy, writer, stage); err != nil {
		return err
	}
	if store == state {
		check := func() error { return stage.checkRelocationLease(ctx, state, lease) }
		if err := cleanupWorkspaceTreeAtChecked(ctx, move.originalParent, move.originalName, check, r.workspace); err != nil {
			return err
		}
		if err := stage.checkRelocationStore(ctx, state, lease); err != nil {
			return err
		}
		if err := move.restore(); err != nil {
			return err
		}
		if err := move.sync(); err != nil {
			return err
		}
	}
	if err := stage.checkRelocationStore(ctx, legacy, lease); err != nil {
		return err
	}
	return stage.finishRelocation(ctx, lease)
}

func (s *workspaceStage) checkRelocationAlias(ctx context.Context) error {
	alias := s.relocation.record.Alias
	if alias == nil {
		return nil
	}
	actual, err := optionalWorkspaceRecord(ctx, s.parent, alias.Entry.Path)
	if err != nil {
		return err
	}
	if actual.identity != "" && actual != alias.state() {
		return replacementConflict(alias.Entry.Path, "relocation lock alias changed")
	}
	return nil
}

func (s *workspaceStage) checkRelocationWorkspace(ctx context.Context, move *legacyStoreMove) error {
	actual, err := optionalTree(ctx, move.originalParent, move.originalName)
	if err != nil || actual.ID == "" {
		return err
	}
	owned := s.relocation.workspace
	if actual.ID != owned.ID {
		return replacementConflict(move.originalName, "vacated catalog path is not the recorded workspace")
	}
	entries := make(map[string]treeEntry, len(owned.Entries))
	for _, entry := range owned.Entries {
		entries[entry.Path] = entry
	}
	for _, entry := range actual.Entries {
		if entries[entry.Path] != entry || owned.identities[entry.Path] != actual.identities[entry.Path] {
			return replacementConflict(filepath.Join(move.originalName, entry.Path), "relocated workspace contains changed or unrecognized entries")
		}
	}
	return nil
}
