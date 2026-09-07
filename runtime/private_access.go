package runtime

import (
	"io/fs"
	"os"

	"github.com/agentstation/starmap/internal/privatefiles"
)

const (
	runtimeDirectoryMode = privatefiles.DirectoryMode
	ownerRecordMode      = privatefiles.FileMode
)

func validatePrivateMetadata(info fs.FileInfo, field string) error {
	return privatefiles.ValidateMetadata(info, field)
}

func validatePrivateACL(root *os.Root, name string, expected fs.FileInfo, field string) error {
	return privatefiles.ValidateACL(root, name, expected, field)
}

func createPrivateRuntimeDirectory(directory string) error {
	return privatefiles.CreateDirectory(directory)
}

func createPrivateRuntimeChild(root *os.Root, name string) error {
	return privatefiles.CreateChild(root, name)
}

func createPrivateRuntimeFile(root *os.Root, name string) (*os.File, error) {
	return privatefiles.CreateFile(root, name)
}
