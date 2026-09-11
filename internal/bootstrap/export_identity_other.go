//go:build !darwin && !linux && !windows

package bootstrap

import (
	"os"

	"github.com/agentstation/starmap/pkg/errors"
)

func baselineFileIdentity(_ *os.File) (string, error) {
	return "", &errors.ValidationError{Field: "baseline.identity", Message: "platform does not support native file identity"}
}
