package privatefiles

import (
	"io/fs"
	"os"
)

// Linux ACL access remains bounded by the directory mode mask and sticky restriction.
func validateAncestorACL(_ *os.Root, _ fs.FileInfo) error { return nil }
