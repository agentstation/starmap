//go:build !darwin && !linux

package workspace

import stderrors "errors"

func swapDirectories(_, _ string) error {
	return stderrors.New("atomic directory exchange is unsupported on this platform")
}
