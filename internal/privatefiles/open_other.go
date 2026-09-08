//go:build !darwin && !linux

package privatefiles

import "os"

func openRecord(root *os.Root, name string) (*os.File, error) {
	return root.Open(name)
}
