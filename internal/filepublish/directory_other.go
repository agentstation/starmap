//go:build !darwin && !linux && !windows

package filepublish

import (
	"os"

	"github.com/agentstation/starmap/pkg/errors"
)

func renameDirectory(_ *os.Root, _ string, _ *os.Root, _ string) error {
	return &errors.ValidationError{Field: "publication.directory", Message: "atomic publication is unsupported on this platform"}
}
