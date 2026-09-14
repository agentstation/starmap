package workspace

import (
	"context"
	"crypto/rand"
	"os"
	"path/filepath"

	"github.com/agentstation/starmap/pkg/productpaths/policy"
)

// The external hard link keeps the same commit lock held while the store directory moves.
func prepareLegacyLockPath(original string, before os.FileInfo) (string, func(), error) {
	if err := policy.Require("catalog-migration-lock", policy.OwnerOnly); err != nil {
		return "", nil, err
	}
	legacy := filepath.Dir(original)
	alias := filepath.Join(filepath.Dir(legacy), "."+filepath.Base(legacy)+".starmap-migration-lock-"+rand.Text())
	if err := os.Link(original, alias); err != nil {
		return "", nil, err
	}
	parent, err := os.OpenRoot(filepath.Dir(alias))
	if err != nil {
		return "", nil, err
	}
	receipt, err := optionalWorkspaceRecord(context.Background(), parent, filepath.Base(alias))
	_ = parent.Close()
	if err != nil {
		return "", nil, err
	}
	cleanup := func() {
		parent, err := os.OpenRoot(filepath.Dir(alias))
		if err != nil {
			return
		}
		defer func() { _ = parent.Close() }()
		ctx, cancel := context.WithTimeout(context.Background(), workspaceCleanupTimeout)
		defer cancel()
		_ = receipt.cleanup(ctx, parent)
	}
	return alias, cleanup, nil
}
