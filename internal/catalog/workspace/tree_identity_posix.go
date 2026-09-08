//go:build darwin || linux

package workspace

import (
	"fmt"
	"os"
	"syscall"

	"github.com/agentstation/starmap/pkg/errors"
)

func entryIdentity(file *os.File) (string, error) {
	info, err := file.Stat()
	if err != nil {
		return "", err
	}
	stat, ok := info.Sys().(*syscall.Stat_t)
	if !ok {
		return "", &errors.ValidationError{Field: "workspace_replacement.identity", Message: "directory identity is unavailable"}
	}
	return fmt.Sprintf("posix:%x:%x", stat.Dev, stat.Ino), nil
}
