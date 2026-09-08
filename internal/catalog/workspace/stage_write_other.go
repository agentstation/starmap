//go:build !windows

package workspace

import "os"

func writableStagedDirectory(_ *os.File) (func() error, error) {
	return func() error { return nil }, nil
}
