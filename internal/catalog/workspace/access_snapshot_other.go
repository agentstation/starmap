//go:build !darwin && !linux && !windows

package workspace

import (
	"io/fs"
	"os"

	"github.com/agentstation/starmap/pkg/errors"
)

func nativeEntryAccess(_ *os.File) ([]byte, error) {
	return nil, &errors.ConfigError{Component: "workspace access", Message: "platform does not support native access snapshots"}
}

func openSnapshotEntry(root *os.Root, name string, _ fs.FileInfo) (*os.File, error) {
	return root.Open(name)
}
