//go:build !windows

package workspace

import "os"

func prepareLegacyLockPath(original string, _ os.FileInfo) (string, func(), error) {
	return original, func() {}, nil
}
