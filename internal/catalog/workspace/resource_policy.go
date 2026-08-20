package workspace

import (
	"io/fs"
	"time"
)

const (
	directoryMode  fs.FileMode = 0o755
	fileMode       fs.FileMode = 0o644
	lockRetryDelay             = 10 * time.Millisecond
)
