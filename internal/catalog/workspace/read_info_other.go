//go:build !windows

package workspace

import "os"

func readTargetInfo(path string) (os.FileInfo, error) { return os.Lstat(path) }
