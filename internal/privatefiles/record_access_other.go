//go:build !darwin && !linux && !windows

package privatefiles

import (
	"github.com/agentstation/starmap/pkg/errors"
	"io/fs"
	"os"
)

func nativePublicationAccess(*os.File) ([]byte, error) {
	return nil, &errors.ConfigError{Component: "private record access", Message: "native ownership is unavailable"}
}

func openPublicationEntry(root *os.Root, name string, _ fs.FileInfo) (*os.File, error) {
	return root.Open(name)
}
