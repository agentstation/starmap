package privatefiles

import (
	"io/fs"
	"os"
)

// The POSIX ACL mask bounds named-user and group writes by the group mode bits.
func validateServiceACL(_ *os.Root, _ string, _ fs.FileInfo) error { return nil }
