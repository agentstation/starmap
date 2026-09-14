package workspace

import (
	"context"
	"os"
	"path/filepath"

	"github.com/agentstation/starmap/internal/filepublish"
)

func (s *workspaceStage) acquireRelocationLease(ctx context.Context, store string) (*legacyStoreLease, error) {
	alias := s.relocation.record.Alias
	if alias == nil {
		return acquireLegacyStoreLease(ctx, store)
	}
	if err := s.checkChildren(ctx); err != nil {
		return nil, err
	}
	if err := s.checkRelocationAlias(ctx); err != nil {
		return nil, err
	}
	original := filepath.Join(store, ".commit.lock")
	before, err := readTargetInfo(original)
	if err != nil {
		return nil, err
	}
	path := filepath.Join(s.parent.Name(), alias.Entry.Path)
	created := false
	if _, err := s.parent.Lstat(alias.Entry.Path); os.IsNotExist(err) {
		if err := os.Link(original, path); err != nil {
			return nil, err
		}
		created = true
		if err := filepublish.SyncDirectory(s.parent); err != nil {
			return nil, err
		}
	} else if err != nil {
		return nil, err
	}
	if err := alias.state().check(ctx, s.parent); err != nil {
		return nil, err
	}
	return lockLegacyStore(ctx, store, path, before, func() {
		if !created {
			return
		}
		cleanup, cancel := context.WithTimeout(context.Background(), workspaceCleanupTimeout)
		defer cancel()
		_ = alias.state().cleanup(cleanup, s.parent)
	})
}

func (s *workspaceStage) finishRelocation(ctx context.Context, lease *legacyStoreLease) error {
	if err := s.checkChildren(ctx); err != nil {
		return err
	}
	if err := s.checkRelocationAlias(ctx); err != nil {
		return err
	}
	lease.close()
	if alias := s.relocation.record.Alias; alias != nil {
		if err := alias.state().cleanup(ctx, s.parent); err != nil {
			return err
		}
	}
	return s.retireRelocation(ctx)
}
