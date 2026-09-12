package workspace

import (
	"io/fs"
	"time"
)

const (
	directoryMode           fs.FileMode = 0o755
	fileMode                fs.FileMode = 0o644
	lockRetryDelay                      = 10 * time.Millisecond
	workspaceCleanupTimeout             = 30 * time.Second

	replacementMaxEntries   = 10000
	replacementMaxBytes     = 256 << 20
	replacementMaxNameBytes = 1 << 20
	replacementJournalMax   = 4 << 20
	replacementReadBatch    = 128
	replacementIdentityMax  = 128
	replacementVersion      = 2
	workspaceACLMaxBytes    = 64 << 10
	workspaceAccessMode     = fs.ModePerm | fs.ModeSetuid | fs.ModeSetgid | fs.ModeSticky
)
