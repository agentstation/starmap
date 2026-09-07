//go:build !darwin && !windows

package privatefiles

import (
	"io/fs"
	"os"
)

// ValidateACL checks native grants on the expected file without changing its ACL.
func ValidateACL(_ *os.Root, _ string, _ fs.FileInfo, _ string) error {
	return nil
}
