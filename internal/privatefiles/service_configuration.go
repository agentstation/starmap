package privatefiles

import (
	"io/fs"
	"os"

	"github.com/agentstation/starmap/pkg/errors"
)

// ReadServiceConfiguration reads a bounded primary input with trusted ownership and protected writes.
// The caller must explicitly select this policy and validate the selected path's ancestors.
// This exception does not change private state or dotenv readers and never changes permissions.
func ReadServiceConfiguration(root *os.Root, name string, limit int64) ([]byte, error) {
	return readCheckedFile(root, name, limit, serviceConfigurationInfo)
}

func serviceConfigurationInfo(root *os.Root, name string) (fs.FileInfo, error) {
	info, err := root.Lstat(name)
	if err != nil {
		return nil, err
	}
	if !info.Mode().IsRegular() {
		return nil, &errors.ValidationError{Field: "configuration.file", Message: "must be a regular file without a symbolic link"}
	}
	if err := validateServiceMetadata(info); err != nil {
		return nil, err
	}
	if err := validateServiceACL(root, name, info); err != nil {
		return nil, err
	}
	return info, nil
}
