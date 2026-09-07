//go:build !darwin && !linux && !windows

package privatefiles

import (
	"io/fs"
	"os"

	"github.com/agentstation/starmap/pkg/errors"
)

func validateServiceMetadata(_ fs.FileInfo) error {
	return &errors.ConfigError{Component: "configuration access", Message: "service-managed access is unsupported on this platform"}
}

func validateServiceACL(_ *os.Root, _ string, _ fs.FileInfo) error {
	return validateServiceMetadata(nil)
}
