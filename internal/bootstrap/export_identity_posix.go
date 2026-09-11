//go:build darwin || linux

package bootstrap

import (
	"fmt"
	"os"
	"syscall"

	"github.com/agentstation/starmap/pkg/errors"
)

func baselineFileIdentity(file *os.File) (string, error) {
	info, err := file.Stat()
	if err != nil {
		return "", err
	}
	stat, ok := info.Sys().(*syscall.Stat_t)
	if !ok {
		return "", &errors.ValidationError{Field: "baseline.identity", Message: "native file identity is unavailable"}
	}
	return fmt.Sprintf("posix:%x:%x", stat.Dev, stat.Ino), nil
}
