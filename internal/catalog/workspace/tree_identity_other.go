//go:build !darwin && !linux && !windows

package workspace

import (
	"os"

	"github.com/agentstation/starmap/pkg/errors"
)

func entryIdentity(_ *os.File) (string, error) {
	return "", &errors.ValidationError{Field: "workspace_replacement.identity", Message: "platform does not support directory identity"}
}
