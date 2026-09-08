//go:build darwin || linux

package privatefiles

import (
	"io/fs"
	"os"
	"syscall"

	"github.com/agentstation/starmap/pkg/errors"
	"github.com/agentstation/starmap/pkg/productpaths/policy"
)

// ValidateMetadata checks private mode bits and native ownership where supported.
func ValidateMetadata(info fs.FileInfo, field string) error {
	observed := policy.POSIXMetadata{Mode: info.Mode()}
	if stat, ok := info.Sys().(*syscall.Stat_t); ok {
		observed.OwnerKnown = true
		observed.OwnerMatches = int64(stat.Uid) == int64(os.Geteuid())
	}
	switch policy.PrivatePOSIXReason(observed) {
	case policy.GroupOrOtherModeBits:
		return &errors.ValidationError{Field: field, Message: "requires owner-only POSIX permissions. Review ownership and remove group and other access before retrying"}
	case policy.DifferentEffectiveOwner, policy.OwnerUnavailable:
		return &errors.ValidationError{Field: field, Message: "must belong to the effective user. Review the service identity and ownership before retrying"}
	}
	return nil
}
