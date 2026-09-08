package runtime

import (
	"os"

	"github.com/agentstation/starmap/internal/filepublish"
)

func publishMigrationDirectory(parent *os.Root, source, target string) error {
	return filepublish.DirectoryNoReplace(parent, source, target)
}
