package filepublish

import (
	"os"

	"github.com/agentstation/starmap/pkg/errors"
)

// SyncDirectory flushes an open directory through the platform's filesystem API.
// It returns unsupported-operation and access errors without changing permissions.
// A successful call does not qualify the filesystem or hardware for power-loss recovery.
func SyncDirectory(root *os.Root) error {
	if root == nil {
		return &errors.ValidationError{Field: "publication.directory", Message: "requires an open root"}
	}
	if err := syncOpenDirectory(root); err != nil {
		return &os.PathError{Op: "sync directory", Path: root.Name(), Err: err}
	}
	return nil
}
