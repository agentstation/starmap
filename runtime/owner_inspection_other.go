//go:build !darwin && !linux && !windows

package runtime

import "os"

const ownerRecordInspectionSupported = false

func openOwnerRecordForInspection(_ *os.Root) (*os.File, error) {
	return nil, os.ErrPermission
}
