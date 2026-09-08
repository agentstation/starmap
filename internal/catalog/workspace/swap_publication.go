//go:build darwin || linux

package workspace

import (
	"path/filepath"

	"github.com/agentstation/starmap/pkg/errors"
)

func promoteExistingDirectory(staged, target string) error {
	if err := swapDirectories(staged, target); err != nil {
		return errors.WrapIO("promote", target, err)
	}
	return syncDirectory(filepath.Dir(target))
}
