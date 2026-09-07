//go:build darwin || linux

package privatefiles

import (
	"io/fs"
	"os"
	"syscall"

	"github.com/agentstation/starmap/pkg/errors"
	"github.com/agentstation/starmap/pkg/productpaths/policy"
)

func validateServiceMetadata(info fs.FileInfo) error {
	observed := policy.POSIXMetadata{Mode: info.Mode()}
	if stat, ok := info.Sys().(*syscall.Stat_t); ok {
		observed.OwnerKnown = true
		observed.OwnerTrusted = stat.Uid == 0 || int64(stat.Uid) == int64(os.Geteuid())
	}
	if reason := policy.ServicePOSIXReason(observed); reason != "" {
		return &errors.ValidationError{Field: "configuration.file", Message: "requires root or effective-user ownership and no group or other writes: " + reason}
	}
	return nil
}
