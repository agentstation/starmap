//go:build !darwin && !linux && !windows

package filepublish

import (
	"os"

	"github.com/agentstation/starmap/pkg/errors"
)

// Identity returns the native volume and file identity for an open handle.
func Identity(_ *os.File) (string, error) {
	return "", &errors.ValidationError{Field: "file.identity", Message: "platform does not support native file identity"}
}
