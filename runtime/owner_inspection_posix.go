//go:build darwin || linux

package runtime

import (
	"os"
	"syscall"
)

const ownerRecordInspectionSupported = true

func openOwnerRecordForInspection(root *os.Root) (*os.File, error) {
	return root.OpenFile(ownerRecordName, os.O_RDONLY|syscall.O_NOFOLLOW|syscall.O_NONBLOCK, 0)
}
