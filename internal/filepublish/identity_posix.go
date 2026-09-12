//go:build darwin || linux

package filepublish

import (
	"fmt"
	"os"
	"syscall"

	"github.com/agentstation/starmap/pkg/errors"
)

// Identity returns the native volume and file identity for an open handle.
func Identity(file *os.File) (string, error) {
	info, err := file.Stat()
	if err != nil {
		return "", err
	}
	stat, ok := info.Sys().(*syscall.Stat_t)
	if !ok {
		return "", &errors.ValidationError{Field: "file.identity", Message: "native file identity is unavailable"}
	}
	return fmt.Sprintf("posix:%x:%x", stat.Dev, stat.Ino), nil
}
