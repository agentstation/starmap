package workspace

import (
	stderrors "errors"
	"io/fs"
	"os"

	"github.com/agentstation/starmap/pkg/errors"
)

// HasProjectionState reports an existing authoring tree or its recovery records.
// It creates nothing and leaves content and ownership validation to repair.
func HasProjectionState(path string) (bool, error) {
	if path == "" {
		return false, nil
	}
	target, err := resolveTarget(path)
	if err != nil {
		return false, err
	}
	for _, candidate := range []string{target, projectionMarkerPath(target), replacementJournalPath(target)} {
		if _, err := os.Lstat(candidate); err == nil {
			return true, nil
		} else if !stderrors.Is(err, fs.ErrNotExist) {
			return false, errors.WrapIO("inspect", candidate, err)
		}
	}
	return false, nil
}
