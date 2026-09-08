package workspace

import (
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
	cleanup := func() {
		selected, err := readTargetInfo(alias)
		if err == nil && selected.Mode().IsRegular() && os.SameFile(before, selected) {
			_ = os.Remove(alias)
		}
	}
	return alias, cleanup, nil
}
