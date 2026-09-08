//go:build !darwin && !linux

package privatefiles

import "io/fs"

// ValidateMetadata checks private mode bits and native ownership where supported.
func ValidateMetadata(_ fs.FileInfo, _ string) error {
	return nil
}
